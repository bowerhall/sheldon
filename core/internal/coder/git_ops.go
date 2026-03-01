package coder

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bowerhall/sheldon/internal/logger"
)

// GitOps handles git operations outside of the coder container.
// This prevents the coder from having access to GIT_TOKEN, avoiding
// prompt injection attacks that could steal credentials.
type GitOps struct {
	userName  string
	userEmail string
	orgURL    string
	token     string
}

// NewGitOps creates a new GitOps instance
func NewGitOps(cfg GitConfig) *GitOps {
	return &GitOps{
		userName:  cfg.UserName,
		userEmail: cfg.UserEmail,
		orgURL:    cfg.OrgURL,
		token:     cfg.Token,
	}
}

// CloneRepo clones a repository into the workspace directory.
// If the repo doesn't exist, it initializes an empty git repo.
func (g *GitOps) CloneRepo(ctx context.Context, repoName, workspacePath string) error {
	if g.token == "" || g.orgURL == "" {
		return fmt.Errorf("git not configured (missing token or org URL)")
	}

	// Build clone URL without embedded token (token passed via env)
	cloneURL, err := g.buildCloneURL(repoName)
	if err != nil {
		return fmt.Errorf("build clone URL: %w", err)
	}

	// Clone using GIT_ASKPASS to provide credentials securely
	// This avoids exposing the token in ps aux
	cmd := exec.CommandContext(ctx, "git", "clone", cloneURL, workspacePath)
	cmd.Env = g.gitEnvWithAuth()
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Check if repo doesn't exist (404)
		if strings.Contains(string(output), "not found") ||
			strings.Contains(string(output), "Repository not found") ||
			strings.Contains(string(output), "does not exist") {
			logger.Debug("repo doesn't exist, initializing new repo", "repo", repoName)
			return g.initRepo(ctx, workspacePath)
		}
		return fmt.Errorf("git clone failed: %s", string(output))
	}

	// Configure git user in the cloned repo
	if err := g.configureGitUser(ctx, workspacePath); err != nil {
		logger.Warn("failed to configure git user", "error", err)
	}

	logger.Debug("cloned repo", "repo", repoName, "path", workspacePath)
	return nil
}

// initRepo initializes a new git repository
func (g *GitOps) initRepo(ctx context.Context, workspacePath string) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}

	// git init
	cmd := exec.CommandContext(ctx, "git", "init")
	cmd.Dir = workspacePath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git init: %s", string(output))
	}

	// Configure git user
	if err := g.configureGitUser(ctx, workspacePath); err != nil {
		return err
	}

	logger.Debug("initialized new repo", "path", workspacePath)
	return nil
}

// configureGitUser sets up git user.name and user.email in the workspace
func (g *GitOps) configureGitUser(ctx context.Context, workspacePath string) error {
	if g.userName != "" {
		cmd := exec.CommandContext(ctx, "git", "config", "user.name", g.userName)
		cmd.Dir = workspacePath
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("set user.name: %w", err)
		}
	}

	if g.userEmail != "" {
		cmd := exec.CommandContext(ctx, "git", "config", "user.email", g.userEmail)
		cmd.Dir = workspacePath
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("set user.email: %w", err)
		}
	}

	return nil
}

// PushChanges commits any changes and pushes to a feature branch.
// Returns true if changes were pushed, false if nothing to push.
func (g *GitOps) PushChanges(ctx context.Context, workspacePath, repoName, branchName string) (bool, error) {
	if g.token == "" || g.orgURL == "" {
		return false, fmt.Errorf("git not configured")
	}

	// Check if there are any changes
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = workspacePath
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}

	if len(strings.TrimSpace(string(output))) == 0 {
		logger.Debug("no changes to push", "path", workspacePath)
		return false, nil
	}

	// Create and checkout branch
	if branchName == "" {
		branchName = "sheldon/auto"
	}

	cmd = exec.CommandContext(ctx, "git", "checkout", "-B", branchName)
	cmd.Dir = workspacePath
	if output, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("checkout branch: %s", string(output))
	}

	// Add all changes
	cmd = exec.CommandContext(ctx, "git", "add", "-A")
	cmd.Dir = workspacePath
	if output, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("git add: %s", string(output))
	}

	// Commit
	cmd = exec.CommandContext(ctx, "git", "commit", "-m", "chore: automated changes by Sheldon")
	cmd.Dir = workspacePath
	if output, err := cmd.CombinedOutput(); err != nil {
		// Check if nothing to commit
		if strings.Contains(string(output), "nothing to commit") {
			return false, nil
		}
		return false, fmt.Errorf("git commit: %s", string(output))
	}

	// Set remote with clean URL (no embedded token)
	remoteURL, err := g.buildCloneURL(repoName)
	if err != nil {
		return false, fmt.Errorf("build remote URL: %w", err)
	}

	// Check if remote exists
	cmd = exec.CommandContext(ctx, "git", "remote", "get-url", "origin")
	cmd.Dir = workspacePath
	if err := cmd.Run(); err != nil {
		// Add remote
		cmd = exec.CommandContext(ctx, "git", "remote", "add", "origin", remoteURL)
		cmd.Dir = workspacePath
		if output, err := cmd.CombinedOutput(); err != nil {
			return false, fmt.Errorf("add remote: %s", string(output))
		}
	} else {
		// Update remote URL
		cmd = exec.CommandContext(ctx, "git", "remote", "set-url", "origin", remoteURL)
		cmd.Dir = workspacePath
		if output, err := cmd.CombinedOutput(); err != nil {
			return false, fmt.Errorf("set remote: %s", string(output))
		}
	}

	// Push with credentials via environment (not in URL)
	cmd = exec.CommandContext(ctx, "git", "push", "-u", "origin", branchName, "--force")
	cmd.Dir = workspacePath
	cmd.Env = g.gitEnvWithAuth()
	if output, err := cmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("git push: %s", string(output))
	}

	logger.Debug("pushed changes", "repo", repoName, "branch", branchName)
	return true, nil
}

