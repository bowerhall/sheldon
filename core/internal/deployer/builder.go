package deployer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bowerhall/sheldon/internal/logger"
)

type Builder struct {
	outputDir string
}

func NewBuilder(outputDir string) (*Builder, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	return &Builder{outputDir: outputDir}, nil
}

func (b *Builder) Build(ctx context.Context, contextDir, imageName, imageTag string) (*BuildResult, error) {
	start := time.Now()

	if !b.hasDockerfile(contextDir) {
		return nil, fmt.Errorf("no Dockerfile found in %s", contextDir)
	}

	fullTag := fmt.Sprintf("%s:%s", imageName, imageTag)

	logger.Debug("building image", "context", contextDir, "tag", fullTag)

	if !b.hasDocker() {
		return nil, fmt.Errorf("docker not available")
	}

	if err := b.buildWithDocker(ctx, contextDir, fullTag); err != nil {
		return nil, err
	}

	// get image size
	size := b.getImageSize(ctx, fullTag)

	return &BuildResult{
		ImageName: imageName,
		ImageTag:  imageTag,
		Size:      size,
		Duration:  time.Since(start).Round(time.Second).String(),
	}, nil
}

func (b *Builder) hasDockerfile(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "Dockerfile"))
	return err == nil
}

func (b *Builder) hasDocker() bool {
	_, err := exec.LookPath("docker")
	return err == nil
}

func (b *Builder) buildWithDocker(ctx context.Context, contextDir, tag string) error {
	buildCmd := exec.CommandContext(ctx, "docker", "build", "-t", tag, contextDir)
	buildCmd.Dir = contextDir

	if output, err := buildCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker build: %w\n%s", err, string(output))
	}

	return nil
}

func (b *Builder) getImageSize(ctx context.Context, tag string) int64 {
	cmd := exec.CommandContext(ctx, "docker", "image", "inspect", tag, "--format", "{{.Size}}")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	var size int64
	fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &size)
	return size
}

func (b *Builder) Cleanup(ctx context.Context, keepLatest int) (int, error) {
	if !b.hasDocker() {
		return 0, nil
	}

	// prune dangling images
	cmd := exec.CommandContext(ctx, "docker", "image", "prune", "-f")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("docker image prune: %w", err)
	}

	// count lines that mention "deleted"
	lines := strings.Split(string(output), "\n")
	count := 0
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "deleted") {
			count++
		}
	}

	return count, nil
}
