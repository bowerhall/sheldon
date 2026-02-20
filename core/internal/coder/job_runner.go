package coder

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bowerhall/sheldon/internal/logger"
)

// JobRunner manages ephemeral k8s Jobs for Claude Code execution
type JobRunner struct {
	namespace    string
	image        string
	artifactsPVC string
	apiKeySecret string
	gitEnabled   bool
	gitUserName  string
	gitUserEmail string
	gitOrgURL    string
}

// JobRunnerConfig holds configuration for JobRunner
type JobRunnerConfig struct {
	Namespace    string
	Image        string
	ArtifactsPVC string
	APIKeySecret string
	GitEnabled   bool
	GitUserName  string
	GitUserEmail string
	GitOrgURL    string
}

// NewJobRunner creates a runner for ephemeral Claude Code Jobs
func NewJobRunner(namespace, image, artifactsPVC, apiKeySecret string) *JobRunner {
	return NewJobRunnerWithConfig(JobRunnerConfig{
		Namespace:    namespace,
		Image:        image,
		ArtifactsPVC: artifactsPVC,
		APIKeySecret: apiKeySecret,
	})
}

// NewJobRunnerWithConfig creates a runner with full configuration including git
func NewJobRunnerWithConfig(cfg JobRunnerConfig) *JobRunner {
	if cfg.Namespace == "" {
		cfg.Namespace = "sheldon"
	}
	if cfg.Image == "" {
		cfg.Image = "sheldon-claude-code:latest"
	}
	if cfg.ArtifactsPVC == "" {
		cfg.ArtifactsPVC = "sheldon-coder-artifacts"
	}
	if cfg.APIKeySecret == "" {
		cfg.APIKeySecret = "sheldon-secrets"
	}
	return &JobRunner{
		namespace:    cfg.Namespace,
		image:        cfg.Image,
		artifactsPVC: cfg.ArtifactsPVC,
		apiKeySecret: cfg.APIKeySecret,
		gitEnabled:   cfg.GitEnabled,
		gitUserName:  cfg.GitUserName,
		gitUserEmail: cfg.GitUserEmail,
		gitOrgURL:    cfg.GitOrgURL,
	}
}

// JobConfig holds configuration for a Claude Code Job
type JobConfig struct {
	TaskID     string
	Prompt     string
	MaxTurns   int
	Timeout    time.Duration
	Context    *MemoryContext
	OnProgress func(StreamEvent)
	GitRepo    string // target repo name for pushing code
}

// RunJob creates and runs an ephemeral k8s Job for Claude Code
func (r *JobRunner) RunJob(ctx context.Context, cfg JobConfig) (*Result, error) {
	start := time.Now()
	jobName := fmt.Sprintf("claude-code-%s", cfg.TaskID)
	workDir := fmt.Sprintf("/artifacts/%s", cfg.TaskID)

	// ensure artifacts directory exists
	if err := r.ensureArtifactsDir(ctx, cfg.TaskID); err != nil {
		return nil, fmt.Errorf("ensure artifacts dir: %w", err)
	}

	// write context file
	if cfg.Context != nil {
		if err := r.writeContext(ctx, cfg.TaskID, cfg.Context); err != nil {
			return nil, fmt.Errorf("write context: %w", err)
		}
	}

	// create the Job
	if err := r.createJob(ctx, jobName, cfg); err != nil {
		return nil, fmt.Errorf("create job: %w", err)
	}

	// ensure cleanup on exit
	defer r.deleteJob(context.Background(), jobName)

	// wait for completion while streaming logs
	output, err := r.waitAndStream(ctx, jobName, cfg.OnProgress)
	if err != nil && ctx.Err() == context.DeadlineExceeded {
		return &Result{
			Error:    "timeout exceeded",
			Duration: time.Since(start),
		}, fmt.Errorf("timeout exceeded")
	}

	// collect artifacts
	files, _ := r.collectArtifacts(ctx, cfg.TaskID)

	// sanitize output
	sanitized, warnings := Sanitize(output)

	return &Result{
		Output:        sanitized,
		Files:         files,
		WorkspacePath: workDir,
		Duration:      time.Since(start),
		Warnings:      warnings,
		Sanitized:     len(warnings) > 0,
		Error:         errToString(err),
	}, nil
}

