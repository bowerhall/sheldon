package tools

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bowerhall/sheldon/internal/llm"
	"github.com/bowerhall/sheldon/internal/storage"
)

// shared HTTP client for URL fetching (reuses connections)
var fetchHTTPClient = &http.Client{Timeout: 5 * time.Minute}

// RegisterStorageTools registers MinIO file storage tools
func RegisterStorageTools(registry *Registry, client *storage.Client) {
	// upload file tool
	uploadTool := llm.Tool{
		Name:        "upload_file",
		Description: "Upload a file to storage. Use 'user' space for user files, 'agent' space for agent's own files (notes, exports, artifacts).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"space": map[string]any{
					"type":        "string",
					"enum":        []string{"user", "agent"},
					"description": "Storage space: 'user' for user files, 'agent' for agent files",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "File path including name (e.g., 'notes/todo.txt', 'exports/data.json')",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "File content as text. For binary files, use base64 encoding and set is_base64=true",
				},
				"is_base64": map[string]any{
					"type":        "boolean",
					"description": "Set to true if content is base64 encoded (for binary files)",
				},
			},
			"required": []string{"space", "path", "content"},
		},
	}

	registry.Register(uploadTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Space    string `json:"space"`
			Path     string `json:"path"`
			Content  string `json:"content"`
			IsBase64 bool   `json:"is_base64"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		bucket := client.UserBucket()
		if params.Space == "agent" {
			bucket = client.AgentBucket()
		}

		var data []byte
		var err error
		if params.IsBase64 {
			data, err = base64.StdEncoding.DecodeString(params.Content)
			if err != nil {
				return "", fmt.Errorf("invalid base64: %w", err)
			}
		} else {
			data = []byte(params.Content)
		}

		contentType := guessContentType(params.Path)
		if err := client.Upload(ctx, bucket, params.Path, data, contentType); err != nil {
			return "", err
		}

		return fmt.Sprintf("uploaded %s to %s (%d bytes)", params.Path, params.Space, len(data)), nil
	})

	// download file tool
	downloadTool := llm.Tool{
		Name:        "download_file",
		Description: "Download a file from storage. Returns file content as text (or base64 for binary files).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"space": map[string]any{
					"type":        "string",
					"enum":        []string{"user", "agent"},
					"description": "Storage space: 'user' for user files, 'agent' for agent files",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "File path to download",
				},
			},
			"required": []string{"space", "path"},
		},
	}

	registry.Register(downloadTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Space string `json:"space"`
			Path  string `json:"path"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		bucket := client.UserBucket()
		if params.Space == "agent" {
			bucket = client.AgentBucket()
		}

		data, err := client.Download(ctx, bucket, params.Path)
		if err != nil {
			return "", err
		}

		// check if binary
		if isBinary(data) {
			return fmt.Sprintf("base64:%s", base64.StdEncoding.EncodeToString(data)), nil
		}

		return string(data), nil
	})

	// list files tool
	listTool := llm.Tool{
		Name:        "list_files",
		Description: "List files in storage. Returns file names, sizes, and modification times.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"space": map[string]any{
					"type":        "string",
					"enum":        []string{"user", "agent"},
					"description": "Storage space: 'user' for user files, 'agent' for agent files",
				},
				"prefix": map[string]any{
					"type":        "string",
					"description": "Optional path prefix to filter files (e.g., 'notes/' to list only notes folder)",
				},
			},
			"required": []string{"space"},
		},
	}

	registry.Register(listTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Space  string `json:"space"`
			Prefix string `json:"prefix"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		bucket := client.UserBucket()
		if params.Space == "agent" {
			bucket = client.AgentBucket()
		}

		files, err := client.List(ctx, bucket, params.Prefix)
		if err != nil {
			return "", err
		}

		if len(files) == 0 {
			return fmt.Sprintf("no files in %s/%s", params.Space, params.Prefix), nil
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("files in %s/%s:\n", params.Space, params.Prefix))
		for _, f := range files {
			if f.IsDir {
				sb.WriteString(fmt.Sprintf("  📁 %s\n", f.Name))
			} else {
				sb.WriteString(fmt.Sprintf("  📄 %s (%d bytes, %s)\n", f.Name, f.Size, f.ModTime))
			}
		}

		return sb.String(), nil
	})

	// delete file tool
	deleteTool := llm.Tool{
		Name:        "delete_file",
		Description: "Delete a file from storage.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"space": map[string]any{
					"type":        "string",
					"enum":        []string{"user", "agent"},
					"description": "Storage space: 'user' for user files, 'agent' for agent files",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "File path to delete",
				},
			},
			"required": []string{"space", "path"},
		},
	}

	registry.Register(deleteTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Space string `json:"space"`
			Path  string `json:"path"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		bucket := client.UserBucket()
		if params.Space == "agent" {
			bucket = client.AgentBucket()
		}

		if err := client.Delete(ctx, bucket, params.Path); err != nil {
			return "", err
		}

		return fmt.Sprintf("deleted %s from %s", params.Path, params.Space), nil
	})

	// share link tool
	shareTool := llm.Tool{
		Name:        "share_link",
		Description: "Generate a temporary shareable link for a file. The link expires after the specified duration.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"space": map[string]any{
					"type":        "string",
					"enum":        []string{"user", "agent"},
					"description": "Storage space: 'user' for user files, 'agent' for agent files",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "File path to share",
				},
				"expires_hours": map[string]any{
					"type":        "integer",
					"description": "Hours until link expires (default: 24, max: 168 = 7 days)",
				},
			},
			"required": []string{"space", "path"},
		},
	}

	registry.Register(shareTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			Space        string `json:"space"`
			Path         string `json:"path"`
			ExpiresHours int    `json:"expires_hours"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		bucket := client.UserBucket()
		if params.Space == "agent" {
			bucket = client.AgentBucket()
		}

		expiry := 24 * time.Hour
		if params.ExpiresHours > 0 {
			if params.ExpiresHours > 168 {
				params.ExpiresHours = 168
			}
			expiry = time.Duration(params.ExpiresHours) * time.Hour
		}

		url, err := client.PresignedURL(ctx, bucket, params.Path, expiry)
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("shareable link (expires in %d hours):\n%s", int(expiry.Hours()), url), nil
	})

	// fetch URL tool
	fetchTool := llm.Tool{
		Name:        "fetch_url",
		Description: "Download a file from a URL and save it to storage. Useful for archiving web content or downloading attachments.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url": map[string]any{
					"type":        "string",
					"description": "URL to download from",
				},
				"space": map[string]any{
					"type":        "string",
					"enum":        []string{"user", "agent"},
					"description": "Storage space: 'user' for user files, 'agent' for agent files",
				},
				"path": map[string]any{
					"type":        "string",
					"description": "Destination path in storage (e.g., 'downloads/file.pdf')",
				},
			},
			"required": []string{"url", "space", "path"},
		},
	}

	registry.Register(fetchTool, func(ctx context.Context, args string) (string, error) {
		var params struct {
			URL   string `json:"url"`
			Space string `json:"space"`
			Path  string `json:"path"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		// SSRF protection: validate URL before fetching
		if err := validateExternalURL(params.URL); err != nil {
			return "", fmt.Errorf("URL blocked: %w", err)
		}

		bucket := client.UserBucket()
		if params.Space == "agent" {
			bucket = client.AgentBucket()
		}

		req, err := http.NewRequestWithContext(ctx, "GET", params.URL, nil)
		if err != nil {
			return "", fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("User-Agent", "Sheldon/1.0")

		resp, err := fetchHTTPClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("download failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
		}

		// limit to 100MB
		const maxSize = 100 * 1024 * 1024
		data, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
		if err != nil {
			return "", fmt.Errorf("read response: %w", err)
		}

		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			contentType = guessContentType(params.Path)
		}

		if err := client.Upload(ctx, bucket, params.Path, data, contentType); err != nil {
			return "", err
		}

		return fmt.Sprintf("downloaded %s to %s/%s (%d bytes)", params.URL, params.Space, params.Path, len(data)), nil
	})
}

