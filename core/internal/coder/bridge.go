package coder

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/bowerhall/sheldon/internal/logger"
)

// CleanupWorkspaces removes workspaces older than maxAge
func (b *Bridge) CleanupWorkspaces(maxAge time.Duration) (int, error) {
	if b.jobRunner != nil {
		// in k8s mode, cleanup is handled differently
		return 0, nil
	}
	return b.sandbox.CleanupOld(maxAge)
}

type Bridge struct {
	sandbox   *Sandbox
	jobRunner *JobRunner
	skills    *Skills
	useJobs   bool
}

// BridgeConfig holds configuration for the Bridge
type BridgeConfig struct {
	SandboxDir   string
	APIKey       string
	BaseURL      string
	SkillsDir    string // directory with skill templates
	UseK8sJobs   bool   // use k8s Jobs instead of subprocess
	K8sNamespace string // namespace for Jobs
	K8sImage     string // Claude Code container image
	ArtifactsPVC string // PVC for artifacts
	SecretName   string // secret containing CODER_API_KEY
	// git integration
	GitEnabled   bool
	GitUserName  string
	GitUserEmail string
	GitOrgURL    string
}

func NewBridge(sandboxDir, apiKey, baseURL string) (*Bridge, error) {
	return NewBridgeWithConfig(BridgeConfig{
		SandboxDir: sandboxDir,
		APIKey:     apiKey,
		BaseURL:    baseURL,
		UseK8sJobs: false,
	})
}

// NewBridgeWithConfig creates a Bridge with full configuration
func NewBridgeWithConfig(cfg BridgeConfig) (*Bridge, error) {
	b := &Bridge{useJobs: cfg.UseK8sJobs}

	// load skills if directory is configured
	if cfg.SkillsDir != "" {
		b.skills = NewSkills(cfg.SkillsDir)
	}

	if cfg.UseK8sJobs {
		b.jobRunner = NewJobRunnerWithConfig(JobRunnerConfig{
			Namespace:    cfg.K8sNamespace,
			Image:        cfg.K8sImage,
			ArtifactsPVC: cfg.ArtifactsPVC,
			APIKeySecret: cfg.SecretName,
			GitEnabled:   cfg.GitEnabled,
			GitUserName:  cfg.GitUserName,
			GitUserEmail: cfg.GitUserEmail,
			GitOrgURL:    cfg.GitOrgURL,
		})
		logger.Info("claude code bridge using k8s jobs", "namespace", cfg.K8sNamespace, "git", cfg.GitEnabled)
	} else {
		sandbox, err := NewSandbox(cfg.SandboxDir, cfg.APIKey, cfg.BaseURL)
		if err != nil {
			return nil, err
		}
		b.sandbox = sandbox
		logger.Info("claude code bridge using subprocess")
	}

	return b, nil
}

func (b *Bridge) Execute(ctx context.Context, task Task) (*Result, error) {
	cfg, ok := complexityConfig[task.Complexity]
	if !ok {
		cfg = complexityConfig[ComplexityStandard]
	}

	// enrich prompt with relevant skills
	task.Prompt = b.enrichPrompt(task.Prompt)

	// use k8s Jobs if configured
	if b.useJobs && b.jobRunner != nil {
		return b.executeWithJob(ctx, task, cfg)
	}

	return b.executeWithSubprocess(ctx, task, cfg)
}

// enrichPrompt adds relevant skill patterns to the prompt
func (b *Bridge) enrichPrompt(prompt string) string {
	if b.skills == nil {
		return prompt
	}

	skills := b.skills.FormatForPrompt(prompt)
	if skills == "" {
		return prompt
	}

	return prompt + skills
}

func (b *Bridge) executeWithJob(ctx context.Context, task Task, cfg struct {
	MaxTurns int
	Timeout  time.Duration
}) (*Result, error) {
	logger.Debug("claude code starting via job", "task", task.ID, "complexity", task.Complexity)

	taskCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	result, err := b.jobRunner.RunJob(taskCtx, JobConfig{
		TaskID:   task.ID,
		Prompt:   task.Prompt,
		MaxTurns: cfg.MaxTurns,
		Timeout:  cfg.Timeout,
		Context:  task.Context,
		GitRepo:  task.GitRepo,
	})

	if err != nil {
		logger.Error("claude code job failed", "error", err, "task", task.ID)
	} else {
		logger.Debug("claude code job complete",
			"task", task.ID,
			"duration", result.Duration,
			"files", len(result.Files),
			"sanitized", result.Sanitized,
		)
	}

	return result, err
}