func errToString(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

func (r *JobRunner) createJob(ctx context.Context, jobName string, cfg JobConfig) error {
	jobYAML := r.buildJobYAML(jobName, cfg)

	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(jobYAML)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl apply: %w\n%s", err, string(output))
	}

	logger.Debug("job created", "name", jobName)
	return nil
}

func (r *JobRunner) buildJobYAML(jobName string, cfg JobConfig) string {
	// escape prompt for shell
	prompt := strings.ReplaceAll(cfg.Prompt, "'", "'\"'\"'")

	// build git env vars if enabled
	gitEnvVars := ""
	if r.gitEnabled {
		gitEnvVars = fmt.Sprintf(`
            - name: GIT_USER_NAME
              value: "%s"
            - name: GIT_USER_EMAIL
              value: "%s"
            - name: GIT_ORG_URL
              value: "%s"
            - name: GIT_TOKEN
              valueFrom:
                secretKeyRef:
                  name: %s
                  key: GIT_TOKEN
                  optional: true`,
			r.gitUserName, r.gitUserEmail, r.gitOrgURL, r.apiKeySecret)

		// add target repo if specified
		if cfg.GitRepo != "" {
			gitEnvVars += fmt.Sprintf(`
            - name: GIT_REPO_NAME
              value: "%s"`, cfg.GitRepo)
		}
	}

	return fmt.Sprintf(`apiVersion: batch/v1
kind: Job
metadata:
  name: %s
  namespace: %s
  labels:
    app: claude-code
    task-id: %s
spec:
  ttlSecondsAfterFinished: 60
  activeDeadlineSeconds: %d
  backoffLimit: 0
  template:
    spec:
      restartPolicy: Never
      securityContext:
        runAsUser: 1001
        runAsGroup: 1000
        fsGroup: 1000
      containers:
        - name: claude-code
          image: %s
          imagePullPolicy: Always
          workingDir: /artifacts/%s
          args:
            - "--print"
            - "--verbose"
            - "--output-format"
            - "stream-json"
            - "--max-turns"
            - "%d"
            - "--dangerously-skip-permissions"
            - "-p"
            - '%s'
          env:
            - name: ANTHROPIC_API_KEY
              valueFrom:
                secretKeyRef:
                  name: %s
                  key: CODER_API_KEY
            - name: HOME
              value: /tmp%s
          resources:
            requests:
              memory: "256Mi"
              cpu: "200m"
            limits:
              memory: "1Gi"
              cpu: "1"
          volumeMounts:
            - name: artifacts
              mountPath: /artifacts
      volumes:
        - name: artifacts
          persistentVolumeClaim:
            claimName: %s
`, jobName, r.namespace, cfg.TaskID,
		int(cfg.Timeout.Seconds()),
		r.image, cfg.TaskID, cfg.MaxTurns, prompt,
		r.apiKeySecret, gitEnvVars, r.artifactsPVC)
}

func (r *JobRunner) waitAndStream(ctx context.Context, jobName string, onProgress func(StreamEvent)) (string, error) {
	// wait for pod to be created
	podName, err := r.waitForPod(ctx, jobName)
	if err != nil {
		return "", fmt.Errorf("wait for pod: %w", err)
	}

	// stream logs
	return r.streamLogs(ctx, podName, onProgress)
}

func (r *JobRunner) waitForPod(ctx context.Context, jobName string) (string, error) {
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Second):
			cmd := exec.CommandContext(ctx, "kubectl", "get", "pods",
				"-n", r.namespace,
				"-l", fmt.Sprintf("job-name=%s", jobName),
				"-o", "jsonpath={.items[0].metadata.name}")
			output, err := cmd.Output()
			if err == nil && len(output) > 0 {
				return string(output), nil
			}
		}
	}
}

