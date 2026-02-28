package deployer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bowerhall/sheldon/internal/logger"
	"gopkg.in/yaml.v3"
)

var validAppName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

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

// findDockerfile searches for Dockerfile in appDir and immediate subdirectories
// Returns the directory containing Dockerfile, or empty string if not found
func (d *ComposeDeployer) findDockerfile(appDir string) string {
	// check root first
	if _, err := os.Stat(filepath.Join(appDir, "Dockerfile")); err == nil {
		return appDir
	}

	// check immediate subdirectories
	entries, err := os.ReadDir(appDir)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		subdir := filepath.Join(appDir, entry.Name())
		if _, err := os.Stat(filepath.Join(subdir, "Dockerfile")); err == nil {
			return subdir
		}
	}

	return ""
}

// Deploy adds a service to apps.yml and runs docker compose up
func (d *ComposeDeployer) Deploy(ctx context.Context, appDir string, name string, domain string) (*DeployResult, error) {
	// validate app name (alphanumeric + hyphens, max 63 chars, must start with alphanumeric)
	if !validAppName.MatchString(name) {
		return nil, fmt.Errorf("invalid app name %q: must be lowercase alphanumeric with hyphens, 1-63 chars, start with letter/number", name)
	}

	// validate domain if provided
	if domain != "" && domain != "localhost" {
		if !validAppName.MatchString(strings.Split(domain, ".")[0]) {
			return nil, fmt.Errorf("invalid domain %q: contains invalid characters", domain)
		}
	}

	// validate app directory exists
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("app directory does not exist: %s (expected path from write_code workspace)", appDir)
	}

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

	// find Dockerfile - check root first, then immediate subdirectories
	dockerfilePath := d.findDockerfile(appDir)
	if dockerfilePath != "" {
		// use container path - compose CLI runs inside container and reads context from here
		service.Build = dockerfilePath
		logger.Debug("found Dockerfile", "path", dockerfilePath)
	} else {
		return nil, fmt.Errorf("no Dockerfile found in %s or its subdirectories", appDir)
	}

	// add traefik labels for routing with HTTPS
	if domain != "" && domain != "localhost" {
		service.Labels = []string{
			"traefik.enable=true",
			fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s.%s`)", name, name, domain),
			fmt.Sprintf("traefik.http.routers.%s.entrypoints=websecure", name),
			fmt.Sprintf("traefik.http.routers.%s.tls.certresolver=letsencrypt", name),
		}
	} else if domain != "" {
		// localhost: HTTP only
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
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", d.appsFile, "ps", name, "--format", "{{.Status}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("get status: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// Logs returns recent logs for a service
func (d *ComposeDeployer) Logs(ctx context.Context, name string, lines int) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", d.appsFile, "logs", "--tail", fmt.Sprintf("%d", lines), name)
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
	// use container path for -f flag (compose reads the file locally)
	// but the build paths INSIDE the file are host paths (for docker daemon)
	composeFile := d.appsFile

	// build if needed - use legacy builder (not buildx) for docker-proxy compatibility
	buildCmd := exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "build", service)
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	buildCmd.Env = append(os.Environ(), "DOCKER_BUILDKIT=0", "COMPOSE_DOCKER_CLI_BUILD=0")

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
	cmd := exec.CommandContext(ctx, "docker", "compose", "-f", d.appsFile, "rm", "-f", "-s", service)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compose down: %w\n%s", err, string(output))
	}
	return nil
}
