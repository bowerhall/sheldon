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
	if b.useIsolated && b.dockerRunner != nil {
		return b.dockerRunner.CleanupOld(maxAge)
	}
	return b.sandbox.CleanupOld(maxAge)
}

type Bridge struct {
	sandbox      *Sandbox
	dockerRunner *DockerRunner
	skills       *Skills
	useIsolated  bool
	// git operations (handled externally, not by coder)
	gitOps *GitOps
}

// BridgeConfig holds configuration for the Bridge
type BridgeConfig struct {
	SandboxDir string
	Provider   string // provider for coder LLM (kimi, claude, nvidia, ollama)
	Model      string // model to use (default: kimi-k2.5:cloud)
	SkillsDir  string // directory with skill templates
	Isolated   bool   // use ephemeral Docker containers
	Image      string // coder container image
	// git integration
	GitEnabled   bool
	GitUserName  string
	GitUserEmail string
	GitOrgURL    string
	GitToken     string
}

func NewBridge(sandboxDir, provider, model string) (*Bridge, error) {
	return NewBridgeWithConfig(BridgeConfig{
		SandboxDir: sandboxDir,
		Provider:   provider,
		Model:      model,
		Isolated:   false,
	})
}

// NewBridgeWithConfig creates a Bridge with full configuration
func NewBridgeWithConfig(cfg BridgeConfig) (*Bridge, error) {
	gitCfg := GitConfig{
		Enabled:   cfg.GitEnabled,
		UserName:  cfg.GitUserName,
		UserEmail: cfg.GitUserEmail,
		OrgURL:    cfg.GitOrgURL,
		Token:     cfg.GitToken,
	}

	b := &Bridge{
		useIsolated: cfg.Isolated,
		gitOps:      NewGitOps(gitCfg),
	}

	// load skills if directory is configured
	if cfg.SkillsDir != "" {
		b.skills = NewSkills(cfg.SkillsDir)
	}

	if cfg.Isolated {
		b.dockerRunner = NewDockerRunner(DockerRunnerConfig{
			Image:        cfg.Image,
			ArtifactsDir: cfg.SandboxDir,
			Provider:     cfg.Provider,
			Model:        cfg.Model,
			Git:          gitCfg,
		})
		logger.Info("coder bridge using isolated containers", "image", cfg.Image)
	} else {
		sandbox, err := NewSandboxWithGit(cfg.SandboxDir, cfg.Provider, cfg.Model, gitCfg)
		if err != nil {
			return nil, err
		}
		b.sandbox = sandbox
		logger.Info("coder bridge using subprocess", "model", cfg.Model)
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

	// use Docker containers if isolated mode
	if b.useIsolated && b.dockerRunner != nil {
		return b.executeWithDocker(ctx, task, cfg)
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

// enrichPromptWithGitContext adds git repo context to the prompt
// Note: The repo has already been cloned by Sheldon before passing to coder.
// Coder should NOT have access to git push credentials.
func (b *Bridge) enrichPromptWithGitContext(prompt, gitRepo string, repoCloned bool) string {
	if gitRepo == "" {
		return prompt
	}

	var gitContext strings.Builder
	gitContext.WriteString("\n\n## Git Repository Context\n")
	gitContext.WriteString(fmt.Sprintf("- Working on project: %s\n", gitRepo))

	if repoCloned {
		gitContext.WriteString("- The repository has been cloned to your workspace\n")
		gitContext.WriteString("- Make your changes directly - git push will be handled automatically\n")
	} else {
		gitContext.WriteString("- This is a new project - create the files from scratch\n")
	}

	gitContext.WriteString("\n### Instructions:\n")
	gitContext.WriteString("- Focus on writing code - do NOT run git clone/push commands\n")
	gitContext.WriteString("- Use conventional commits locally if helpful (feat:, fix:, chore:)\n")
	gitContext.WriteString("- Changes will be pushed automatically when you're done\n")

	return prompt + gitContext.String()
}

func (b *Bridge) executeWithDocker(ctx context.Context, task Task, cfg struct {
	MaxTurns int
	Timeout  time.Duration
}) (*Result, error) {
	logger.Debug("coder starting via docker", "task", task.ID, "complexity", task.Complexity)

	taskCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	// Get workspace path for this task
	workDir, _ := b.GetLocalWorkspacePath(ctx, task.ID)

	// Clone repo before passing to coder (if GitRepo is set)
	repoCloned := false
	if task.GitRepo != "" && b.gitOps != nil {
		if err := b.gitOps.CloneRepo(taskCtx, task.GitRepo, workDir); err != nil {
			logger.Warn("git clone failed, proceeding without repo", "error", err, "repo", task.GitRepo)
		} else {
			repoCloned = true
			logger.Debug("cloned repo for coder", "repo", task.GitRepo, "path", workDir)
		}
	}

	result, err := b.dockerRunner.RunJob(taskCtx, JobConfig{
		TaskID:   task.ID,
		Prompt:   b.enrichPromptWithGitContext(task.Prompt, task.GitRepo, repoCloned),
		MaxTurns: cfg.MaxTurns,
		Timeout:  cfg.Timeout,
		Context:  task.Context,
		GitRepo:  task.GitRepo,
	})

	if err != nil {
		logger.Error("coder docker job failed", "error", err, "task", task.ID)
	} else {
		logger.Debug("coder docker job complete",
			"task", task.ID,
			"duration", result.Duration,
			"files", len(result.Files),
			"sanitized", result.Sanitized,
		)

		// Push changes after coder completes (if GitRepo is set)
		if task.GitRepo != "" && b.gitOps != nil {
			branchName := "sheldon/" + task.ID
			pushed, pushErr := b.gitOps.PushChanges(taskCtx, result.WorkspacePath, task.GitRepo, branchName)
			if pushErr != nil {
				logger.Error("git push failed", "error", pushErr, "repo", task.GitRepo)
				result.GitError = pushErr.Error()
			} else if pushed {
				logger.Debug("pushed changes to repo", "repo", task.GitRepo, "branch", branchName)
				result.GitPushed = true
				result.GitBranch = branchName
			}
		}
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

	// Clone repo before passing to coder (if GitRepo is set)
	repoCloned := false
	if task.GitRepo != "" && b.gitOps != nil {
		if err := b.gitOps.CloneRepo(taskCtx, task.GitRepo, ws.Path); err != nil {
			logger.Warn("git clone failed, proceeding without repo", "error", err, "repo", task.GitRepo)
		} else {
			repoCloned = true
			logger.Debug("cloned repo for coder", "repo", task.GitRepo, "path", ws.Path)
		}
	}

	// don't cleanup - workspace persists for build_image/deploy
	// cleanup happens via periodic cleanup or cleanup_images tool

	if err := b.sandbox.WriteContext(ws, task.Context); err != nil {
		return nil, fmt.Errorf("write context: %w", err)
	}

	logger.Debug("claude code starting", "task", task.ID, "complexity", task.Complexity)

	// Enrich prompt with git context if applicable
	prompt := task.Prompt
	if task.GitRepo != "" {
		prompt = b.enrichPromptWithGitContext(prompt, task.GitRepo, repoCloned)
	}

	output, err := b.run(taskCtx, ws, prompt, cfg.MaxTurns)
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

	// Push changes after coder completes (if GitRepo is set)
	if task.GitRepo != "" && b.gitOps != nil && result.Error == "" {
		branchName := "sheldon/" + task.ID
		pushed, pushErr := b.gitOps.PushChanges(taskCtx, ws.Path, task.GitRepo, branchName)
		if pushErr != nil {
			logger.Error("git push failed", "error", pushErr, "repo", task.GitRepo)
			result.GitError = pushErr.Error()
		} else if pushed {
			logger.Debug("pushed changes to repo", "repo", task.GitRepo, "branch", branchName)
			result.GitPushed = true
			result.GitBranch = branchName
		}
	}

	return result, nil
}

func (b *Bridge) run(ctx context.Context, ws *Workspace, prompt string, maxTurns int) (string, error) {
	// Build ollama launch claude command with model from env
	model := b.sandbox.model
	if model == "" {
		model = "kimi-k2.5"
	}

	// ollama launch claude --model MODEL -- CLAUDE_ARGS
	args := []string{
		"launch", "claude",
		"--model", model,
		"--",
		"--print",
		"--output-format", "text",
		"--max-turns", fmt.Sprintf("%d", maxTurns),
		"--dangerously-skip-permissions",
		"-p", prompt,
	}

	cmd := exec.CommandContext(ctx, "ollama", args...)
	cmd.Dir = ws.Path
	cmd.Env = b.sandbox.CleanEnv()

	logger.Debug("ollama launch claude command", "dir", ws.Path, "model", model)

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

	// use Docker containers if isolated mode
	if b.useIsolated && b.dockerRunner != nil {
		return b.executeWithDockerProgress(ctx, task, cfg, onProgress)
	}

	return b.executeWithSubprocessProgress(ctx, task, cfg, onProgress)
}

func (b *Bridge) executeWithDockerProgress(ctx context.Context, task Task, cfg struct {
	MaxTurns int
	Timeout  time.Duration
}, onProgress func(StreamEvent)) (*Result, error) {
	logger.Debug("coder starting via docker", "task", task.ID, "complexity", task.Complexity)

	taskCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	// Get workspace path for this task
	workDir, _ := b.GetLocalWorkspacePath(ctx, task.ID)

	// Clone repo before passing to coder (if GitRepo is set)
	repoCloned := false
	if task.GitRepo != "" && b.gitOps != nil {
		if err := b.gitOps.CloneRepo(taskCtx, task.GitRepo, workDir); err != nil {
			logger.Warn("git clone failed, proceeding without repo", "error", err, "repo", task.GitRepo)
		} else {
			repoCloned = true
			logger.Debug("cloned repo for coder", "repo", task.GitRepo, "path", workDir)
		}
	}

	result, err := b.dockerRunner.RunJobWithProgress(taskCtx, JobConfig{
		TaskID:   task.ID,
		Prompt:   b.enrichPromptWithGitContext(task.Prompt, task.GitRepo, repoCloned),
		MaxTurns: cfg.MaxTurns,
		Timeout:  cfg.Timeout,
		Context:  task.Context,
		GitRepo:  task.GitRepo,
	}, onProgress)

	if err != nil {
		logger.Error("coder docker job failed", "error", err, "task", task.ID)
	} else {
		logger.Debug("coder docker job complete",
			"task", task.ID,
			"duration", result.Duration,
			"files", len(result.Files),
		)

		// Push changes after coder completes (if GitRepo is set)
		if task.GitRepo != "" && b.gitOps != nil {
			branchName := "sheldon/" + task.ID
			pushed, pushErr := b.gitOps.PushChanges(taskCtx, result.WorkspacePath, task.GitRepo, branchName)
			if pushErr != nil {
				logger.Error("git push failed", "error", pushErr, "repo", task.GitRepo)
				result.GitError = pushErr.Error()
			} else if pushed {
				logger.Debug("pushed changes to repo", "repo", task.GitRepo, "branch", branchName)
				result.GitPushed = true
				result.GitBranch = branchName
			}
		}
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

	// Clone repo before passing to coder (if GitRepo is set)
	repoCloned := false
	if task.GitRepo != "" && b.gitOps != nil {
		if err := b.gitOps.CloneRepo(taskCtx, task.GitRepo, ws.Path); err != nil {
			logger.Warn("git clone failed, proceeding without repo", "error", err, "repo", task.GitRepo)
		} else {
			repoCloned = true
			logger.Debug("cloned repo for coder", "repo", task.GitRepo, "path", ws.Path)
		}
	}

	// don't cleanup - workspace persists for build_image/deploy

	if err := b.sandbox.WriteContext(ws, task.Context); err != nil {
		return nil, fmt.Errorf("write context: %w", err)
	}

	logger.Debug("claude code starting", "task", task.ID, "complexity", task.Complexity)

	// Enrich prompt with git context if applicable
	prompt := task.Prompt
	if task.GitRepo != "" {
		prompt = b.enrichPromptWithGitContext(prompt, task.GitRepo, repoCloned)
	}

	output, err := b.runWithProgress(taskCtx, ws, prompt, cfg.MaxTurns, onProgress)
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

	// Push changes after coder completes (if GitRepo is set)
	if task.GitRepo != "" && b.gitOps != nil && result.Error == "" {
		branchName := "sheldon/" + task.ID
		pushed, pushErr := b.gitOps.PushChanges(taskCtx, ws.Path, task.GitRepo, branchName)
		if pushErr != nil {
			logger.Error("git push failed", "error", pushErr, "repo", task.GitRepo)
			result.GitError = pushErr.Error()
		} else if pushed {
			logger.Debug("pushed changes to repo", "repo", task.GitRepo, "branch", branchName)
			result.GitPushed = true
			result.GitBranch = branchName
		}
	}

	return result, nil
}

func (b *Bridge) runWithProgress(ctx context.Context, ws *Workspace, prompt string, maxTurns int, onProgress func(StreamEvent)) (string, error) {
	// Build ollama launch claude command with model from env
	model := b.sandbox.model
	if model == "" {
		model = "kimi-k2.5"
	}

	// ollama launch claude --model MODEL -- CLAUDE_ARGS
	args := []string{
		"launch", "claude",
		"--model", model,
		"--",
		"--print",
		"--verbose",
		"--output-format", "stream-json",
		"--max-turns", fmt.Sprintf("%d", maxTurns),
		"--dangerously-skip-permissions",
		"-p", prompt,
	}

	cmd := exec.CommandContext(ctx, "ollama", args...)
	cmd.Dir = ws.Path
	cmd.Env = b.sandbox.CleanEnv()

	logger.Debug("ollama launch claude command (progress)", "dir", ws.Path, "model", model)

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
func (b *Bridge) GetLocalWorkspacePath(ctx context.Context, taskID string) (string, error) {
	if b.useIsolated && b.dockerRunner != nil {
		// docker mode - artifacts are in the configured artifacts dir
		return b.dockerRunner.artifactsDir + "/" + taskID, nil
	}
	// subprocess mode - path is in sandbox
	return b.sandbox.baseDir + "/" + taskID, nil
}

// CleanupTask removes artifacts for a completed task
func (b *Bridge) CleanupTask(ctx context.Context, taskID string) error {
	if b.useIsolated && b.dockerRunner != nil {
		return b.dockerRunner.CleanupArtifacts(taskID)
	}
	// subprocess mode - use sandbox cleanup
	return b.sandbox.Cleanup(&Workspace{TaskID: taskID, Path: b.sandbox.baseDir + "/" + taskID})
}

// IsUsingIsolatedContainers returns true if the bridge uses Docker containers
func (b *Bridge) IsUsingIsolatedContainers() bool {
	return b.useIsolated
}
