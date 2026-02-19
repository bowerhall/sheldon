package coder

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/kadet/kora/internal/logger"
)

type Bridge struct {
	sandbox *Sandbox
}

func NewBridge(sandboxDir, apiKey, baseURL string) (*Bridge, error) {
	sandbox, err := NewSandbox(sandboxDir, apiKey, baseURL)
	if err != nil {
		return nil, err
	}

	return &Bridge{sandbox: sandbox}, nil
}

func (b *Bridge) Execute(ctx context.Context, task Task) (*Result, error) {
	start := time.Now()
	result := &Result{}

	cfg, ok := complexityConfig[task.Complexity]
	if !ok {
		cfg = complexityConfig[ComplexityStandard]
	}

	taskCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	ws, err := b.sandbox.Create(task.ID)
	if err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}

	defer b.sandbox.Cleanup(ws)

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

	var output strings.Builder
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

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			logger.Debug("claude stderr", "line", scanner.Text())
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
		return output.String(), fmt.Errorf("exit: %w", err)
	}

	return output.String(), nil
}

func (b *Bridge) ExecuteWithProgress(ctx context.Context, task Task, onProgress func(StreamEvent)) (*Result, error) {
	start := time.Now()
	result := &Result{}

	cfg, ok := complexityConfig[task.Complexity]
	if !ok {
		cfg = complexityConfig[ComplexityStandard]
	}

	taskCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	ws, err := b.sandbox.Create(task.ID)
	if err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}
	defer b.sandbox.Cleanup(ws)

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

	return result, nil
}

func (b *Bridge) runWithProgress(ctx context.Context, ws *Workspace, prompt string, maxTurns int, onProgress func(StreamEvent)) (string, error) {
	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--max-turns", fmt.Sprintf("%d", maxTurns),
		"--dangerously-skip-permissions",
		"-p", prompt,
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = ws.Path
	cmd.Env = b.sandbox.CleanEnv()

	var output strings.Builder
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start: %w", err)
	}

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

		return output.String(), fmt.Errorf("exit: %w", err)
	}

	return output.String(), nil
}
