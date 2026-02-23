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

// DockerRunner runs code generation in ephemeral Docker containers
type DockerRunner struct {
	image        string
	artifactsDir string
	apiKey       string
	fallbackKey  string
	model        string
	git          GitConfig
}

// DockerRunnerConfig holds configuration for DockerRunner
type DockerRunnerConfig struct {
	Image        string // container image (default: sheldon-coder-sandbox:latest)
	ArtifactsDir string // local directory for artifacts
	APIKey       string // NVIDIA NIM API key
	FallbackKey  string // Kimi API key
	Model        string // model to use
	Git          GitConfig
}

// JobConfig holds configuration for a code generation job
type JobConfig struct {
	TaskID   string
	Prompt   string
	MaxTurns int
	Timeout  time.Duration
	Context  *MemoryContext
	GitRepo  string // target repo name (e.g., "weather-bot")
}

// NewDockerRunner creates a new DockerRunner
func NewDockerRunner(cfg DockerRunnerConfig) *DockerRunner {
	if cfg.Image == "" {
		cfg.Image = "sheldon-coder-sandbox:latest"
	}
	if cfg.ArtifactsDir == "" {
		cfg.ArtifactsDir = "/tmp/sheldon-artifacts"
	}
	if cfg.Model == "" {
		cfg.Model = "kimi-k2.5"
	}

	// ensure artifacts directory exists
	os.MkdirAll(cfg.ArtifactsDir, 0755)

	return &DockerRunner{
		image:        cfg.Image,
		artifactsDir: cfg.ArtifactsDir,
		apiKey:       cfg.APIKey,
		fallbackKey:  cfg.FallbackKey,
		model:        cfg.Model,
		git:          cfg.Git,
	}
}

// RunJob runs a code generation task in an ephemeral container
func (r *DockerRunner) RunJob(ctx context.Context, cfg JobConfig) (*Result, error) {
	start := time.Now()

	// create workspace directory for this task
	workDir := filepath.Join(r.artifactsDir, cfg.TaskID)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}

	// write context file if provided
	if cfg.Context != nil {
		if err := r.writeContext(workDir, cfg.Context); err != nil {
			return nil, fmt.Errorf("write context: %w", err)
		}
	}

	logger.Debug("docker runner starting", "task", cfg.TaskID, "image", r.image)

	// build docker run command
	args := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/workspace", workDir),
		"-w", "/workspace",
	}

	// pass API keys as environment variables
	if r.apiKey != "" {
		args = append(args, "-e", "NVIDIA_API_KEY="+r.apiKey)
	}
	if r.fallbackKey != "" {
		args = append(args, "-e", "KIMI_API_KEY="+r.fallbackKey)
	}
	if r.model != "" {
		args = append(args, "-e", "CODER_MODEL="+r.model)
	}

	// pass git user config (NOT the token - coder should never have access to GIT_TOKEN)
	// git clone/push is handled by Sheldon externally via GitOps
	if r.git.UserName != "" {
		args = append(args, "-e", "GIT_USER_NAME="+r.git.UserName)
	}
	if r.git.UserEmail != "" {
		args = append(args, "-e", "GIT_USER_EMAIL="+r.git.UserEmail)
	}

	// add image and coder arguments
	args = append(args, r.image,
		"--print",
		"--output-format", "text",
		"--max-turns", fmt.Sprintf("%d", cfg.MaxTurns),
		"--dangerously-skip-permissions",
		"-p", cfg.Prompt,
	)

	cmd := exec.CommandContext(ctx, "docker", args...)

	var output strings.Builder
	var stderrBuf strings.Builder

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start container: %w", err)
	}

	// capture stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			stderrBuf.WriteString(line)
			stderrBuf.WriteString("\n")
			logger.Debug("coder stderr", "line", line)
		}
	}()

	// capture stdout
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		output.WriteString(line)
		output.WriteString("\n")
	}

	result := &Result{
		Duration:      time.Since(start),
		WorkspacePath: workDir,
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = "timeout exceeded"
			return result, fmt.Errorf("timeout exceeded")
		}
		if stderrBuf.Len() > 0 {
			logger.Error("coder stderr output", "stderr", stderrBuf.String())
		}
		result.Error = err.Error()
		return result, fmt.Errorf("container exit: %w", err)
	}

	// sanitize output
	sanitized, warnings := Sanitize(output.String())
	result.Output = sanitized
	result.Warnings = warnings
	result.Sanitized = len(warnings) > 0

	// collect generated files
	files, _ := r.collectFiles(workDir)
	result.Files = files

	logger.Debug("docker runner complete",
		"task", cfg.TaskID,
		"duration", result.Duration,
		"files", len(files),
	)

	return result, nil
}

