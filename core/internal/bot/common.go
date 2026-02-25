package bot

import (
	"strings"
	"sync"
)

// sessionMu protects active session maps across all bot implementations.
// Each bot maintains its own activeSessions map, but they share this mutex
// to ensure thread-safe access when processing messages concurrently.
var sessionMu sync.Mutex

// stopWords are natural language commands that cancel the current operation.
// Keep this list minimal to avoid false positives.
var stopWords = []string{"stop", "cancel", "abort", "nevermind", "never mind", "quit", "halt"}

// isStopCommand checks if the message is a request to stop the current operation.
// This is a shared utility for all bot implementations (Telegram, Discord, future voice).
func isStopCommand(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	for _, word := range stopWords {
		if lower == word {
			return true
		}
	}
	return false
}

// maxMediaSize is the maximum size for media attachments (20MB).
const maxMediaSize = 20 * 1024 * 1024
