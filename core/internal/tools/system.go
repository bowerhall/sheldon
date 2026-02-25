package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/bowerhall/sheldon/internal/llm"
)

func RegisterSystemTools(registry *Registry, memoryPath string) {
	registerSystemStatus(registry, memoryPath)
}

func registerSystemStatus(registry *Registry, memoryPath string) {
	tool := llm.Tool{
		Name: "system_status",
		Description: `Check system disk space and memory database size. Use this before pulling large models or when you need to know storage capacity. Returns:
- Available disk space
- Memory database size (facts + embeddings)
- Total/used disk space`,
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}

	registry.Register(tool, func(ctx context.Context, args string) (string, error) {
		var sb strings.Builder

		// Get disk space for the directory containing memory DB
		dir := filepath.Dir(memoryPath)
		if dir == "" || dir == "." {
			dir = "/"
		}

		var stat syscall.Statfs_t
		if err := syscall.Statfs(dir, &stat); err == nil {
			total := stat.Blocks * uint64(stat.Bsize)
			free := stat.Bavail * uint64(stat.Bsize)
			used := total - free

			sb.WriteString("Disk Space:\n")
			sb.WriteString(fmt.Sprintf("  Total: %s\n", formatBytes(total)))
			sb.WriteString(fmt.Sprintf("  Used: %s (%.1f%%)\n", formatBytes(used), float64(used)/float64(total)*100))
			sb.WriteString(fmt.Sprintf("  Available: %s\n", formatBytes(free)))
			sb.WriteString("\n")
		}

		// Get memory database size
		if info, err := os.Stat(memoryPath); err == nil {
			sb.WriteString("Memory Database:\n")
			sb.WriteString(fmt.Sprintf("  Size: %s\n", formatBytes(uint64(info.Size()))))

			// Check for WAL file
			walPath := memoryPath + "-wal"
			if walInfo, err := os.Stat(walPath); err == nil {
				sb.WriteString(fmt.Sprintf("  WAL: %s\n", formatBytes(uint64(walInfo.Size()))))
			}
		}

		// Check conversation buffer (same DB or separate)
		convoPath := filepath.Join(dir, "conversation.db")
		if info, err := os.Stat(convoPath); err == nil {
			sb.WriteString(fmt.Sprintf("  Conversation buffer: %s\n", formatBytes(uint64(info.Size()))))
		}

		return sb.String(), nil
	})
}

func formatBytes(bytes uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}
