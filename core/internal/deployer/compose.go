package deployer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bowerhall/sheldon/internal/logger"
	"gopkg.in/yaml.v3"
)

// ComposeDeployer deploys apps using docker compose
type ComposeDeployer struct {
	appsFile     string // container path to apps.yml (for file I/O)
	hostAppsFile string // host path for docker compose commands
	pathPrefix   string // container path prefix (e.g., /data)
	hostPrefix   string // host path prefix (e.g., /opt/sheldon/data)
	network      string // docker network name
}

// ComposeService represents a service in docker compose
type ComposeService struct {
	Image       string            `yaml:"image,omitempty"`
	Build       string            `yaml:"build,omitempty"`
	Restart     string            `yaml:"restart,omitempty"`
	Ports       []string          `yaml:"ports,omitempty"`
	Volumes     []string          `yaml:"volumes,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty"`
	Labels      []string          `yaml:"labels,omitempty"`
	Networks    []string          `yaml:"networks,omitempty"`
	DependsOn   []string          `yaml:"depends_on,omitempty"`
}

// ComposeFile represents a docker-compose.yml structure
type ComposeFile struct {
	Services map[string]ComposeService `yaml:"services"`
	Networks map[string]ComposeNetwork `yaml:"networks,omitempty"`
}

// ComposeNetwork represents a docker network
type ComposeNetwork struct {
	External bool `yaml:"external,omitempty"`
}

// ComposeDeployerConfig holds configuration for ComposeDeployer
type ComposeDeployerConfig struct {
	AppsFile     string // container path for apps.yml
	HostAppsFile string // host path for docker compose -f
	PathPrefix   string // container path prefix (e.g., /data)
	HostPrefix   string // host path prefix (e.g., /opt/sheldon/data)
	Network      string // docker network name
}

// NewComposeDeployer creates a new compose deployer
func NewComposeDeployer(cfg ComposeDeployerConfig) *ComposeDeployer {
	if cfg.AppsFile == "" {
		cfg.AppsFile = "/data/apps.yml"
	}
	if cfg.HostAppsFile == "" {
		cfg.HostAppsFile = cfg.AppsFile // fallback if not in container
	}
	if cfg.Network == "" {
		cfg.Network = "sheldon-net"
	}
	return &ComposeDeployer{
		appsFile:     cfg.AppsFile,
		hostAppsFile: cfg.HostAppsFile,
		pathPrefix:   cfg.PathPrefix,
		hostPrefix:   cfg.HostPrefix,
		network:      cfg.Network,
	}
}

// toHostPath converts a container path to host path
func (d *ComposeDeployer) toHostPath(containerPath string) string {
	if d.pathPrefix == "" || d.hostPrefix == "" {
		return containerPath
	}
	if strings.HasPrefix(containerPath, d.pathPrefix) {
		return strings.Replace(containerPath, d.pathPrefix, d.hostPrefix, 1)
	}
	return containerPath
}

