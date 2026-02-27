package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
)

var validContainerName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	agent := &Agent{
		logger: logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", agent.handleHealth)
	mux.HandleFunc("GET /status", agent.handleStatus)

	// generic container management
	mux.HandleFunc("GET /containers", agent.handleListContainers)
	mux.HandleFunc("GET /containers/{name}", agent.handleContainerStatus)
	mux.HandleFunc("POST /containers/{name}/restart", agent.handleContainerRestart)
	mux.HandleFunc("POST /containers/{name}/stop", agent.handleContainerStop)
	mux.HandleFunc("POST /containers/{name}/start", agent.handleContainerStart)
	mux.HandleFunc("GET /containers/{name}/logs", agent.handleContainerLogs)

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		logger.Info("homelab-agent starting", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Info("shutting down")
	server.Shutdown(ctx)
}

type Agent struct {
	logger *slog.Logger
}

type StatusResponse struct {
	Hostname string  `json:"hostname"`
	OS       string  `json:"os"`
	Arch     string  `json:"arch"`
	CPUUsage float64 `json:"cpu_usage_percent"`
	MemTotal uint64  `json:"mem_total_bytes"`
	MemUsed  uint64  `json:"mem_used_bytes"`
	MemUsage float64 `json:"mem_usage_percent"`
	DiskPath string  `json:"disk_path"`
	DiskUsed uint64  `json:"disk_used_bytes"`
	DiskFree uint64  `json:"disk_free_bytes"`
}

func (a *Agent) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func (a *Agent) handleStatus(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()

	cpuPercent, _ := cpu.Percent(time.Second, false)
	cpuUsage := 0.0
	if len(cpuPercent) > 0 {
		cpuUsage = cpuPercent[0]
	}

	memInfo, _ := mem.VirtualMemory()
	diskInfo, _ := disk.Usage("/")

	status := StatusResponse{
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		CPUUsage: cpuUsage,
		MemTotal: memInfo.Total,
		MemUsed:  memInfo.Used,
		MemUsage: memInfo.UsedPercent,
		DiskPath: "/",
		DiskUsed: diskInfo.Used,
		DiskFree: diskInfo.Free,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

type ContainerInfo struct {
	Name    string `json:"name"`
	ID      string `json:"id"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	Running bool   `json:"running"`
}

func (a *Agent) handleListContainers(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("docker", "ps", "-a", "--format", "{{.Names}}\t{{.ID}}\t{{.Image}}\t{{.Status}}\t{{.State}}")
	output, err := cmd.Output()
	if err != nil {
		http.Error(w, fmt.Sprintf("docker ps failed: %v", err), http.StatusInternalServerError)
		return
	}

	var containers []ContainerInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) >= 5 {
			containers = append(containers, ContainerInfo{
				Name:    parts[0],
				ID:      parts[1],
				Image:   parts[2],
				Status:  parts[3],
				Running: parts[4] == "running",
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(containers)
}

func validateContainerName(name string) error {
	if name == "" {
		return fmt.Errorf("container name required")
	}
	if !validContainerName.MatchString(name) {
		return fmt.Errorf("invalid container name")
	}
	return nil
}

func (a *Agent) handleContainerStatus(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := validateContainerName(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cmd := exec.Command("docker", "inspect", "-f",
		"{{.Name}}\t{{.Id}}\t{{.Config.Image}}\t{{.State.Status}}\t{{.State.Running}}", name)
	output, err := cmd.Output()
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ContainerInfo{
			Name:    name,
			Status:  "not found",
			Running: false,
		})
		return
	}

	parts := strings.Split(strings.TrimSpace(string(output)), "\t")
	if len(parts) >= 5 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ContainerInfo{
			Name:    strings.TrimPrefix(parts[0], "/"),
			ID:      parts[1][:12],
			Image:   parts[2],
			Status:  parts[3],
			Running: parts[4] == "true",
		})
		return
	}

	http.Error(w, "failed to parse container info", http.StatusInternalServerError)
}

func (a *Agent) handleContainerRestart(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := validateContainerName(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.logger.Info("restarting container", "container", name)

	cmd := exec.Command("docker", "restart", name)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		a.logger.Error("failed to restart container", "container", name, "error", err)
		http.Error(w, fmt.Sprintf("restart failed: %s", stderr.String()), http.StatusInternalServerError)
		return
	}

	a.logger.Info("container restarted", "container", name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "restarted",
		"container": name,
	})
}

func (a *Agent) handleContainerStop(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := validateContainerName(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.logger.Info("stopping container", "container", name)

	cmd := exec.Command("docker", "stop", name)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		a.logger.Error("failed to stop container", "container", name, "error", err)
		http.Error(w, fmt.Sprintf("stop failed: %s", stderr.String()), http.StatusInternalServerError)
		return
	}

	a.logger.Info("container stopped", "container", name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "stopped",
		"container": name,
	})
}

func (a *Agent) handleContainerStart(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := validateContainerName(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	a.logger.Info("starting container", "container", name)

	cmd := exec.Command("docker", "start", name)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		a.logger.Error("failed to start container", "container", name, "error", err)
		http.Error(w, fmt.Sprintf("start failed: %s", stderr.String()), http.StatusInternalServerError)
		return
	}

	a.logger.Info("container started", "container", name)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":    "started",
		"container": name,
	})
}

func (a *Agent) handleContainerLogs(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := validateContainerName(name); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	lines := r.URL.Query().Get("lines")
	if lines == "" {
		lines = "100"
	}
	if n, err := strconv.Atoi(lines); err != nil || n < 1 || n > 10000 {
		lines = "100"
	}

	cmd := exec.Command("docker", "logs", "--tail", lines, name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		http.Error(w, fmt.Sprintf("logs failed: %s", string(output)), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write(output)
}
