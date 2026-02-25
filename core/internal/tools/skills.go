package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
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

	// read_skill_file reads a specific file from a multi-file skill
	readFileTool := llm.Tool{
		Name:        "read_skill_file",
		Description: "Read a specific file from a multi-file skill. Use this to access rule files or submodules referenced by the main skill.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"skill": map[string]any{
					"type":        "string",
					"description": "Name of the skill (e.g., 'remotion')",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Relative path to the file within the skill (e.g., 'rules/animations.md')",
				},
			},
			"required": []string{"skill", "path"},
		},
	}

	registry.Register(readFileTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Skill string `json:"skill"`
			Path  string `json:"path"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		content, err := manager.ReadFile(params.Skill, params.Path)
		if err != nil {
			return "", err
		}

		return content, nil
	})

	installTool := llm.Tool{
		Name:        "install_skill",
		Description: "Install a skill from a URL. Supports single .md files or GitHub directories containing multiple files. GitHub directories are installed as skill folders with all subfiles preserved.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "URL to the skill. Can be: GitHub directory (github.com/owner/repo/tree/branch/path), GitHub file (github.com/owner/repo/blob/branch/path.md), or raw URL to .md file",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "Name for the skill. If not provided, extracted from URL.",
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

		path, fileCount, err := manager.InstallFromURL(ctx, params.URL, name)
		if err != nil {
			return "", err
		}

		if fileCount > 1 {
			registry.Notify(ctx, fmt.Sprintf("✅ Skill installed: %s (%d files)", name, fileCount))
			return fmt.Sprintf("Skill installed: %s\nPath: %s\nFiles: %d", name, path, fileCount), nil
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
			if skill.IsDir {
				fmt.Fprintf(&sb, "- %s (multi-file): %s\n", skill.Name, skill.Description)
			} else {
				fmt.Fprintf(&sb, "- %s: %s\n", skill.Name, skill.Description)
			}
		}
		return sb.String(), nil
	})

	removeTool := llm.Tool{
		Name:        "remove_skill",
		Description: "Remove an installed skill by name. Removes both single-file and multi-file skills.",
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
		Description: "Read the main content of an installed skill. For multi-file skills, reads the main SKILL.md file.",
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

// githubInfo holds parsed GitHub URL information
type githubInfo struct {
	Owner  string
	Repo   string
	Branch string
	Path   string
	IsDir  bool
}

// parseGitHubURL extracts owner, repo, branch, and path from GitHub URLs
// Supports: github.com/owner/repo/tree/branch/path (dir) and github.com/owner/repo/blob/branch/path (file)
func parseGitHubURL(url string) (*githubInfo, bool) {
	// Match github.com/owner/repo/(tree|blob)/branch/path
	re := regexp.MustCompile(`github\.com/([^/]+)/([^/]+)/(tree|blob)/([^/]+)/(.+)`)
	matches := re.FindStringSubmatch(url)
	if len(matches) != 6 {
		return nil, false
	}

	return &githubInfo{
		Owner:  matches[1],
		Repo:   matches[2],
		Branch: matches[4],
		Path:   matches[5],
		IsDir:  matches[3] == "tree",
	}, true
}

// githubContent represents a file/directory from GitHub Contents API
type githubContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"` // "file" or "dir"
	DownloadURL string `json:"download_url"`
}

// fetchGitHubContents fetches directory contents from GitHub API
func fetchGitHubContents(ctx context.Context, owner, repo, path, branch string) ([]githubContent, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s", owner, repo, path, branch)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API error: %s", resp.Status)
	}

	var contents []githubContent
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return nil, err
	}

	return contents, nil
}

// downloadFile fetches a file from a URL
func downloadFile(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// InstallFromURL installs a skill from a URL. Returns path, file count, and error.
func (m *SkillsManager) InstallFromURL(ctx context.Context, url, name string) (string, int, error) {
	// Check if it's a GitHub URL
	ghInfo, isGitHub := parseGitHubURL(url)

	if isGitHub && ghInfo.IsDir {
		// GitHub directory - install as multi-file skill
		return m.installGitHubDir(ctx, ghInfo, name)
	}

	// Single file install
	var downloadURL string
	if isGitHub {
		// Convert blob URL to raw URL
		downloadURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s",
			ghInfo.Owner, ghInfo.Repo, ghInfo.Branch, ghInfo.Path)
	} else {
		downloadURL = url
	}

	content, err := downloadFile(ctx, downloadURL)
	if err != nil {
		return "", 0, fmt.Errorf("fetch skill: %w", err)
	}

	path, err := m.Save(name, string(content))
	return path, 1, err
}

// installGitHubDir installs a GitHub directory as a multi-file skill
func (m *SkillsManager) installGitHubDir(ctx context.Context, gh *githubInfo, name string) (string, int, error) {
	skillDir := filepath.Join(m.skillsDir, strings.ToLower(name))
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		return "", 0, fmt.Errorf("create skill dir: %w", err)
	}

	fileCount, err := m.downloadGitHubDirRecursive(ctx, gh.Owner, gh.Repo, gh.Path, gh.Branch, skillDir, "")
	if err != nil {
		os.RemoveAll(skillDir)
		return "", 0, err
	}

	return skillDir, fileCount, nil
}

