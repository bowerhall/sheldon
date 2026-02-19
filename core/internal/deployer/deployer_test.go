package deployer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuilder_Build(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=1 to run.")
	}

	// Find testdata relative to this test file
	testdataDir := filepath.Join("..", "..", "testdata", "sample-app")
	if _, err := os.Stat(testdataDir); err != nil {
		t.Fatalf("testdata not found: %v", err)
	}

	tmpDir := t.TempDir()
	builder, err := NewBuilder(tmpDir)
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := builder.Build(ctx, testdataDir, "sample-app", "test")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if result.ImageName != "sample-app" {
		t.Errorf("ImageName = %q, want %q", result.ImageName, "sample-app")
	}

	if result.ImageTag != "test" {
		t.Errorf("ImageTag = %q, want %q", result.ImageTag, "test")
	}

	t.Logf("Built image: %s:%s (%d bytes, %s)", result.ImageName, result.ImageTag, result.Size, result.Duration)
}

func TestDeployer_Deploy(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=1 to run.")
	}

	testdataDir := filepath.Join("..", "..", "testdata", "sample-app")
	if _, err := os.Stat(testdataDir); err != nil {
		t.Fatalf("testdata not found: %v", err)
	}

	d := NewDeployer("kora-test")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	result, err := d.Deploy(ctx, testdataDir)
	if err != nil {
		t.Fatalf("Deploy: %v", err)
	}

	if result.Namespace != "kora-test" {
		t.Errorf("Namespace = %q, want %q", result.Namespace, "kora-test")
	}

	if len(result.Resources) == 0 {
		t.Error("Expected at least one resource to be applied")
	}

	t.Logf("Deployed to %s: %v (status: %s)", result.Namespace, result.Resources, result.Status)

	// Cleanup
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Delete the test namespace
		cleanup := NewDeployer("kora-test")
		_ = cleanup.deleteNamespace(cleanupCtx)
	})
}

func TestDeployer_Status(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=1 to run.")
	}

	d := NewDeployer("kora-test")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	status, err := d.Status(ctx, "sample-app")
	if err != nil {
		t.Logf("Status check (may fail if not deployed): %v", err)
		return
	}

	t.Logf("Deployment status: %s", status)
}

func TestBuilder_HasDockerfile(t *testing.T) {
	tmpDir := t.TempDir()
	builder, _ := NewBuilder(tmpDir)

	// No Dockerfile
	if builder.hasDockerfile(tmpDir) {
		t.Error("Expected hasDockerfile to return false for empty dir")
	}

	// Create Dockerfile
	if err := os.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte("FROM alpine"), 0644); err != nil {
		t.Fatal(err)
	}

	if !builder.hasDockerfile(tmpDir) {
		t.Error("Expected hasDockerfile to return true")
	}
}

func TestDeployer_IsK8sManifest(t *testing.T) {
	d := NewDeployer("")

	tests := []struct {
		content string
		want    bool
	}{
		{"apiVersion: v1\nkind: Pod", true},
		{"apiVersion: apps/v1\nkind: Deployment", true},
		{"name: test\nvalue: 123", false},
		{"random yaml content", false},
	}

	for _, tt := range tests {
		got := d.isK8sManifest(tt.content)
		if got != tt.want {
			t.Errorf("isK8sManifest(%q) = %v, want %v", tt.content[:20], got, tt.want)
		}
	}
}
