package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bowerhall/sheldon/internal/llm"
)

type SkillsManager struct {
	skillsDir string
}

func NewSkillsManager(skillsDir string) (*SkillsManager, error) {
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return nil, fmt.Errorf("create skills dir: %w", err)
	}
	return &SkillsManager{skillsDir: skillsDir}, nil
}

func RegisterSkillsTools(registry *Registry, manager *SkillsManager) {
	// use_skill loads a skill's instructions into the current context
	useTool := llm.Tool{
		Name:        "use_skill",
		Description: "Activate a skill by loading its instructions. Use this when you need specialized behavior defined in a skill. The skill's instructions will guide your next actions.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Name of the skill to activate",
				},
			},
			"required": []string{"name"},
		},
	}

	registry.Register(useTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		content, err := manager.Read(params.Name)
		if err != nil {
			return "", fmt.Errorf("skill not found: %s", params.Name)
		}

		return fmt.Sprintf("=== SKILL ACTIVATED: %s ===\n\n%s\n\n=== END SKILL ===\n\nFollow the instructions above to complete the task.", params.Name, content), nil
	})
	installTool := llm.Tool{
		Name:        "install_skill",
		Description: "Install a skill from a URL. Fetches the skill markdown file and saves it to the skills directory. Supports GitHub raw URLs, MinIO URLs, or any public URL to an MD file.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "URL to the skill markdown file (e.g., GitHub raw URL, MinIO URL)",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Name for the skill file (without .md extension). If not provided, extracted from URL.",
				},
			},
			"required": []string{"url"},
		},
	}

	registry.Register(installTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			URL  string `json:"url"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		name := params.Name
		if name == "" {
			name = extractSkillName(params.URL)
		}

		registry.Notify(ctx, fmt.Sprintf("📥 Installing skill: %s", name))

		path, err := manager.InstallFromURL(ctx, params.URL, name)
		if err != nil {
			return "", err
		}

		registry.Notify(ctx, fmt.Sprintf("✅ Skill installed: %s", name))
		return fmt.Sprintf("Skill installed: %s\nPath: %s", name, path), nil
	})

	listTool := llm.Tool{
		Name:        "list_skills",
		Description: "List all installed skills. Shows skill names and descriptions.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}

	registry.Register(listTool, func(ctx context.Context, args string) (string, error) {
		skills, err := manager.List()
		if err != nil {
			return "", err
		}

		if len(skills) == 0 {
			return "No skills installed.", nil
		}

		var sb strings.Builder
		sb.WriteString("Installed skills:\n")
		for _, skill := range skills {
			fmt.Fprintf(&sb, "- %s: %s\n", skill.Name, skill.Description)
		}
		return sb.String(), nil
	})

	removeTool := llm.Tool{
		Name:        "remove_skill",
		Description: "Remove an installed skill by name.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Name of the skill to remove",
				},
			},
			"required": []string{"name"},
		},
	}

	registry.Register(removeTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		if err := manager.Remove(params.Name); err != nil {
			return "", err
		}

		return fmt.Sprintf("Skill removed: %s", params.Name), nil
	})

	readTool := llm.Tool{
		Name:        "read_skill",
		Description: "Read the contents of an installed skill to understand how it works.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Name of the skill to read",
				},
			},
			"required": []string{"name"},
		},
	}

	registry.Register(readTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		content, err := manager.Read(params.Name)
		if err != nil {
			return "", err
		}

		return content, nil
	})

	saveTool := llm.Tool{
		Name:        "save_skill",
		Description: "Save or update a skill. Use this to create a new skill or modify an existing one. The content should be valid skill markdown format.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Name for the skill (without .md extension)",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "The skill markdown content",
				},
			},
			"required": []string{"name", "content"},
		},
	}

	registry.Register(saveTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Name    string `json:"name"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		path, err := manager.Save(params.Name, params.Content)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("Skill saved: %s\nPath: %s", params.Name, path), nil
	})
}

func (m *SkillsManager) InstallFromURL(ctx context.Context, url, name string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch skill: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch failed: %s", resp.Status)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	return m.Save(name, string(content))
}

func (m *SkillsManager) Save(name, content string) (string, error) {
	filename := strings.ToUpper(name)
	if !strings.HasSuffix(filename, ".MD") {
		filename += ".md"
	}

	path := filepath.Join(m.skillsDir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write skill: %w", err)
	}

	return path, nil
}

func (m *SkillsManager) Read(name string) (string, error) {
	filename := strings.ToUpper(name)
	if !strings.HasSuffix(filename, ".MD") {
		filename += ".md"
	}

	path := filepath.Join(m.skillsDir, filename)
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read skill: %w", err)
	}

	return string(content), nil
}

func (m *SkillsManager) Remove(name string) error {
	filename := strings.ToUpper(name)
	if !strings.HasSuffix(filename, ".MD") {
		filename += ".md"
	}

	path := filepath.Join(m.skillsDir, filename)
	return os.Remove(path)
}

type SkillInfo struct {
	Name        string
	Description string
	Path        string
}

func (m *SkillsManager) List() ([]SkillInfo, error) {
	entries, err := os.ReadDir(m.skillsDir)
	if err != nil {
		return nil, err
	}

	var skills []SkillInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}

		path := filepath.Join(m.skillsDir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		desc := extractDescription(string(content))
		skillName := strings.TrimSuffix(name, ".md")
		skillName = strings.TrimSuffix(skillName, ".MD")

		skills = append(skills, SkillInfo{
			Name:        skillName,
			Description: desc,
			Path:        path,
		})
	}

	return skills, nil
}

func extractSkillName(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) == 0 {
		return "unnamed"
	}

	filename := parts[len(parts)-1]
	filename = strings.TrimSuffix(filename, ".md")
	filename = strings.TrimSuffix(filename, ".MD")
	filename = strings.TrimPrefix(filename, "SKILL-")
	filename = strings.TrimPrefix(filename, "SKILL_")

	if filename == "" {
		return "unnamed"
	}
	return filename
}

func extractDescription(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if len(line) > 100 {
			return line[:100] + "..."
		}
		return line
	}
	return "No description"
}