// CreateRepo creates a new repository using gh CLI
func (g *GitOps) CreateRepo(ctx context.Context, repoName string, private bool) error {
	visibility := "--public"
	if private {
		visibility = "--private"
	}

	// Extract org from orgURL
	org := g.extractOrg()
	if org == "" {
		return fmt.Errorf("could not extract org from URL: %s", g.orgURL)
	}

	fullName := fmt.Sprintf("%s/%s", org, repoName)

	cmd := exec.CommandContext(ctx, "gh", "repo", "create", fullName, visibility, "--confirm")
	cmd.Env = append(os.Environ(), "GH_TOKEN="+g.token)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh repo create: %s", string(output))
	}

	logger.Debug("created repo", "name", fullName)
	return nil
}

// buildCloneURL constructs a git URL without embedded credentials
func (g *GitOps) buildCloneURL(repoName string) (string, error) {
	parsed, err := url.Parse(g.orgURL)
	if err != nil {
		return "", fmt.Errorf("parse org URL: %w", err)
	}

	// URL without token - credentials provided via GIT_ASKPASS
	return fmt.Sprintf("https://%s%s/%s.git", parsed.Host, parsed.Path, repoName), nil
}

// gitEnvWithAuth returns environment variables for git commands with secure auth.
// Uses git credential helper to provide credentials without exposing them in process args.
func (g *GitOps) gitEnvWithAuth() []string {
	env := os.Environ()

	// Parse org URL to get the host
	parsed, _ := url.Parse(g.orgURL)
	host := "github.com"
	if parsed != nil && parsed.Host != "" {
		host = parsed.Host
	}

	// Use git credential helper via environment
	// GIT_CONFIG_COUNT + GIT_CONFIG_KEY/VALUE allows setting config without files
	env = append(env,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=credential.helper",
		// The helper script echoes username and password from env vars
		fmt.Sprintf("GIT_CONFIG_VALUE_0=!f() { echo username=x-access-token; echo password=$GIT_TOKEN; }; f"),
		"GIT_TOKEN="+g.token,
		// Also set credential for specific host
		fmt.Sprintf("GIT_CREDENTIAL_%s_USERNAME=x-access-token", strings.ToUpper(strings.ReplaceAll(host, ".", "_"))),
	)

	return env
}

// buildAuthURL constructs an authenticated git URL (deprecated, kept for gh CLI)
func (g *GitOps) buildAuthURL(repoName string) (string, error) {
	parsed, err := url.Parse(g.orgURL)
	if err != nil {
		return "", fmt.Errorf("parse org URL: %w", err)
	}

	// Build: https://TOKEN@github.com/org/repo.git
	authURL := fmt.Sprintf("https://%s@%s%s/%s.git",
		g.token,
		parsed.Host,
		parsed.Path,
		repoName,
	)

	return authURL, nil
}

// extractOrg extracts the organization name from orgURL
func (g *GitOps) extractOrg() string {
	parsed, err := url.Parse(g.orgURL)
	if err != nil {
		return ""
	}
	// Path is like "/myorg" - trim leading slash
	return strings.TrimPrefix(parsed.Path, "/")
}

// HasChanges checks if the workspace has uncommitted changes
func (g *GitOps) HasChanges(ctx context.Context, workspacePath string) (bool, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = workspacePath
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// GetDiff returns the git diff of uncommitted changes
func (g *GitOps) GetDiff(ctx context.Context, workspacePath string) (string, error) {
	// Get staged + unstaged diff
	cmd := exec.CommandContext(ctx, "git", "diff", "HEAD")
	cmd.Dir = workspacePath
	output, err := cmd.Output()
	if err != nil {
		// Maybe no commits yet, try without HEAD
		cmd = exec.CommandContext(ctx, "git", "diff")
		cmd.Dir = workspacePath
		output, err = cmd.Output()
		if err != nil {
			return "", err
		}
	}
	return string(output), nil
}

// IsGitRepo checks if the path is a git repository
func IsGitRepo(path string) bool {
	gitDir := filepath.Join(path, ".git")
	info, err := os.Stat(gitDir)
	return err == nil && info.IsDir()
}
