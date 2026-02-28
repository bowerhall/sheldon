package config

import (
	"os"
	"testing"
)

func TestDetectProviderKimi(t *testing.T) {
	// save and restore env
	oldKimi := os.Getenv("KIMI_API_KEY")
	oldClaude := os.Getenv("ANTHROPIC_API_KEY")
	oldOpenAI := os.Getenv("OPENAI_API_KEY")
	defer func() {
		os.Setenv("KIMI_API_KEY", oldKimi)
		os.Setenv("ANTHROPIC_API_KEY", oldClaude)
		os.Setenv("OPENAI_API_KEY", oldOpenAI)
	}()

	os.Setenv("KIMI_API_KEY", "test-key")
	os.Setenv("ANTHROPIC_API_KEY", "")
	os.Setenv("OPENAI_API_KEY", "")

	provider := DetectProvider()
	if provider != "kimi" {
		t.Errorf("expected kimi, got %s", provider)
	}
}

func TestDetectProviderClaude(t *testing.T) {
	oldKimi := os.Getenv("KIMI_API_KEY")
	oldClaude := os.Getenv("ANTHROPIC_API_KEY")
	oldOpenAI := os.Getenv("OPENAI_API_KEY")
	defer func() {
		os.Setenv("KIMI_API_KEY", oldKimi)
		os.Setenv("ANTHROPIC_API_KEY", oldClaude)
		os.Setenv("OPENAI_API_KEY", oldOpenAI)
	}()

	os.Setenv("KIMI_API_KEY", "")
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	os.Setenv("OPENAI_API_KEY", "")

	provider := DetectProvider()
	if provider != "claude" {
		t.Errorf("expected claude, got %s", provider)
	}
}

func TestDetectProviderOpenAI(t *testing.T) {
	oldKimi := os.Getenv("KIMI_API_KEY")
	oldClaude := os.Getenv("ANTHROPIC_API_KEY")
	oldOpenAI := os.Getenv("OPENAI_API_KEY")
	defer func() {
		os.Setenv("KIMI_API_KEY", oldKimi)
		os.Setenv("ANTHROPIC_API_KEY", oldClaude)
		os.Setenv("OPENAI_API_KEY", oldOpenAI)
	}()

	os.Setenv("KIMI_API_KEY", "")
	os.Setenv("ANTHROPIC_API_KEY", "")
	os.Setenv("OPENAI_API_KEY", "test-key")

	provider := DetectProvider()
	if provider != "openai" {
		t.Errorf("expected openai, got %s", provider)
	}
}

func TestDetectProviderFallbackOllama(t *testing.T) {
	oldKimi := os.Getenv("KIMI_API_KEY")
	oldClaude := os.Getenv("ANTHROPIC_API_KEY")
	oldOpenAI := os.Getenv("OPENAI_API_KEY")
	defer func() {
		os.Setenv("KIMI_API_KEY", oldKimi)
		os.Setenv("ANTHROPIC_API_KEY", oldClaude)
		os.Setenv("OPENAI_API_KEY", oldOpenAI)
	}()

	os.Setenv("KIMI_API_KEY", "")
	os.Setenv("ANTHROPIC_API_KEY", "")
	os.Setenv("OPENAI_API_KEY", "")

	provider := DetectProvider()
	if provider != "ollama" {
		t.Errorf("expected ollama fallback, got %s", provider)
	}
}

func TestDetectProviderPriority(t *testing.T) {
	oldKimi := os.Getenv("KIMI_API_KEY")
	oldClaude := os.Getenv("ANTHROPIC_API_KEY")
	oldOpenAI := os.Getenv("OPENAI_API_KEY")
	defer func() {
		os.Setenv("KIMI_API_KEY", oldKimi)
		os.Setenv("ANTHROPIC_API_KEY", oldClaude)
		os.Setenv("OPENAI_API_KEY", oldOpenAI)
	}()

	// all set - should prefer kimi
	os.Setenv("KIMI_API_KEY", "test")
	os.Setenv("ANTHROPIC_API_KEY", "test")
	os.Setenv("OPENAI_API_KEY", "test")

	provider := DetectProvider()
	if provider != "kimi" {
		t.Errorf("expected kimi (priority), got %s", provider)
	}
}

func TestDefaultCoderModel(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"kimi", "kimi-k2.5"},
		{"claude", "claude-sonnet-4-20250514"},
		{"openai", "gpt-4o"},
		{"ollama", "qwen2.5-coder:7b"},
		{"unknown", "qwen2.5-coder:7b"},
	}

	for _, tt := range tests {
		got := DefaultCoderModel(tt.provider)
		if got != tt.want {
			t.Errorf("DefaultCoderModel(%s) = %s, want %s", tt.provider, got, tt.want)
		}
	}
}

func TestEnvKeyForProvider(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"kimi", "KIMI_API_KEY"},
		{"claude", "ANTHROPIC_API_KEY"},
		{"openai", "OPENAI_API_KEY"},
		{"ollama", ""},
		{"unknown", "UNKNOWN_API_KEY"}, // unknown providers get uppercased + _API_KEY
	}

	for _, tt := range tests {
		got := EnvKeyForProvider(tt.provider)
		if got != tt.want {
			t.Errorf("EnvKeyForProvider(%s) = %s, want %s", tt.provider, got, tt.want)
		}
	}
}