func (r *JobRunner) streamLogs(ctx context.Context, podName string, onProgress func(StreamEvent)) (string, error) {
	// wait for container to start
	if err := r.waitForContainerReady(ctx, podName); err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, "kubectl", "logs",
		"-n", r.namespace,
		"-f",
		podName)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start logs: %w", err)
	}

	var output strings.Builder
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()

		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		eventType, _ := event["type"].(string)

		switch eventType {
		case "assistant":
			if msg, ok := event["message"].(map[string]any); ok {
				if content, ok := msg["content"].([]any); ok {
					for _, c := range content {
						if block, ok := c.(map[string]any); ok {
							if text, ok := block["text"].(string); ok {
								output.WriteString(text)
							}
							if blockType, ok := block["type"].(string); ok && blockType == "tool_use" {
								if toolName, ok := block["name"].(string); ok && onProgress != nil {
									onProgress(StreamEvent{Type: "tool_use", Tool: toolName})
								}
							}
						}
					}
				}
			}
			if onProgress != nil {
				onProgress(StreamEvent{Type: "thinking"})
			}

		case "result":
			if onProgress != nil {
				onProgress(StreamEvent{Type: "complete"})
			}
		}
	}

	cmd.Wait()
	return output.String(), nil
}

func (r *JobRunner) waitForContainerReady(ctx context.Context, podName string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
			cmd := exec.CommandContext(ctx, "kubectl", "get", "pod", podName,
				"-n", r.namespace,
				"-o", "jsonpath={.status.containerStatuses[0].state}")
			output, err := cmd.Output()
			if err != nil {
				continue
			}
			// running or terminated means we can stream logs
			if strings.Contains(string(output), "running") || strings.Contains(string(output), "terminated") {
				return nil
			}
		}
	}
}

func (r *JobRunner) deleteJob(ctx context.Context, jobName string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "delete", "job",
		jobName,
		"-n", r.namespace,
		"--ignore-not-found")
	_, err := cmd.CombinedOutput()
	if err == nil {
		logger.Debug("job deleted", "name", jobName)
	}
	return err
}

