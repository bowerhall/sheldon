package coder

import (
	"strings"
	"testing"
)

func TestSanitizeAnthropicKey(t *testing.T) {
	input := "my key is sk-ant-api03-abcdefghijklmnopqrstuvwxyz"
	output, warnings := Sanitize(input)

	if strings.Contains(output, "sk-ant") {
		t.Error("Anthropic key should be redacted")
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Error("should contain [REDACTED]")
	}
	if len(warnings) == 0 {
		t.Error("should have warnings")
	}
}

func TestSanitizeOpenAIKey(t *testing.T) {
	input := "export OPENAI_API_KEY=sk-1234567890123456789012345678901234567890123456789012"
	output, warnings := Sanitize(input)

	if strings.Contains(output, "sk-123") {
		t.Error("OpenAI key should be redacted")
	}
	if len(warnings) == 0 {
		t.Error("should have warnings")
	}
}

func TestSanitizeTelegramToken(t *testing.T) {
	input := "TELEGRAM_TOKEN=bot123456789:ABCdefGHIjklMNOpqrsTUVwxyz123456789"
	output, warnings := Sanitize(input)

	if strings.Contains(output, "bot123456789:") {
		t.Error("Telegram token should be redacted")
	}
	if len(warnings) == 0 {
		t.Error("should have warnings")
	}
}

func TestSanitizeAWSKeys(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"access key", "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE"},
		{"secret key", "aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, warnings := Sanitize(tt.input)
			if len(warnings) == 0 {
				t.Errorf("%s should be detected", tt.name)
			}
			if !strings.Contains(output, "[REDACTED]") {
				t.Errorf("%s should be redacted", tt.name)
			}
		})
	}
}

func TestSanitizeGitHubTokens(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"classic PAT", "ghp_1234567890123456789012345678901234567"},
		{"oauth", "gho_1234567890123456789012345678901234567"},
		{"fine-grained", "github_pat_11ABCDEFG_1234567890abcdefghij"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, warnings := Sanitize(tt.input)
			if len(warnings) == 0 {
				t.Errorf("%s should be detected", tt.name)
			}
			if !strings.Contains(output, "[REDACTED]") {
				t.Errorf("%s should be redacted", tt.name)
			}
		})
	}
}

func TestSanitizePrivateKey(t *testing.T) {
	input := `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA...
-----END RSA PRIVATE KEY-----`
	output, warnings := Sanitize(input)

	if strings.Contains(output, "BEGIN RSA PRIVATE KEY") {
		t.Error("private key header should be redacted")
	}
	if len(warnings) == 0 {
		t.Error("should have warnings for private key")
	}
}

func TestSanitizePasswordPatterns(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"password: mysecret123"},
		{"PASSWORD=hunter2"},
		{"password='secret'"},
		{`password: "mypass"`},
	}

	for _, tt := range tests {
		output, warnings := Sanitize(tt.input)
		if len(warnings) == 0 {
			t.Errorf("should detect password pattern in: %s", tt.input)
		}
		if !strings.Contains(output, "[REDACTED]") {
			t.Errorf("should redact password in: %s", tt.input)
		}
	}
}

func TestSanitizeAPIKeyPatterns(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"api_key: abc123xyz"},
		{"API_KEY=secret"},
		{"api_key='mykey'"},
	}

	for _, tt := range tests {
		output, warnings := Sanitize(tt.input)
		if len(warnings) == 0 {
			t.Errorf("should detect api_key pattern in: %s", tt.input)
		}
		if !strings.Contains(output, "[REDACTED]") {
			t.Errorf("should redact api_key in: %s", tt.input)
		}
	}
}

func TestSanitizeCleanInput(t *testing.T) {
	input := "This is just normal code without any secrets"
	output, warnings := Sanitize(input)

	if output != input {
		t.Errorf("clean input should not be modified: got '%s'", output)
	}
	if len(warnings) > 0 {
		t.Errorf("clean input should have no warnings: %v", warnings)
	}
}

func TestSanitizeFiles(t *testing.T) {
	files := map[string]string{
		"config.go":  "const apiKey = sk-ant-api03-abcdefghijklmnopqrstuvwxyz",
		"main.go":    "fmt.Println(\"hello\")",
		"secrets.go": "password: hunter2",
	}

	sanitized, warnings := SanitizeFiles(files)

	if strings.Contains(sanitized["config.go"], "sk-ant") {
		t.Error("config.go should be sanitized")
	}

	if sanitized["main.go"] != files["main.go"] {
		t.Error("main.go should be unchanged")
	}

	// should have warnings from config.go and secrets.go
	if len(warnings) < 2 {
		t.Errorf("expected at least 2 warnings, got %d", len(warnings))
	}
}

func TestContainsSensitive(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"normal code", false},
		{"my password is secret", true},
		{"API_KEY=abc", true},
		{"the token expires", true},
		{"credential_file", true},
		{"sk-ant-api03-abcdef1234567890", true},
		{"hello world", false},
	}

	for _, tt := range tests {
		got := ContainsSensitive(tt.input)
		if got != tt.expected {
			t.Errorf("ContainsSensitive(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestSanitizeMultipleSecrets(t *testing.T) {
	input := `
config:
  openai_key: sk-1234567890123456789012345678901234567890123456789012
  telegram: bot123456789:ABCdefGHIjklMNOpqrsTUVwxyz123456789
  password: supersecret
`
	output, warnings := Sanitize(input)

	// should have multiple redactions
	redactionCount := strings.Count(output, "[REDACTED]")
	if redactionCount < 3 {
		t.Errorf("expected at least 3 redactions, got %d in output: %s", redactionCount, output)
	}

	if len(warnings) < 3 {
		t.Errorf("expected at least 3 warnings, got %d", len(warnings))
	}
}
