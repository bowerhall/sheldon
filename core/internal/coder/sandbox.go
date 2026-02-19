package coder

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Sandbox struct {
	baseDir string
	apiKey  string
	baseURL string
}

type Workspace struct {
	Path   string
	TaskID string
}

func NewSandbox(baseDir, apiKey, baseURL string) (*Sandbox, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create sandbox dir: %w", err)
	}

	return &Sandbox{
		baseDir: baseDir,
		apiKey:  apiKey,
		baseURL: baseURL,
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
	}

	if s.baseURL != "" {
		env = append(env, "ANTHROPIC_BASE_URL="+s.baseURL)
	}

	if s.apiKey != "" {
		env = append(env, "ANTHROPIC_API_KEY="+s.apiKey)
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
