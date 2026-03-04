package telemetry

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/bowerhall/sheldon/internal/logger"
)

const heartbeatURL = "https://bowerhall.ai/api/heartbeat"

// Heartbeat sends anonymous install telemetry
// Disabled by setting TELEMETRY_DISABLED=true
func Heartbeat(version, dbPath string) {
	if os.Getenv("TELEMETRY_DISABLED") == "true" {
		return
	}

	go func() {
		// Small delay to not slow down startup
		time.Sleep(5 * time.Second)

		id := generateInstallID(dbPath)
		payload := map[string]string{
			"id": id,
			"v":  version,
			"os": runtime.GOOS,
		}

		body, _ := json.Marshal(payload)

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Post(heartbeatURL, "application/json", bytes.NewReader(body))
		if err != nil {
			logger.Debug("telemetry heartbeat failed", "error", err)
			return
		}
		defer resp.Body.Close()

		logger.Debug("telemetry heartbeat sent", "id", id[:8]+"...")
	}()
}

// generateInstallID creates a stable anonymous identifier
// Based on db path + hostname - unique per install, not personally identifiable
func generateInstallID(dbPath string) string {
	hostname, _ := os.Hostname()
	data := dbPath + ":" + hostname + ":sheldon"

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
