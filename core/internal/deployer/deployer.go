package deployer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bowerhall/sheldon/internal/logger"
)

type Deployer struct {
	namespace string
}

func NewDeployer(namespace string) *Deployer {
	if namespace == "" {
		namespace = "sheldon-apps"
	}
	return &Deployer{namespace: namespace}
}

func (d *Deployer) Namespace() string {
	return d.namespace
}

func (d *Deployer) Deploy(ctx context.Context, manifestDir string) (*DeployResult, error) {
	manifests, err := d.findManifests(manifestDir)
	if err != nil {
		return nil, err
	}

	if len(manifests) == 0 {
		return nil, fmt.Errorf("no kubernetes manifests found in %s", manifestDir)
	}

	if err := d.ensureNamespace(ctx); err != nil {
		return nil, err
	}

	var applied []string

	for _, manifest := range manifests {
		if err := d.applyManifest(ctx, manifest); err != nil {
			return &DeployResult{
				Resources: applied,
				Namespace: d.namespace,
				Status:    fmt.Sprintf("failed at %s: %v", filepath.Base(manifest), err),
			}, err
		}
		applied = append(applied, filepath.Base(manifest))
	}

	return &DeployResult{
		Resources: applied,
		Namespace: d.namespace,
		Status:    "deployed",
	}, nil
}

func (d *Deployer) findManifests(dir string) ([]string, error) {
	var manifests []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".yaml" || ext == ".yml" {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			if d.isK8sManifest(string(content)) {
				manifests = append(manifests, path)
			}
		}
		return nil
	})

	return manifests, err
}

func (d *Deployer) isK8sManifest(content string) bool {
	return strings.Contains(content, "apiVersion:") && strings.Contains(content, "kind:")
}

func (d *Deployer) ensureNamespace(ctx context.Context) error {
	checkCmd := exec.CommandContext(ctx, "kubectl", "get", "namespace", d.namespace)
	if err := checkCmd.Run(); err == nil {
		return nil
	}

	createCmd := exec.CommandContext(ctx, "kubectl", "create", "namespace", d.namespace)
	if output, err := createCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create namespace: %w\n%s", err, string(output))
	}

	logger.Info("namespace created", "namespace", d.namespace)
	return nil
}

func (d *Deployer) applyManifest(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "apply", "-n", d.namespace, "-f", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl apply: %w\n%s", err, string(output))
	}

	logger.Debug("manifest applied", "file", filepath.Base(path), "namespace", d.namespace)

	// patch deployments to use local images (imagePullPolicy: Never)
	d.patchImagePullPolicy(ctx)

	return nil
}

func (d *Deployer) patchImagePullPolicy(ctx context.Context) {
	// get all deployments in namespace
	listCmd := exec.CommandContext(ctx, "kubectl", "get", "deployments", "-n", d.namespace, "-o", "name")

	output, err := listCmd.CombinedOutput()
	if err != nil {
		return
	}

	deployments := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, dep := range deployments {
		if dep == "" {
			continue
		}

		// patch each deployment's imagePullPolicy to Never
		patchCmd := exec.CommandContext(ctx, "kubectl", "patch", dep, "-n", d.namespace,
			"--type=json", "-p", `[{"op":"replace","path":"/spec/template/spec/containers/0/imagePullPolicy","value":"Never"}]`)
		patchCmd.Run() // ignore errors - some deployments may not need patching
	}
}

func (d *Deployer) Rollback(ctx context.Context, deploymentName string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "rollout", "undo",
		"-n", d.namespace, "deployment/"+deploymentName)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rollback: %w\n%s", err, string(output))
	}

	return nil
}

func (d *Deployer) Status(ctx context.Context, deploymentName string) (string, error) {
	cmd := exec.CommandContext(ctx, "kubectl", "rollout", "status",
		"-n", d.namespace, "deployment/"+deploymentName, "--timeout=60s")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), err
	}

	return strings.TrimSpace(string(output)), nil
}

func (d *Deployer) deleteNamespace(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "kubectl", "delete", "namespace", d.namespace, "--ignore-not-found")
	_, err := cmd.CombinedOutput()

	return err
}
