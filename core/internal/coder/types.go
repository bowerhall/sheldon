package coder

import "time"

type Complexity string

const (
	ComplexitySimple   Complexity = "simple"
	ComplexityStandard Complexity = "standard"
	ComplexityComplex  Complexity = "complex"
)

var complexityConfig = map[Complexity]struct {
	MaxTurns int
	Timeout  time.Duration
}{
	ComplexitySimple:   {MaxTurns: 20, Timeout: 5 * time.Minute},
	ComplexityStandard: {MaxTurns: 50, Timeout: 10 * time.Minute},
	ComplexityComplex:  {MaxTurns: 100, Timeout: 20 * time.Minute},
}

type Task struct {
	ID          string
	Prompt      string
	Complexity  Complexity
	Context     *MemoryContext
	SystemHints string
	GitRepo     string // target repo name (e.g., "weather-bot")
}

type MemoryContext struct {
	UserPreferences map[string]string
	RelevantFacts   []Fact
	Constraints     []string
}

type Fact struct {
	Field string
	Value string
}

type Result struct {
	Output        string
	Files         []string
	WorkspacePath string
	TurnsUsed     int
	Duration      time.Duration
	Warnings      []string
	Sanitized     bool
	Error         string
}

type StreamEvent struct {
	Type    string
	Content string
	Tool    string
}

// GitConfig holds git integration settings
type GitConfig struct {
	Enabled   bool
	UserName  string
	UserEmail string
	OrgURL    string
	Token     string
}
