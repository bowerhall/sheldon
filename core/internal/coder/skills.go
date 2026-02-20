package coder

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kadet/kora/internal/logger"
)

// Skills manages coding skill templates
type Skills struct {
	dir    string
	skills map[string]string
}

// NewSkills creates a skills loader from a directory
func NewSkills(dir string) *Skills {
	s := &Skills{
		dir:    dir,
		skills: make(map[string]string),
	}
	s.load()
	return s
}

func (s *Skills) load() {
	if s.dir == "" {
		return
	}

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		logger.Debug("skills dir not found", "dir", s.dir)
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(s.dir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".md")
		s.skills[name] = string(content)
	}

	logger.Info("skills loaded", "count", len(s.skills))
}

// Get returns a skill by name
func (s *Skills) Get(name string) string {
	return s.skills[name]
}

// GetRelevant returns skills relevant to a prompt
// uses simple keyword matching for now
func (s *Skills) GetRelevant(prompt string) []string {
	prompt = strings.ToLower(prompt)
	var relevant []string

	// always include general guidelines
	if general, ok := s.skills["general"]; ok {
		relevant = append(relevant, general)
	}

	// match by keywords
	keywords := map[string][]string{
		"go-api":       {"go ", "golang", "api"},
		"python-api":   {"python", "fastapi", "flask"},
		"dockerfile":   {"docker", "container", "image"},
		"k8s-manifest": {"kubernetes", "k8s", "deploy", "manifest"},
	}

	for skill, kws := range keywords {
		for _, kw := range kws {
			if strings.Contains(prompt, kw) {
				if content, ok := s.skills[skill]; ok {
					relevant = append(relevant, content)
					break
				}
			}
		}
	}

	return relevant
}

// FormatForPrompt formats relevant skills for injection into a prompt
func (s *Skills) FormatForPrompt(prompt string) string {
	relevant := s.GetRelevant(prompt)
	if len(relevant) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n---\n# reference patterns\n\n")
	for _, skill := range relevant {
		sb.WriteString(skill)
		sb.WriteString("\n\n")
	}
	sb.WriteString("---\n")

	return sb.String()
}

// List returns all available skill names
func (s *Skills) List() []string {
	names := make([]string, 0, len(s.skills))
	for name := range s.skills {
		names = append(names, name)
	}
	return names
}
