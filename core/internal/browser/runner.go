package browser

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/bowerhall/sheldon/internal/logger"
)

// Runner executes agent-browser commands in isolated containers
type Runner struct {
	image   string
	timeout time.Duration
}

// Config holds configuration for the browser runner
type Config struct {
	Image   string        // container image (default: sheldon-browser-sandbox:latest)
	Timeout time.Duration // command timeout (default: 60s)
}

// NewRunner creates a new browser runner
func NewRunner(cfg Config) *Runner {
	if cfg.Image == "" {
		cfg.Image = "sheldon-browser-sandbox:latest"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}

	return &Runner{
		image:   cfg.Image,
		timeout: cfg.Timeout,
	}
}

// allowedCommands defines the whitelist of safe agent-browser commands
var allowedCommands = map[string]bool{
	"open":       true,
	"snapshot":   true,
	"screenshot": true,
	"click":      true,
	"fill":       true,
	"type":       true,
	"press":      true,
	"hover":      true,
	"scroll":     true,
	"wait":       true,
	"get":        true,
	"find":       true,
	"is":         true,
	"close":      true,
}

// Run executes a sequence of agent-browser commands in a container
func (r *Runner) Run(ctx context.Context, commands []string) (string, error) {
	if len(commands) == 0 {
		return "", fmt.Errorf("no commands provided")
	}

	// validate all commands before execution
	for _, cmd := range commands {
		if err := r.validateCommand(cmd); err != nil {
			return "", err
		}
	}

	// build shell script to run commands sequentially
	var script strings.Builder
	script.WriteString("set -e\n")
	for _, cmd := range commands {
		script.WriteString(fmt.Sprintf("agent-browser %s\n", cmd))
	}

	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	args := []string{
		"run", "--rm",
		"--network=host", // needed for browser to access the internet
		"--shm-size=2g",  // needed for Chrome
		r.image,
		"sh", "-c", script.String(),
	}

	logger.Debug("browser runner executing", "commands", len(commands))

	cmd := exec.CommandContext(ctx, "docker", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("timeout after %s", r.timeout)
		}
		logger.Debug("browser runner stderr", "stderr", stderr.String())
		return "", fmt.Errorf("browser command failed: %w", err)
	}

	return stdout.String(), nil
}

// Browse opens a URL and returns a snapshot of the page
func (r *Runner) Browse(ctx context.Context, url string) (string, error) {
	// validate URL
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return "", fmt.Errorf("invalid URL: must start with http:// or https://")
	}

	commands := []string{
		fmt.Sprintf("open %q", url),
		"snapshot",
	}

	return r.Run(ctx, commands)
}

// Click clicks an element by reference
func (r *Runner) Click(ctx context.Context, ref string) (string, error) {
	commands := []string{
		fmt.Sprintf("click %s", ref),
		"snapshot",
	}

	return r.Run(ctx, commands)
}

// Fill fills a form field
func (r *Runner) Fill(ctx context.Context, ref, value string) (string, error) {
	commands := []string{
		fmt.Sprintf("fill %s %q", ref, value),
		"snapshot",
	}

	return r.Run(ctx, commands)
}

// GetText extracts text from an element
func (r *Runner) GetText(ctx context.Context, ref string) (string, error) {
	commands := []string{
		fmt.Sprintf("get text %s", ref),
	}

	return r.Run(ctx, commands)
}

// Screenshot captures a screenshot of the page
func (r *Runner) Screenshot(ctx context.Context, url string) ([]byte, error) {
	// for now, just return an error - would need to mount volume for file
	return nil, fmt.Errorf("screenshot not yet implemented")
}

// validateCommand checks if a command is in the allowlist
func (r *Runner) validateCommand(cmd string) error {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	baseCmd := parts[0]
	if !allowedCommands[baseCmd] {
		return fmt.Errorf("command not allowed: %s", baseCmd)
	}

	// additional safety: reject shell metacharacters and newlines
	dangerous := []string{";", "&", "|", "`", "$", "(", ")", "{", "}", "<", ">", "\\", "\n", "\r"}
	for _, d := range dangerous {
		if strings.Contains(cmd, d) {
			return fmt.Errorf("invalid characters in command")
		}
	}

	return nil
}