func (b *Bridge) executeWithSubprocess(ctx context.Context, task Task, cfg struct {
	MaxTurns int
	Timeout  time.Duration
}) (*Result, error) {
	start := time.Now()
	result := &Result{}

	taskCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	ws, err := b.sandbox.Create(task.ID)
	if err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}

	// don't cleanup - workspace persists for build_image/deploy
	// cleanup happens via periodic cleanup or cleanup_images tool

	if err := b.sandbox.WriteContext(ws, task.Context); err != nil {
		return nil, fmt.Errorf("write context: %w", err)
	}

	logger.Debug("claude code starting", "task", task.ID, "complexity", task.Complexity)

	output, err := b.run(taskCtx, ws, task.Prompt, cfg.MaxTurns)
	if err != nil {
		result.Error = err.Error()
		logger.Error("claude code failed", "error", err, "task", task.ID)
	}

	sanitized, warnings := Sanitize(output)
	result.Output = sanitized
	result.Warnings = warnings
	result.Sanitized = len(warnings) > 0
	result.Duration = time.Since(start)

	files, _ := b.sandbox.CollectFiles(ws)
	result.Files = files
	result.WorkspacePath = ws.Path

	logger.Debug("claude code complete",
		"task", task.ID,
		"duration", result.Duration,
		"files", len(files),
		"sanitized", result.Sanitized,
	)

	return result, nil
}

func (b *Bridge) run(ctx context.Context, ws *Workspace, prompt string, maxTurns int) (string, error) {
	args := []string{
		"--print",
		"--output-format", "text",
		"--max-turns", fmt.Sprintf("%d", maxTurns),
		"--dangerously-skip-permissions",
		"-p", prompt,
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = ws.Path
	cmd.Env = b.sandbox.CleanEnv()

	logger.Debug("claude command", "dir", ws.Path, "args", args)

	var output strings.Builder
	var stderrBuf strings.Builder

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start: %w", err)
	}

	// capture stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			stderrBuf.WriteString(line)
			stderrBuf.WriteString("\n")
			logger.Debug("claude stderr", "line", line)
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		output.WriteString(line)
		output.WriteString("\n")
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return output.String(), fmt.Errorf("timeout exceeded")
		}
		// log stderr on error
		if stderrBuf.Len() > 0 {
			logger.Error("claude stderr output", "stderr", stderrBuf.String())
		}
		return output.String(), fmt.Errorf("exit: %w", err)
	}

	return output.String(), nil
}

func (b *Bridge) ExecuteWithProgress(ctx context.Context, task Task, onProgress func(StreamEvent)) (*Result, error) {
	cfg, ok := complexityConfig[task.Complexity]
	if !ok {
		cfg = complexityConfig[ComplexityStandard]
	}

	// enrich prompt with relevant skills
	task.Prompt = b.enrichPrompt(task.Prompt)

	// use k8s Jobs if configured
	if b.useJobs && b.jobRunner != nil {
		return b.executeWithJobProgress(ctx, task, cfg, onProgress)
	}

	return b.executeWithSubprocessProgress(ctx, task, cfg, onProgress)
}

func (b *Bridge) executeWithJobProgress(ctx context.Context, task Task, cfg struct {
	MaxTurns int
	Timeout  time.Duration
}, onProgress func(StreamEvent)) (*Result, error) {
	logger.Debug("claude code starting via job", "task", task.ID, "complexity", task.Complexity)

	taskCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	result, err := b.jobRunner.RunJob(taskCtx, JobConfig{
		TaskID:     task.ID,
		Prompt:     task.Prompt,
		MaxTurns:   cfg.MaxTurns,
		Timeout:    cfg.Timeout,
		Context:    task.Context,
		OnProgress: onProgress,
		GitRepo:    task.GitRepo,
	})

	if err != nil {
		logger.Error("claude code job failed", "error", err, "task", task.ID)
	} else {
		logger.Debug("claude code job complete",
			"task", task.ID,
			"duration", result.Duration,
			"files", len(result.Files),
		)
	}

	return result, err
}

