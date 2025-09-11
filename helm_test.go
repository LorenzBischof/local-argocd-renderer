package renderer

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestHelmRenderer_SkipCrdsOption(t *testing.T) {
	// Skip if helm is not available
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not found in PATH")
	}

	// Create a temporary directory with a simple chart
	tempDir := t.TempDir()
	createTestHelmChart(t, tempDir)

	// Create ArgoCD Application with skipCrds option
	app := &Application{
		Spec: ApplicationSpec{
			Source: &ApplicationSource{
				Path: "",
				Helm: &ApplicationSourceHelm{
					SkipCrds: true,
				},
			},
		},
	}

	renderCtx := &RenderContext{
		Application: app,
		Source:      app.Spec.Source,
		RepoPath:    tempDir,
		AppName:     "test-app",
		Namespace:   "default",
	}

	// Capture stderr to verify the skipCrds option is applied (verbose output goes to stderr)
	originalStderr := os.Stderr
	defer func() { os.Stderr = originalStderr }()

	r, w, _ := os.Pipe()
	os.Stderr = w

	renderer := NewHelmRenderer()
	err := renderer.Execute(context.Background(), renderCtx, nil, true)

	w.Close()
	os.Stderr = originalStderr

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify that --skip-crds flag was used (should appear in verbose output)
	if !strings.Contains(output, "--skip-crds") {
		t.Errorf("Expected --skip-crds flag in helm command, but it was not found in output: %s", output)
	}
}

func TestHelmRenderer_HelmOptionsOverride(t *testing.T) {
	// Skip if helm is not available
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not found in PATH")
	}

	// Create a temporary directory with a simple chart
	tempDir := t.TempDir()
	createTestHelmChart(t, tempDir)

	// Create ArgoCD Application without skipCrds in spec
	app := &Application{
		Spec: ApplicationSpec{
			Source: &ApplicationSource{
				Path: "",
				Helm: &ApplicationSourceHelm{
					// No SkipCrds option here
				},
			},
		},
	}

	renderCtx := &RenderContext{
		Application: app,
		Source:      app.Spec.Source,
		RepoPath:    tempDir,
		AppName:     "test-app",
		Namespace:   "default",
	}

	// Create HelmOptions with SkipCrds enabled
	helmOpts := &HelmOptions{
		SkipCrds:  true,
		SkipTests: true,
	}

	// Capture stderr to verify the options are applied (verbose output goes to stderr)
	originalStderr := os.Stderr
	defer func() { os.Stderr = originalStderr }()

	r, w, _ := os.Pipe()
	os.Stderr = w

	renderer := NewHelmRenderer()
	err := renderer.Execute(context.Background(), renderCtx, helmOpts, true)

	w.Close()
	os.Stderr = originalStderr

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify that both --skip-crds and --skip-tests flags were used
	if !strings.Contains(output, "--skip-crds") {
		t.Errorf("Expected --skip-crds flag in helm command, but it was not found in output: %s", output)
	}
	if !strings.Contains(output, "--skip-tests") {
		t.Errorf("Expected --skip-tests flag in helm command, but it was not found in output: %s", output)
	}
}

// createTestHelmChart creates a minimal valid Helm chart for testing
func createTestHelmChart(t *testing.T, dir string) {
	// Create Chart.yaml
	chartYaml := `apiVersion: v2
name: test-chart
version: 1.0.0
description: A test chart
`
	err := os.WriteFile(dir+"/Chart.yaml", []byte(chartYaml), 0644)
	if err != nil {
		t.Fatalf("Failed to create Chart.yaml: %v", err)
	}

	// Create templates directory
	templatesDir := dir + "/templates"
	err = os.Mkdir(templatesDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create templates directory: %v", err)
	}

	// Create a simple deployment template
	deploymentYaml := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: test
        image: nginx:latest
`
	err = os.WriteFile(templatesDir+"/deployment.yaml", []byte(deploymentYaml), 0644)
	if err != nil {
		t.Fatalf("Failed to create deployment template: %v", err)
	}
}