// downloadGitHubDirRecursive downloads all files from a GitHub directory recursively
func (m *SkillsManager) downloadGitHubDirRecursive(ctx context.Context, owner, repo, path, branch, destDir, subPath string) (int, error) {
	contents, err := fetchGitHubContents(ctx, owner, repo, path, branch)
	if err != nil {
		return 0, err
	}

	fileCount := 0
	for _, item := range contents {
		localPath := filepath.Join(destDir, subPath, item.Name)

		if item.Type == "dir" {
			if err := os.MkdirAll(localPath, 0755); err != nil {
				return fileCount, err
			}
			count, err := m.downloadGitHubDirRecursive(ctx, owner, repo, item.Path, branch, destDir, filepath.Join(subPath, item.Name))
			if err != nil {
				return fileCount, err
			}
			fileCount += count
		} else if item.Type == "file" && item.DownloadURL != "" {
			data, err := downloadFile(ctx, item.DownloadURL)
			if err != nil {
				return fileCount, fmt.Errorf("download %s: %w", item.Name, err)
			}
			if err := os.WriteFile(localPath, data, 0644); err != nil {
				return fileCount, fmt.Errorf("write %s: %w", item.Name, err)
			}
			fileCount++
		}
	}

	return fileCount, nil
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
	// First check for directory-based skill
	dirPath := filepath.Join(m.skillsDir, strings.ToLower(name))
	if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
		// Look for SKILL.md or main .md file
		skillFile := filepath.Join(dirPath, "SKILL.md")
		if content, err := os.ReadFile(skillFile); err == nil {
			return string(content), nil
		}
		// Fallback to first .md file found
		entries, _ := os.ReadDir(dirPath)
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".md") {
				content, err := os.ReadFile(filepath.Join(dirPath, e.Name()))
				if err == nil {
					return string(content), nil
				}
			}
		}
		return "", fmt.Errorf("no .md files found in skill directory")
	}

	// Fall back to single file
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

// ReadFile reads a specific file from a multi-file skill
func (m *SkillsManager) ReadFile(skillName, relativePath string) (string, error) {
	dirPath := filepath.Join(m.skillsDir, strings.ToLower(skillName))
	if info, err := os.Stat(dirPath); err != nil || !info.IsDir() {
		return "", fmt.Errorf("skill '%s' is not a multi-file skill", skillName)
	}

	filePath := filepath.Join(dirPath, relativePath)
	// Security: ensure path is within skill directory
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return "", err
	}
	absDir, err := filepath.Abs(dirPath)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(absPath, absDir) {
		return "", fmt.Errorf("path traversal not allowed")
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	return string(content), nil
}

func (m *SkillsManager) Remove(name string) error {
	// Check for directory-based skill first
	dirPath := filepath.Join(m.skillsDir, strings.ToLower(name))
	if info, err := os.Stat(dirPath); err == nil && info.IsDir() {
		return os.RemoveAll(dirPath)
	}

	// Fall back to single file
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
	IsDir       bool
}

func (m *SkillsManager) List() ([]SkillInfo, error) {
	entries, err := os.ReadDir(m.skillsDir)
	if err != nil {
		return nil, err
	}

	var skills []SkillInfo
	for _, entry := range entries {
		name := entry.Name()
		path := filepath.Join(m.skillsDir, name)

		if entry.IsDir() {
			// Multi-file skill - look for SKILL.md
			skillFile := filepath.Join(path, "SKILL.md")
			content, err := os.ReadFile(skillFile)
			if err != nil {
				// Try first .md file
				subEntries, _ := os.ReadDir(path)
				for _, se := range subEntries {
					if !se.IsDir() && strings.HasSuffix(strings.ToLower(se.Name()), ".md") {
						content, _ = os.ReadFile(filepath.Join(path, se.Name()))
						break
					}
				}
			}
			desc := "Multi-file skill"
			if len(content) > 0 {
				desc = extractDescription(string(content))
			}
			skills = append(skills, SkillInfo{
				Name:        name,
				Description: desc,
				Path:        path,
				IsDir:       true,
			})
		} else if strings.HasSuffix(strings.ToLower(name), ".md") {
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
				IsDir:       false,
			})
		}
	}

	return skills, nil
}

func extractSkillName(url string) string {
	// For GitHub directory URLs, use the last path component
	if gh, ok := parseGitHubURL(url); ok {
		parts := strings.Split(gh.Path, "/")
		name := parts[len(parts)-1]
		name = strings.TrimSuffix(name, ".md")
		name = strings.TrimSuffix(name, ".MD")
		if name != "" {
			return name
		}
	}

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
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "---") {
			continue
		}
		if len(line) > 100 {
			return line[:100] + "..."
		}
		return line
	}
	return "No description"
}