func (b *Bridge) executeWithSubprocessProgress(ctx context.Context, task Task, cfg struct {
	MaxTurns int
	Timeout  time.Duration
}, onProgress func(StreamEvent)) (*Result, error) {
	start := time.Now()
	result := &Result{}

	taskCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	ws, err := b.sandbox.Create(task.ID)
	if err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}

	// don't cleanup - workspace persists for build_image/deploy

	if err := b.sandbox.WriteContext(ws, task.Context); err != nil {
		return nil, fmt.Errorf("write context: %w", err)
	}

	logger.Debug("claude code starting", "task", task.ID, "complexity", task.Complexity)

	output, err := b.runWithProgress(taskCtx, ws, task.Prompt, cfg.MaxTurns, onProgress)
	if err != nil {
		result.Error = err.Error()
		logger.Error("claude code failed", "error", err, "task", task.ID)
	}

	sanitized, warnings := Sanitize(output)
	result.Output = sanitized
	result.Warnings = warnings
	result.Sanitized = len(warnings) > 0
	result.Duration = time.Since(start)

	files, _ := b.sandbox.CollectFiles(ws)
	result.Files = files
	result.WorkspacePath = ws.Path

	return result, nil
}

func (b *Bridge) runWithProgress(ctx context.Context, ws *Workspace, prompt string, maxTurns int, onProgress func(StreamEvent)) (string, error) {
	args := []string{
		"--print",
		"--verbose",
		"--output-format", "stream-json",
		"--max-turns", fmt.Sprintf("%d", maxTurns),
		"--dangerously-skip-permissions",
		"-p", prompt,
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = ws.Path
	cmd.Env = b.sandbox.CleanEnv()

	logger.Debug("claude command (progress)", "dir", ws.Path, "prompt_len", len(prompt))

	var output strings.Builder
	var stderrBuf strings.Builder

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start: %w", err)
	}

	// capture stderr in background
	go func() {
		stderrScanner := bufio.NewScanner(stderr)
		for stderrScanner.Scan() {
			line := stderrScanner.Text()
			stderrBuf.WriteString(line)
			stderrBuf.WriteString("\n")
		}
	}()

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
							// detect tool use
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

	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return output.String(), fmt.Errorf("timeout exceeded")
		}
		// log stderr on error
		if stderrBuf.Len() > 0 {
			logger.Error("claude stderr output", "stderr", stderrBuf.String())
		}
		return output.String(), fmt.Errorf("exit: %w", err)
	}

	return output.String(), nil
}

// GetLocalWorkspacePath returns the local filesystem path for artifacts.
// For subprocess mode, this is the direct sandbox path.
// For k8s mode, this copies artifacts from PVC to local temp dir.
func (b *Bridge) GetLocalWorkspacePath(ctx context.Context, taskID string) (string, error) {
	if !b.useJobs || b.jobRunner == nil {
		// subprocess mode - path is already local
		return b.sandbox.baseDir + "/" + taskID, nil
	}

	// k8s mode - copy artifacts to local temp
	localDir := "/tmp/sheldon-artifacts/" + taskID
	if err := b.jobRunner.CopyArtifacts(ctx, taskID, localDir); err != nil {
		return "", fmt.Errorf("copy artifacts: %w", err)
	}

	return localDir, nil
}

// CleanupTask removes artifacts for a completed task
func (b *Bridge) CleanupTask(ctx context.Context, taskID string) error {
	if !b.useJobs || b.jobRunner == nil {
		// subprocess mode - use sandbox cleanup
		return b.sandbox.Cleanup(&Workspace{TaskID: taskID, Path: b.sandbox.baseDir + "/" + taskID})
	}

	// k8s mode - cleanup PVC artifacts
	return b.jobRunner.CleanupArtifacts(ctx, taskID)
}

// IsUsingK8sJobs returns true if the bridge is configured to use k8s Jobs
func (b *Bridge) IsUsingK8sJobs() bool {
	return b.useJobs
}