func (r *JobRunner) ensureArtifactsDir(ctx context.Context, taskID string) error {
	// use a temporary pod to create the directory
	podName := fmt.Sprintf("init-%s", taskID)

	yaml := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
spec:
  restartPolicy: Never
  containers:
    - name: init
      image: alpine:3.19
      command: ["mkdir", "-p", "/artifacts/%s"]
      volumeMounts:
        - name: artifacts
          mountPath: /artifacts
  volumes:
    - name: artifacts
      persistentVolumeClaim:
        claimName: %s
`, podName, r.namespace, taskID, r.artifactsPVC)

	createCmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	createCmd.Stdin = strings.NewReader(yaml)
	if _, err := createCmd.CombinedOutput(); err != nil {
		return err
	}

	// wait for completion
	for i := 0; i < 30; i++ {
		time.Sleep(time.Second)
		checkCmd := exec.CommandContext(ctx, "kubectl", "get", "pod", podName,
			"-n", r.namespace,
			"-o", "jsonpath={.status.phase}")
		output, _ := checkCmd.Output()
		if strings.Contains(string(output), "Succeeded") || strings.Contains(string(output), "Failed") {
			break
		}
	}

	// cleanup
	exec.CommandContext(ctx, "kubectl", "delete", "pod", podName,
		"-n", r.namespace, "--ignore-not-found").Run()

	return nil
}

func (r *JobRunner) writeContext(ctx context.Context, taskID string, memCtx *MemoryContext) error {
	// build context content
	var buf strings.Builder
	buf.WriteString("# Task Context\n\n")

	if len(memCtx.UserPreferences) > 0 {
		buf.WriteString("## Preferences\n")
		for k, v := range memCtx.UserPreferences {
			fmt.Fprintf(&buf, "- %s: %s\n", k, v)
		}
		buf.WriteString("\n")
	}

	if len(memCtx.RelevantFacts) > 0 {
		buf.WriteString("## Context\n")
		for _, f := range memCtx.RelevantFacts {
			fmt.Fprintf(&buf, "- %s: %s\n", f.Field, f.Value)
		}
		buf.WriteString("\n")
	}

	if len(memCtx.Constraints) > 0 {
		buf.WriteString("## Constraints\n")
		for _, c := range memCtx.Constraints {
			fmt.Fprintf(&buf, "- %s\n", c)
		}
		buf.WriteString("\n")
	}

	buf.WriteString("## Rules\n")
	buf.WriteString("- Do not hardcode secrets or API keys\n")
	buf.WriteString("- Handle errors gracefully\n")
	buf.WriteString("- Keep code minimal and focused\n")

	content := buf.String()

	// write via configmap and mount, or use kubectl exec
	// for simplicity, use a temporary pod
	return r.writeFile(ctx, taskID, "CLAUDE.md", content)
}

func (r *JobRunner) writeFile(ctx context.Context, taskID, filename, content string) error {
	podName := fmt.Sprintf("write-%s", taskID)

	// escape content for shell
	escaped := strings.ReplaceAll(content, "'", "'\"'\"'")

	yaml := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
spec:
  restartPolicy: Never
  containers:
    - name: writer
      image: alpine:3.19
      command: ["sh", "-c", "cat > /artifacts/%s/%s << 'ENDOFFILE'\n%s\nENDOFFILE"]
      volumeMounts:
        - name: artifacts
          mountPath: /artifacts
  volumes:
    - name: artifacts
      persistentVolumeClaim:
        claimName: %s
`, podName, r.namespace, taskID, filename, escaped, r.artifactsPVC)

	createCmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	createCmd.Stdin = strings.NewReader(yaml)
	if _, err := createCmd.CombinedOutput(); err != nil {
		return err
	}

	// wait for completion
	for i := 0; i < 30; i++ {
		time.Sleep(time.Second)
		checkCmd := exec.CommandContext(ctx, "kubectl", "get", "pod", podName,
			"-n", r.namespace,
			"-o", "jsonpath={.status.phase}")
		output, _ := checkCmd.Output()
		if strings.Contains(string(output), "Succeeded") || strings.Contains(string(output), "Failed") {
			break
		}
	}

	// cleanup
	exec.CommandContext(ctx, "kubectl", "delete", "pod", podName,
		"-n", r.namespace, "--ignore-not-found").Run()

	return nil
}

