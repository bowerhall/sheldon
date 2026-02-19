package coder

import (
	"regexp"
	"strings"
)

var patterns = []*regexp.Regexp{
	regexp.MustCompile(`sk-ant-[a-zA-Z0-9\-_]{20,}`),         // anthropic
	regexp.MustCompile(`sk-[a-zA-Z0-9]{48,}`),                // openai
	regexp.MustCompile(`bot\d+:[a-zA-Z0-9_-]{35}`),           // telegram
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),                   // aws access key
	regexp.MustCompile(`(?i)aws_secret_access_key\s*=\s*\S+`),
	regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`),                // github pat
	regexp.MustCompile(`gho_[a-zA-Z0-9]{36}`),                // github oauth
	regexp.MustCompile(`github_pat_[a-zA-Z0-9_]{22,}`),       // github fine-grained
	regexp.MustCompile(`voyage-[a-zA-Z0-9]{20,}`),            // voyage
	regexp.MustCompile(`(?i)password\s*[:=]\s*["']?[^\s"']+`),
	regexp.MustCompile(`(?i)api_key\s*[:=]\s*["']?[^\s"']+`),
	regexp.MustCompile(`(?i)secret\s*[:=]\s*["']?[^\s"']+`),
	regexp.MustCompile(`-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----`),
}

func Sanitize(input string) (string, []string) {
	var warnings []string
	output := input

	for _, pat := range patterns {
		if pat.MatchString(output) {
			warnings = append(warnings, "redacted potential credential")
			output = pat.ReplaceAllString(output, "[REDACTED]")
		}
	}

	return output, warnings
}

func SanitizeFiles(files map[string]string) (map[string]string, []string) {
	var allWarnings []string
	result := make(map[string]string, len(files))

	for path, content := range files {
		sanitized, warnings := Sanitize(content)
		result[path] = sanitized
		for _, w := range warnings {
			allWarnings = append(allWarnings, path+": "+w)
		}
	}

	return result, allWarnings
}

func ContainsSensitive(input string) bool {
	lower := strings.ToLower(input)
	sensitiveTerms := []string{
		"password", "secret", "api_key", "apikey", "token",
		"credential", "private_key", "access_key",
	}

	for _, term := range sensitiveTerms {
		if strings.Contains(lower, term) {
			return true
		}
	}

	for _, pat := range patterns {
		if pat.MatchString(input) {
			return true
		}
	}

	return false
}
