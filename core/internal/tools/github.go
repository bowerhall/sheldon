package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/bowerhall/sheldon/internal/config"
	"github.com/bowerhall/sheldon/internal/llm"
)

// RegisterGitHubTools registers GitHub-related tools (PR, repo management)
func RegisterGitHubTools(registry *Registry, cfg *config.GitConfig) {
	if cfg.Token == "" {
		return // no git token, skip registration
	}

	// open_pr tool
	openPRTool := llm.Tool{
		Name:        "open_pr",
		Description: "Open a pull request on a GitHub repository. Use this after pushing changes to a branch to request review and merge into main.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"repo": map[string]any{
					"type":        "string",
					"description": "Repository name (e.g., 'sheldon' or 'weather-bot'). Will use GIT_ORG_URL as the org.",
				},
				"branch": map[string]any{
					"type":        "string",
					"description": "Source branch name (e.g., 'feature/add-voice-support')",
				},
				"title": map[string]any{
					"type":        "string",
					"description": "PR title (e.g., 'Add voice support for Telegram')",
				},
				"body": map[string]any{
					"type":        "string",
					"description": "PR description explaining the changes",
				},
				"base": map[string]any{
					"type":        "string",
					"description": "Target branch to merge into (default: 'main')",
				},
			},
			"required": []string{"repo", "branch", "title"},
		},
	}

	registry.Register(openPRTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Repo   string `json:"repo"`
			Branch string `json:"branch"`
			Title  string `json:"title"`
			Body   string `json:"body"`
			Base   string `json:"base"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		if params.Base == "" {
			params.Base = "main"
		}

		// extract org from GIT_ORG_URL (e.g., "https://github.com/bowerhall" -> "bowerhall")
		org := extractOrg(cfg.OrgURL)
		if org == "" {
			return "", fmt.Errorf("GIT_ORG_URL not configured or invalid")
		}

		fullRepo := fmt.Sprintf("%s/%s", org, params.Repo)

		// use gh CLI to create PR
		cmdArgs := []string{
			"pr", "create",
			"--repo", fullRepo,
			"--head", params.Branch,
			"--base", params.Base,
			"--title", params.Title,
		}

		if params.Body != "" {
			cmdArgs = append(cmdArgs, "--body", params.Body)
		} else {
			cmdArgs = append(cmdArgs, "--body", "")
		}

		cmd := exec.CommandContext(ctx, "gh", cmdArgs...)
		cmd.Env = append(cmd.Environ(), "GH_TOKEN="+cfg.Token)

		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to create PR: %s", string(output))
		}

		return fmt.Sprintf("PR created: %s", strings.TrimSpace(string(output))), nil
	})

	// list_prs tool
	listPRsTool := llm.Tool{
		Name:        "list_prs",
		Description: "List open pull requests on a repository",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"repo": map[string]any{
					"type":        "string",
					"description": "Repository name (e.g., 'sheldon')",
				},
				"state": map[string]any{
					"type":        "string",
					"enum":        []string{"open", "closed", "merged", "all"},
					"description": "PR state filter (default: 'open')",
				},
			},
			"required": []string{"repo"},
		},
	}

	registry.Register(listPRsTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Repo  string `json:"repo"`
			State string `json:"state"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		if params.State == "" {
			params.State = "open"
		}

		org := extractOrg(cfg.OrgURL)
		if org == "" {
			return "", fmt.Errorf("GIT_ORG_URL not configured or invalid")
		}

		fullRepo := fmt.Sprintf("%s/%s", org, params.Repo)

		cmd := exec.CommandContext(ctx, "gh", "pr", "list",
			"--repo", fullRepo,
			"--state", params.State,
			"--json", "number,title,state,author,url",
		)
		cmd.Env = append(cmd.Environ(), "GH_TOKEN="+cfg.Token)

		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to list PRs: %s", string(output))
		}

		// parse and format output
		var prs []struct {
			Number int    `json:"number"`
			Title  string `json:"title"`
			State  string `json:"state"`
			Author struct {
				Login string `json:"login"`
			} `json:"author"`
			URL string `json:"url"`
		}
		if err := json.Unmarshal(output, &prs); err != nil {
			return string(output), nil // return raw if parse fails
		}

		if len(prs) == 0 {
			return fmt.Sprintf("No %s PRs in %s", params.State, params.Repo), nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("PRs in %s (%s):\n", params.Repo, params.State))
		for _, pr := range prs {
			fmt.Fprintf(&sb, "- #%d: %s (by %s)\n  %s\n", pr.Number, pr.Title, pr.Author.Login, pr.URL)
		}

		return sb.String(), nil
	})

	// create_repo tool
	createRepoTool := llm.Tool{
		Name:        "create_repo",
		Description: "Create a new repository in the configured GitHub org",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Repository name (e.g., 'weather-bot')",
				},
				"description": map[string]any{
					"type":        "string",
					"description": "Repository description",
				},
				"private": map[string]any{
					"type":        "boolean",
					"description": "Make repository private (default: false)",
				},
			},
			"required": []string{"name"},
		},
	}

	registry.Register(createRepoTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Private     bool   `json:"private"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		org := extractOrg(cfg.OrgURL)
		if org == "" {
			return "", fmt.Errorf("GIT_ORG_URL not configured or invalid")
		}

		cmdArgs := []string{"repo", "create", fmt.Sprintf("%s/%s", org, params.Name)}

		if params.Description != "" {
			cmdArgs = append(cmdArgs, "--description", params.Description)
		}

		if params.Private {
			cmdArgs = append(cmdArgs, "--private")
		} else {
			cmdArgs = append(cmdArgs, "--public")
		}

		cmd := exec.CommandContext(ctx, "gh", cmdArgs...)
		cmd.Env = append(cmd.Environ(), "GH_TOKEN="+cfg.Token)

		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("failed to create repo: %s", string(output))
		}

		return fmt.Sprintf("Repository created: %s/%s", org, params.Name), nil
	})
}

// extractOrg extracts org name from URL like "https://github.com/bowerhall" -> "bowerhall"
func extractOrg(orgURL string) string {
	orgURL = strings.TrimSuffix(orgURL, "/")
	parts := strings.Split(orgURL, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