// RunJobWithProgress runs with progress callbacks
func (r *DockerRunner) RunJobWithProgress(ctx context.Context, cfg JobConfig, onProgress func(StreamEvent)) (*Result, error) {
	start := time.Now()

	// create workspace directory
	workDir := filepath.Join(r.artifactsDir, cfg.TaskID)
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}

	// write context file
	if cfg.Context != nil {
		if err := r.writeContext(workDir, cfg.Context); err != nil {
			return nil, fmt.Errorf("write context: %w", err)
		}
	}

	logger.Debug("docker runner starting (progress)", "task", cfg.TaskID, "image", r.image)

	// build docker run command with stream-json output
	args := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/workspace", workDir),
		"-w", "/workspace",
	}

	if r.apiKey != "" {
		args = append(args, "-e", "NVIDIA_API_KEY="+r.apiKey)
	}
	if r.fallbackKey != "" {
		args = append(args, "-e", "KIMI_API_KEY="+r.fallbackKey)
	}
	if r.model != "" {
		args = append(args, "-e", "CODER_MODEL="+r.model)
	}

	// pass git user config (NOT the token - coder should never have access to GIT_TOKEN)
	// git clone/push is handled by Sheldon externally via GitOps
	if r.git.UserName != "" {
		args = append(args, "-e", "GIT_USER_NAME="+r.git.UserName)
	}
	if r.git.UserEmail != "" {
		args = append(args, "-e", "GIT_USER_EMAIL="+r.git.UserEmail)
	}

	args = append(args, r.image,
		"--print",
		"--verbose",
		"--output-format", "stream-json",
		"--max-turns", fmt.Sprintf("%d", cfg.MaxTurns),
		"--dangerously-skip-permissions",
		"-p", cfg.Prompt,
	)

	cmd := exec.CommandContext(ctx, "docker", args...)

	var output strings.Builder

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start container: %w", err)
	}

	// capture stderr in background
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			// just drain stderr
		}
	}()

	// process streaming json
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

	result := &Result{
		Duration:      time.Since(start),
		WorkspacePath: workDir,
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = "timeout exceeded"
			return result, fmt.Errorf("timeout exceeded")
		}
		result.Error = err.Error()
		return result, fmt.Errorf("container exit: %w", err)
	}

	sanitized, warnings := Sanitize(output.String())
	result.Output = sanitized
	result.Warnings = warnings
	result.Sanitized = len(warnings) > 0

	files, _ := r.collectFiles(workDir)
	result.Files = files

	return result, nil
}

// CleanupArtifacts removes artifacts for a task
func (r *DockerRunner) CleanupArtifacts(taskID string) error {
	return os.RemoveAll(filepath.Join(r.artifactsDir, taskID))
}

// CleanupOld removes old artifacts
func (r *DockerRunner) CleanupOld(maxAge time.Duration) (int, error) {
	entries, err := os.ReadDir(r.artifactsDir)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			path := filepath.Join(r.artifactsDir, entry.Name())
			if err := os.RemoveAll(path); err == nil {
				removed++
			}
		}
	}

	return removed, nil
}

func (r *DockerRunner) writeContext(workDir string, ctx *MemoryContext) error {
	var buf strings.Builder
	buf.WriteString("# Task Context\n\n")

	if len(ctx.UserPreferences) > 0 {
		buf.WriteString("## Preferences\n")
		for k, v := range ctx.UserPreferences {
			fmt.Fprintf(&buf, "- %s: %s\n", k, v)
		}
		buf.WriteString("\n")
	}

	if len(ctx.RelevantFacts) > 0 {
		buf.WriteString("## Context\n")
		for _, f := range ctx.RelevantFacts {
			fmt.Fprintf(&buf, "- %s: %s\n", f.Field, f.Value)
		}
		buf.WriteString("\n")
	}

	if len(ctx.Constraints) > 0 {
		buf.WriteString("## Constraints\n")
		for _, c := range ctx.Constraints {
			fmt.Fprintf(&buf, "- %s\n", c)
		}
		buf.WriteString("\n")
	}

	buf.WriteString("## Rules\n")
	buf.WriteString("- Do not hardcode secrets or API keys\n")
	buf.WriteString("- Handle errors gracefully\n")
	buf.WriteString("- Keep code minimal and focused\n")

	return os.WriteFile(filepath.Join(workDir, "CONTEXT.md"), []byte(buf.String()), 0644)
}

func (r *DockerRunner) collectFiles(workDir string) ([]string, error) {
	var files []string

	err := filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(workDir, path)
		if err != nil {
			return err
		}

		if rel != "CONTEXT.md" && !strings.HasPrefix(rel, ".") {
			files = append(files, rel)
		}

		return nil
	})

	return files, err
}