// DocumentSender can send documents to users
type DocumentSender interface {
	SendDocument(chatID int64, data []byte, filename, caption string) error
}

// RegisterBackupTool registers the memory backup tool (requires memory path)
func RegisterBackupTool(registry *Registry, client *storage.Client, memoryPath string, sender DocumentSender) {
	tool := llm.Tool{
		Name: "backup_memory",
		Description: `Create a backup of Sheldon's memory database and send it directly to you.

The backup is a zip file containing the SQLite database. It's also stored in the backups bucket for redundancy.

IMPORTANT: Only use this when the user explicitly asks for a backup with words like "backup", "export memory", "download my data". Do NOT use proactively.`,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}

	registry.Register(tool, func(ctx context.Context, args string) (string, error) {
		if err := client.InitBackupBucket(ctx); err != nil {
			return "", fmt.Errorf("init backup bucket: %w", err)
		}

		// read the database file
		dbData, err := os.ReadFile(memoryPath)
		if err != nil {
			return "", fmt.Errorf("read memory db: %w", err)
		}

		// also try to read WAL file if it exists
		walPath := memoryPath + "-wal"
		walData, _ := os.ReadFile(walPath)

		// create zip file containing db and wal
		timestamp := time.Now().Format("2006-01-02_15-04-05")
		zipData, err := createBackupZip(dbData, walData, timestamp)
		if err != nil {
			return "", fmt.Errorf("create zip: %w", err)
		}

		zipName := fmt.Sprintf("sheldon_backup_%s.zip", timestamp)

		// store in backup bucket for redundancy
		if err := client.Upload(ctx, client.BackupBucket(), zipName, zipData, "application/zip"); err != nil {
			// non-fatal, continue to send to user
			fmt.Printf("backup storage failed (non-fatal): %s\n", err.Error())
		}

		// send directly to user - no URL exposed
		chatID := ChatIDFromContext(ctx)
		if chatID == 0 {
			return "", fmt.Errorf("no chat ID in context")
		}

		if err := sender.SendDocument(chatID, zipData, zipName, "Memory backup"); err != nil {
			return "", fmt.Errorf("send backup: %w", err)
		}

		return fmt.Sprintf("Backup sent: %s (%d bytes)", zipName, len(zipData)), nil
	})
}

