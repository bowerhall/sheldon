package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bowerhall/sheldon/internal/config"
	"github.com/bowerhall/sheldon/internal/llm"
)

type RemoteClient struct {
	runtimeConfig *config.RuntimeConfig
	client        *http.Client
}

func NewRemoteClient(rc *config.RuntimeConfig) *RemoteClient {
	return &RemoteClient{
		runtimeConfig: rc,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (h *RemoteClient) agentURL() string {
	ollamaHost := h.runtimeConfig.Get("ollama_host")
	// homelab-agent runs on port 8080, ollama on 11434
	// if ollama_host is http://gpu-monster:11434, agent is http://gpu-monster:8080
	if strings.Contains(ollamaHost, ":11434") {
		return strings.Replace(ollamaHost, ":11434", ":8080", 1)
	}
	// default to localhost
	return "http://localhost:8080"
}

func (h *RemoteClient) isLocalhost() bool {
	url := h.agentURL()
	return strings.Contains(url, "localhost") || strings.Contains(url, "127.0.0.1")
}

func RegisterRemoteTools(registry *Registry, rc *config.RuntimeConfig) {
	client := NewRemoteClient(rc)

	registerRemoteStatus(registry, client)
	registerListContainers(registry, client)
	registerContainerStatus(registry, client)
	registerContainerRestart(registry, client)
	registerContainerStop(registry, client)
	registerContainerStart(registry, client)
	registerContainerLogs(registry, client)
}

func registerRemoteStatus(registry *Registry, client *RemoteClient) {
	tool := llm.Tool{
		Name:        "remote_status",
		Description: "Get system status of the current Ollama host machine (CPU, memory, disk usage). Works on remote machines connected via Tailscale.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}

	registry.Register(tool, func(ctx context.Context, args string) (string, error) {
		if client.isLocalhost() {
			return "remote_status only works on remote machines. Current ollama_host is localhost.", nil
		}

		url := client.agentURL() + "/status"
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return "", fmt.Errorf("create request: %w", err)
		}

		resp, err := client.client.Do(req)
		if err != nil {
			return "", fmt.Errorf("remote host unreachable: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("remote host returned %d: %s", resp.StatusCode, string(body))
		}

		var status struct {
			Hostname string  `json:"hostname"`
			OS       string  `json:"os"`
			Arch     string  `json:"arch"`
			CPUUsage float64 `json:"cpu_usage_percent"`
			MemTotal uint64  `json:"mem_total_bytes"`
			MemUsed  uint64  `json:"mem_used_bytes"`
			MemUsage float64 `json:"mem_usage_percent"`
			DiskUsed uint64  `json:"disk_used_bytes"`
			DiskFree uint64  `json:"disk_free_bytes"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			return "", fmt.Errorf("decode response: %w", err)
		}

		return fmt.Sprintf(`remote host status: %s

system:
  hostname: %s
  os: %s/%s

resources:
  cpu: %.1f%% used
  memory: %.1f%% used (%.1f GB / %.1f GB)
  disk: %.1f GB used, %.1f GB free`,
			status.Hostname,
			status.Hostname, status.OS, status.Arch,
			status.CPUUsage,
			status.MemUsage, float64(status.MemUsed)/1e9, float64(status.MemTotal)/1e9,
			float64(status.DiskUsed)/1e9, float64(status.DiskFree)/1e9,
		), nil
	})
}

func registerListContainers(registry *Registry, client *RemoteClient) {
	tool := llm.Tool{
		Name:        "list_containers",
		Description: "List all Docker containers on the remote host. Shows name, image, status, and whether running.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}

	registry.Register(tool, func(ctx context.Context, args string) (string, error) {
		if client.isLocalhost() {
			return "list_containers only works on remote machines. Current ollama_host is localhost.", nil
		}

		url := client.agentURL() + "/containers"
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return "", fmt.Errorf("create request: %w", err)
		}

		resp, err := client.client.Do(req)
		if err != nil {
			return "", fmt.Errorf("remote host unreachable: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("remote host returned %d: %s", resp.StatusCode, string(body))
		}

		var containers []struct {
			Name    string `json:"name"`
			Image   string `json:"image"`
			Status  string `json:"status"`
			Running bool   `json:"running"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
			return "", fmt.Errorf("decode response: %w", err)
		}

		if len(containers) == 0 {
			return "no containers found on remote host", nil
		}

		var sb strings.Builder
		sb.WriteString("containers on remote host:\n\n")
		for _, c := range containers {
			state := "stopped"
			if c.Running {
				state = "running"
			}
			sb.WriteString(fmt.Sprintf("  %s [%s]\n    image: %s\n    status: %s\n\n",
				c.Name, state, c.Image, c.Status))
		}

		return sb.String(), nil
	})
}

func registerContainerStatus(registry *Registry, client *RemoteClient) {
	tool := llm.Tool{
		Name:        "container_status",
		Description: "Get status of a specific container on the remote host.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Container name (e.g., 'ollama', 'sheldon', 'minio')",
				},
			},
			"required": []string{"name"},
		},
	}

	registry.Register(tool, func(ctx context.Context, args string) (string, error) {
		if client.isLocalhost() {
			return "container_status only works on remote machines.", nil
		}

		var params struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		url := client.agentURL() + "/containers/" + params.Name
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return "", fmt.Errorf("create request: %w", err)
		}

		resp, err := client.client.Do(req)
		if err != nil {
			return "", fmt.Errorf("remote host unreachable: %w", err)
		}
		defer resp.Body.Close()

		var container struct {
			Name    string `json:"name"`
			ID      string `json:"id"`
			Image   string `json:"image"`
			Status  string `json:"status"`
			Running bool   `json:"running"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&container); err != nil {
			return "", fmt.Errorf("decode response: %w", err)
		}

		state := "stopped"
		if container.Running {
			state = "running"
		}

		return fmt.Sprintf("container %s: %s\n  id: %s\n  image: %s\n  status: %s",
			container.Name, state, container.ID, container.Image, container.Status), nil
	})
}

func registerContainerRestart(registry *Registry, client *RemoteClient) {
	tool := llm.Tool{
		Name:        "restart_container",
		Description: "Restart a Docker container on the remote host. Use this to restart ollama or other services.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Container name to restart (e.g., 'ollama')",
				},
			},
			"required": []string{"name"},
		},
	}

	registry.Register(tool, func(ctx context.Context, args string) (string, error) {
		if client.isLocalhost() {
			return "restart_container only works on remote machines. Ask the user to restart it manually.", nil
		}

		var params struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		registry.Notify(ctx, fmt.Sprintf("restarting %s on remote host...", params.Name))

		url := client.agentURL() + "/containers/" + params.Name + "/restart"
		req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
		if err != nil {
			return "", fmt.Errorf("create request: %w", err)
		}

		resp, err := client.client.Do(req)
		if err != nil {
			return "", fmt.Errorf("remote host unreachable: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("restart failed: %s", string(body))
		}

		registry.Notify(ctx, fmt.Sprintf("%s restarted successfully", params.Name))
		return fmt.Sprintf("container %q restarted successfully", params.Name), nil
	})
}

func registerContainerStop(registry *Registry, client *RemoteClient) {
	tool := llm.Tool{
		Name:        "stop_container",
		Description: "Stop a Docker container on the remote host.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Container name to stop",
				},
			},
			"required": []string{"name"},
		},
	}

	registry.Register(tool, func(ctx context.Context, args string) (string, error) {
		if client.isLocalhost() {
			return "stop_container only works on remote machines.", nil
		}

		var params struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		registry.Notify(ctx, fmt.Sprintf("stopping %s on remote host...", params.Name))

		url := client.agentURL() + "/containers/" + params.Name + "/stop"
		req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
		if err != nil {
			return "", fmt.Errorf("create request: %w", err)
		}

		resp, err := client.client.Do(req)
		if err != nil {
			return "", fmt.Errorf("remote host unreachable: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("stop failed: %s", string(body))
		}

		return fmt.Sprintf("container %q stopped", params.Name), nil
	})
}

func registerContainerStart(registry *Registry, client *RemoteClient) {
	tool := llm.Tool{
		Name:        "start_container",
		Description: "Start a stopped Docker container on the remote host.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Container name to start",
				},
			},
			"required": []string{"name"},
		},
	}

	registry.Register(tool, func(ctx context.Context, args string) (string, error) {
		if client.isLocalhost() {
			return "start_container only works on remote machines.", nil
		}

		var params struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}

		registry.Notify(ctx, fmt.Sprintf("starting %s on remote host...", params.Name))

		url := client.agentURL() + "/containers/" + params.Name + "/start"
		req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
		if err != nil {
			return "", fmt.Errorf("create request: %w", err)
		}

		resp, err := client.client.Do(req)
		if err != nil {
			return "", fmt.Errorf("remote host unreachable: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("start failed: %s", string(body))
		}

		return fmt.Sprintf("container %q started", params.Name), nil
	})
}

func registerContainerLogs(registry *Registry, client *RemoteClient) {
	tool := llm.Tool{
		Name:        "container_logs",
		Description: "Get recent logs from a Docker container on the remote host. Useful for debugging issues.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Container name (e.g., 'ollama')",
				},
				"lines": map[string]any{
					"type":        "integer",
					"description": "Number of log lines to retrieve (default: 50)",
				},
			},
			"required": []string{"name"},
		},
	}

	registry.Register(tool, func(ctx context.Context, args string) (string, error) {
		if client.isLocalhost() {
			return "container_logs only works on remote machines.", nil
		}

		var params struct {
			Name  string `json:"name"`
			Lines int    `json:"lines"`
		}
		if err := json.Unmarshal([]byte(args), &params); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		if params.Lines == 0 {
			params.Lines = 50
		}

		url := fmt.Sprintf("%s/containers/%s/logs?lines=%d", client.agentURL(), params.Name, params.Lines)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return "", fmt.Errorf("create request: %w", err)
		}

		resp, err := client.client.Do(req)
		if err != nil {
			return "", fmt.Errorf("remote host unreachable: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("logs failed: %s", string(body))
		}

		logs := string(body)
		if len(logs) > 4000 {
			logs = logs[len(logs)-4000:]
			logs = "... (truncated)\n" + logs
		}

		return fmt.Sprintf("%s logs (last %d lines):\n\n%s", params.Name, params.Lines, logs), nil
	})
}