// Deploy adds a service to apps.yml and runs docker compose up
func (d *ComposeDeployer) Deploy(ctx context.Context, appDir string, name string, domain string) (*DeployResult, error) {
	// load or create compose file
	compose, err := d.loadComposeFile()
	if err != nil {
		return nil, fmt.Errorf("load compose file: %w", err)
	}

	// determine if we're deploying from a build or an image
	service := ComposeService{
		Restart:  "unless-stopped",
		Networks: []string{d.network},
	}

	// check if there's a Dockerfile
	if _, err := os.Stat(filepath.Join(appDir, "Dockerfile")); err == nil {
		// use host path for docker compose build context
		service.Build = d.toHostPath(appDir)
	} else {
		// no Dockerfile, assume image name matches app name
		service.Image = name + ":latest"
	}

	// add traefik labels for routing
	if domain != "" {
		service.Labels = []string{
			"traefik.enable=true",
			fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s.%s`)", name, name, domain),
			fmt.Sprintf("traefik.http.routers.%s.entrypoints=web", name),
		}
	}

	// add service to compose
	if compose.Services == nil {
		compose.Services = make(map[string]ComposeService)
	}
	compose.Services[name] = service

	// ensure network is defined
	if compose.Networks == nil {
		compose.Networks = make(map[string]ComposeNetwork)
	}
	compose.Networks[d.network] = ComposeNetwork{External: true}

	// save compose file
	if err := d.saveComposeFile(compose); err != nil {
		return nil, fmt.Errorf("save compose file: %w", err)
	}

	// run docker compose up
	if err := d.composeUp(ctx, name); err != nil {
		return &DeployResult{
			Resources: []string{name},
			Status:    fmt.Sprintf("failed: %v", err),
		}, err
	}

	logger.Info("app deployed via compose", "name", name, "file", d.appsFile)

	return &DeployResult{
		Resources: []string{name},
		Status:    "deployed",
	}, nil
}

// Remove stops and removes a service from apps.yml
func (d *ComposeDeployer) Remove(ctx context.Context, name string) error {
	// load compose file
	compose, err := d.loadComposeFile()
	if err != nil {
		return fmt.Errorf("load compose file: %w", err)
	}

	// check if service exists
	if _, exists := compose.Services[name]; !exists {
		return fmt.Errorf("service %s not found", name)
	}

	// stop the service first
	if err := d.composeDown(ctx, name); err != nil {
		logger.Warn("failed to stop service", "name", name, "error", err)
	}

	// remove from compose
	delete(compose.Services, name)

	// save compose file
	if err := d.saveComposeFile(compose); err != nil {
		return fmt.Errorf("save compose file: %w", err)
	}

	logger.Info("app removed from compose", "name", name)
	return nil
}

// List returns all deployed services
func (d *ComposeDeployer) List(ctx context.Context) ([]string, error) {
	compose, err := d.loadComposeFile()
	if err != nil {
		return nil, err
	}

	var services []string
	for name := range compose.Services {
		services = append(services, name)
	}
	return services, nil
}

// Status returns the status of a service
func (d *ComposeDeployer) Status(ctx context.Context, name string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", d.hostAppsFile, "ps", name, "--format", "{{.Status}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("get status: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// Logs returns recent logs for a service
func (d *ComposeDeployer) Logs(ctx context.Context, name string, lines int) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", d.hostAppsFile, "logs", "--tail", fmt.Sprintf("%d", lines), name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("get logs: %w", err)
	}
	return string(output), nil
}

func (d *ComposeDeployer) loadComposeFile() (*ComposeFile, error) {
	compose := &ComposeFile{
		Services: make(map[string]ComposeService),
		Networks: make(map[string]ComposeNetwork),
	}

	data, err := os.ReadFile(d.appsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return compose, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, compose); err != nil {
		return nil, fmt.Errorf("parse compose file: %w", err)
	}

	if compose.Services == nil {
		compose.Services = make(map[string]ComposeService)
	}
	if compose.Networks == nil {
		compose.Networks = make(map[string]ComposeNetwork)
	}

	return compose, nil
}

func (d *ComposeDeployer) saveComposeFile(compose *ComposeFile) error {
	// ensure directory exists
	dir := filepath.Dir(d.appsFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(compose)
	if err != nil {
		return err
	}

	// add header comment
	header := "# Sheldon-Managed Apps\n# This file is managed by Sheldon. Do not edit manually.\n\n"
	return os.WriteFile(d.appsFile, []byte(header+string(data)), 0644)
}

func (d *ComposeDeployer) composeUp(ctx context.Context, service string) error {
	// use host path for docker compose commands
	composeFile := d.hostAppsFile

	// build if needed
	buildCmd := exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "build", service)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	// ignore build errors - might not have a Dockerfile

	buildCmd.Run()

	// start service
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "up", "-d", service)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compose up: %w\n%s", err, string(output))
	}
	return nil
}

func (d *ComposeDeployer) composeDown(ctx context.Context, service string) error {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", d.hostAppsFile, "rm", "-f", "-s", service)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compose down: %w\n%s", err, string(output))
	}
	return nil
}