// createBackupZip creates a zip file containing the database and WAL files
func createBackupZip(dbData, walData []byte, timestamp string) ([]byte, error) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	// add main database file
	dbFile, err := w.Create(fmt.Sprintf("memory_%s.db", timestamp))
	if err != nil {
		return nil, err
	}
	if _, err := dbFile.Write(dbData); err != nil {
		return nil, err
	}

	// add WAL file if present
	if len(walData) > 0 {
		walFile, err := w.Create(fmt.Sprintf("memory_%s.db-wal", timestamp))
		if err != nil {
			return nil, err
		}
		if _, err := walFile.Write(walData); err != nil {
			return nil, err
		}
	}

	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func guessContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt", ".md":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".html":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	default:
		return "application/octet-stream"
	}
}

func isBinary(data []byte) bool {
	// check for null bytes or high proportion of non-printable chars
	for _, b := range data[:min(len(data), 512)] {
		if b == 0 {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// validateExternalURL checks if a URL is safe to fetch (prevents SSRF)
func validateExternalURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// only allow http and https
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("invalid scheme: only http and https allowed")
	}

	host := parsed.Hostname()

	// block localhost variants
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" {
		return fmt.Errorf("localhost access not allowed")
	}

	// resolve hostname to check IP
	ips, err := net.LookupIP(host)
	if err != nil {
		// if we can't resolve, allow it (might be valid external host)
		return nil
	}

	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("private/internal IP access not allowed: %s", ip)
		}
	}

	return nil
}

// isPrivateIP checks if an IP is private, loopback, or cloud metadata
func isPrivateIP(ip net.IP) bool {
	// loopback (127.x.x.x, ::1)
	if ip.IsLoopback() {
		return true
	}

	// link-local (169.254.x.x - includes cloud metadata)
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// private ranges
	if ip.IsPrivate() {
		return true
	}

	// unspecified (0.0.0.0, ::)
	if ip.IsUnspecified() {
		return true
	}

	// explicit check for cloud metadata IP
	if ip.String() == "169.254.169.254" {
		return true
	}

	return false
}
