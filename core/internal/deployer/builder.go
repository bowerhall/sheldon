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
	tarPath := filepath.Join(b.outputDir, fmt.Sprintf("%s-%s.tar", imageName, imageTag))

	logger.Debug("building image", "context", contextDir, "tag", fullTag)

	var err error
	if b.hasDocker() {
		err = b.buildWithDocker(ctx, contextDir, fullTag, tarPath)
	} else if b.hasKaniko() {
		err = b.buildWithKaniko(ctx, contextDir, fullTag, tarPath)
	} else {
		return nil, fmt.Errorf("no build tool available (need docker or kaniko)")
	}

	if err != nil {
		return nil, err
	}

	if err := b.importToK3s(ctx, tarPath); err != nil {
		os.Remove(tarPath)
		return nil, fmt.Errorf("import to k3s: %w", err)
	}

	stat, _ := os.Stat(tarPath)
	size := int64(0)
	if stat != nil {
		size = stat.Size()
	}

	os.Remove(tarPath)

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

func (b *Builder) hasKaniko() bool {
	_, err := exec.LookPath("kaniko")
	if err != nil {
		_, err = exec.LookPath("/kaniko/executor")
	}

	return err == nil
}

func (b *Builder) buildWithDocker(ctx context.Context, contextDir, tag, tarPath string) error {
	buildCmd := exec.CommandContext(ctx, "docker", "build", "-t", tag, contextDir)
	buildCmd.Dir = contextDir

	if output, err := buildCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker build: %w\n%s", err, string(output))
	}

	saveCmd := exec.CommandContext(ctx, "docker", "save", "-o", tarPath, tag)
	if output, err := saveCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker save: %w\n%s", err, string(output))
	}

	return nil
}

func (b *Builder) buildWithKaniko(ctx context.Context, contextDir, tag, tarPath string) error {
	executor := "kaniko"
	if _, err := exec.LookPath(executor); err != nil {
		executor = "/kaniko/executor"
	}

	cmd := exec.CommandContext(ctx, executor,
		"--context", contextDir,
		"--dockerfile", filepath.Join(contextDir, "Dockerfile"),
		"--destination", tag,
		"--tar-path", tarPath,
		"--no-push",
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("kaniko: %w\n%s", err, string(output))
	}

	return nil
}

func (b *Builder) importToK3s(ctx context.Context, tarPath string) error {
	var cmd *exec.Cmd

	if _, err := exec.LookPath("k3s"); err == nil {
		cmd = exec.CommandContext(ctx, "k3s", "ctr", "images", "import", tarPath)
	} else if _, err := exec.LookPath("ctr"); err == nil {
		cmd = exec.CommandContext(ctx, "ctr", "-n", "k8s.io", "images", "import", tarPath)
	} else {
		logger.Warn("no k3s/ctr found, skipping import (image saved as tar)")
		return nil
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ctr import: %w\n%s", err, string(output))
	}

	logger.Debug("image imported to k3s", "tar", tarPath)
	return nil
}

func (b *Builder) Cleanup(ctx context.Context, keepLatest int) (int, error) {
	if _, err := exec.LookPath("k3s"); err != nil {
		return 0, nil
	}

	cmd := exec.CommandContext(ctx, "k3s", "crictl", "rmi", "--prune")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("crictl rmi: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	return len(lines), nil
}
