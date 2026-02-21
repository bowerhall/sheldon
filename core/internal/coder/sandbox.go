package coder

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Sandbox struct {
	baseDir     string
	apiKey      string // NVIDIA NIM API key (primary)
	fallbackKey string // Moonshot Kimi API key (fallback)
	model       string // model to use (default: kimi-k2.5)
	git         GitConfig
}

type Workspace struct {
	Path   string
	TaskID string
}

func NewSandbox(baseDir, apiKey, fallbackKey, model string) (*Sandbox, error) {
	return NewSandboxWithGit(baseDir, apiKey, fallbackKey, model, GitConfig{})
}

func NewSandboxWithGit(baseDir, apiKey, fallbackKey, model string, git GitConfig) (*Sandbox, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create sandbox dir: %w", err)
	}

	if model == "" {
		model = "kimi-k2.5"
	}

	return &Sandbox{
		baseDir:     baseDir,
		apiKey:      apiKey,
		fallbackKey: fallbackKey,
		model:       model,
		git:         git,
	}, nil
}

func (s *Sandbox) Create(taskID string) (*Workspace, error) {
	path := filepath.Join(s.baseDir, taskID)

	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}

	return &Workspace{
		Path:   path,
		TaskID: taskID,
	}, nil
}

func (s *Sandbox) Cleanup(ws *Workspace) error {
	return os.RemoveAll(ws.Path)
}

func (s *Sandbox) CleanEnv() []string {
	env := []string{
		"HOME=/tmp",
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"LANG=en_US.UTF-8",
		"TERM=dumb",
		"SHELL=/bin/sh",
	}

	// NVIDIA NIM API key (primary)
	if s.apiKey != "" {
		env = append(env, "NVIDIA_API_KEY="+s.apiKey)
	}

	// Moonshot Kimi API key (fallback)
	if s.fallbackKey != "" {
		env = append(env, "KIMI_API_KEY="+s.fallbackKey)
	}

	// Model to use
	if s.model != "" {
		env = append(env, "CODER_MODEL="+s.model)
	}

	// Git user config (NOT the token - coder should never have access to GIT_TOKEN)
	// git clone/push is handled by Sheldon externally via GitOps
	if s.git.UserName != "" {
		env = append(env, "GIT_USER_NAME="+s.git.UserName)
	}
	if s.git.UserEmail != "" {
		env = append(env, "GIT_USER_EMAIL="+s.git.UserEmail)
	}

	return env
}

func (s *Sandbox) WriteContext(ws *Workspace, ctx *MemoryContext) error {
	if ctx == nil {
		return nil
	}

	var buf bytes.Buffer
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

	return os.WriteFile(filepath.Join(ws.Path, "CLAUDE.md"), buf.Bytes(), 0644)
}

func (s *Sandbox) CollectFiles(ws *Workspace) ([]string, error) {
	var files []string

	err := filepath.Walk(ws.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(ws.Path, path)
		if err != nil {
			return err
		}

		if rel != "CLAUDE.md" && !strings.HasPrefix(rel, ".") {
			files = append(files, rel)
		}

		return nil
	})

	return files, err
}

// CleanupOld removes workspaces older than maxAge and returns count of removed
func (s *Sandbox) CleanupOld(maxAge time.Duration) (int, error) {
	entries, err := os.ReadDir(s.baseDir)
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
			path := filepath.Join(s.baseDir, entry.Name())
			if err := os.RemoveAll(path); err == nil {
				removed++
			}
		}
	}

	return removed, nil
}

// ListWorkspaces returns all workspace directories
func (s *Sandbox) ListWorkspaces() ([]WorkspaceInfo, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, err
	}

	var workspaces []WorkspaceInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		workspaces = append(workspaces, WorkspaceInfo{
			TaskID:  entry.Name(),
			Path:    filepath.Join(s.baseDir, entry.Name()),
			ModTime: info.ModTime(),
		})
	}

	return workspaces, nil
}

type WorkspaceInfo struct {
	TaskID  string
	Path    string
	ModTime time.Time
}