func (r *JobRunner) collectArtifacts(ctx context.Context, taskID string) ([]string, error) {
	podName := fmt.Sprintf("collect-%s", taskID)
	workDir := fmt.Sprintf("/artifacts/%s", taskID)

	// list files via temporary pod
	yaml := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
spec:
  restartPolicy: Never
  containers:
    - name: collector
      image: alpine:3.19
      command: ["find", "%s", "-type", "f", "-not", "-name", "CLAUDE.md", "-not", "-path", "*/.git/*"]
      volumeMounts:
        - name: artifacts
          mountPath: /artifacts
  volumes:
    - name: artifacts
      persistentVolumeClaim:
        claimName: %s
`, podName, r.namespace, workDir, r.artifactsPVC)

	createCmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	createCmd.Stdin = strings.NewReader(yaml)
	if _, err := createCmd.CombinedOutput(); err != nil {
		return nil, err
	}

	// wait for completion
	for i := 0; i < 30; i++ {
		time.Sleep(time.Second)
		checkCmd := exec.CommandContext(ctx, "kubectl", "get", "pod", podName,
			"-n", r.namespace,
			"-o", "jsonpath={.status.phase}")
		output, _ := checkCmd.Output()
		if strings.Contains(string(output), "Succeeded") {
			break
		}
		if strings.Contains(string(output), "Failed") {
			exec.CommandContext(ctx, "kubectl", "delete", "pod", podName,
				"-n", r.namespace, "--ignore-not-found").Run()
			return nil, nil
		}
	}

	// get logs (file list)
	logsCmd := exec.CommandContext(ctx, "kubectl", "logs", podName, "-n", r.namespace)
	output, _ := logsCmd.Output()

	// cleanup
	exec.CommandContext(ctx, "kubectl", "delete", "pod", podName,
		"-n", r.namespace, "--ignore-not-found").Run()

	var files []string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			// convert to relative path
			rel := strings.TrimPrefix(line, workDir+"/")
			if rel != "" && rel != line {
				files = append(files, rel)
			}
		}
	}

	return files, nil
}

// IsK8sAvailable checks if kubectl is available and we're in a cluster
func IsK8sAvailable() bool {
	cmd := exec.Command("kubectl", "cluster-info")
	return cmd.Run() == nil
}

// CopyArtifacts copies artifacts from PVC to local directory
func (r *JobRunner) CopyArtifacts(ctx context.Context, taskID, destDir string) error {
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	podName := fmt.Sprintf("copy-%s", taskID)
	srcDir := fmt.Sprintf("/artifacts/%s", taskID)

	// create pod that tars the artifacts
	yaml := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
spec:
  restartPolicy: Never
  containers:
    - name: copier
      image: alpine:3.19
      command: ["tar", "-cf", "-", "-C", "%s", "."]
      volumeMounts:
        - name: artifacts
          mountPath: /artifacts
  volumes:
    - name: artifacts
      persistentVolumeClaim:
        claimName: %s
`, podName, r.namespace, srcDir, r.artifactsPVC)

	createCmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	createCmd.Stdin = strings.NewReader(yaml)
	if _, err := createCmd.CombinedOutput(); err != nil {
		return err
	}

	defer func() {
		exec.CommandContext(ctx, "kubectl", "delete", "pod", podName,
			"-n", r.namespace, "--ignore-not-found").Run()
	}()

	// wait for running
	for i := 0; i < 30; i++ {
		time.Sleep(time.Second)
		checkCmd := exec.CommandContext(ctx, "kubectl", "get", "pod", podName,
			"-n", r.namespace,
			"-o", "jsonpath={.status.phase}")
		output, _ := checkCmd.Output()
		if strings.Contains(string(output), "Running") || strings.Contains(string(output), "Succeeded") {
			break
		}
	}

	// pipe logs (tar stream) to local tar extraction
	logsCmd := exec.CommandContext(ctx, "kubectl", "logs", podName, "-n", r.namespace)
	tarCmd := exec.CommandContext(ctx, "tar", "-xf", "-", "-C", destDir)

	pipe, err := logsCmd.StdoutPipe()
	if err != nil {
		return err
	}
	tarCmd.Stdin = pipe

	if err := logsCmd.Start(); err != nil {
		return err
	}
	if err := tarCmd.Start(); err != nil {
		return err
	}

	logsCmd.Wait()
	tarCmd.Wait()

	return nil
}

// CleanupArtifacts removes artifacts for a task from the PVC
func (r *JobRunner) CleanupArtifacts(ctx context.Context, taskID string) error {
	podName := fmt.Sprintf("cleanup-%s", taskID)
	workDir := fmt.Sprintf("/artifacts/%s", taskID)

	yaml := fmt.Sprintf(`apiVersion: v1
kind: Pod
metadata:
  name: %s
  namespace: %s
spec:
  restartPolicy: Never
  containers:
    - name: cleaner
      image: alpine:3.19
      command: ["rm", "-rf", "%s"]
      volumeMounts:
        - name: artifacts
          mountPath: /artifacts
  volumes:
    - name: artifacts
      persistentVolumeClaim:
        claimName: %s
`, podName, r.namespace, workDir, r.artifactsPVC)

	createCmd := exec.CommandContext(ctx, "kubectl", "apply", "-f", "-")
	createCmd.Stdin = strings.NewReader(yaml)
	if _, err := createCmd.CombinedOutput(); err != nil {
		return err
	}

	// wait briefly
	time.Sleep(3 * time.Second)

	// cleanup the cleanup pod
	exec.CommandContext(ctx, "kubectl", "delete", "pod", podName,
		"-n", r.namespace, "--ignore-not-found").Run()

	return nil
}

// GetWorkspacePath returns the PVC path for a task's workspace
func (r *JobRunner) GetWorkspacePath(taskID string) string {
	return filepath.Join("/artifacts", taskID)
}
