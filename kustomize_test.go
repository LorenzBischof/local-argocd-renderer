package renderer

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestKustomizeRenderer_NamePrefixOption(t *testing.T) {
	// Skip if kustomize is not available
	if _, err := exec.LookPath("kustomize"); err != nil {
		t.Skip("kustomize not found in PATH")
	}

	// Create a temporary directory with a simple kustomization
	tempDir := t.TempDir()
	createTestKustomization(t, tempDir)

	// Create ArgoCD Application with namePrefix option
	app := &Application{
		Spec: ApplicationSpec{
			Source: &ApplicationSource{
				Path: "",
				Kustomize: &ApplicationSourceKustomize{
					NamePrefix: "test-prefix-",
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

	renderer := NewKustomizeRenderer()
	result, err := renderer.Execute(context.Background(), renderCtx, nil, false)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify that namePrefix was applied to the deployment name
	if !strings.Contains(result.Output, "name: test-prefix-test-deployment") {
		t.Errorf("Expected namePrefix 'test-prefix-' to be applied to deployment name, but output was: %s", result.Output)
	}
}

func TestKustomizeRenderer_ImageOverrideOption(t *testing.T) {
	// Skip if kustomize is not available
	if _, err := exec.LookPath("kustomize"); err != nil {
		t.Skip("kustomize not found in PATH")
	}

	// Create a temporary directory with a simple kustomization
	tempDir := t.TempDir()
	createTestKustomization(t, tempDir)

	// Create ArgoCD Application with image override
	app := &Application{
		Spec: ApplicationSpec{
			Source: &ApplicationSource{
				Path: "",
				Kustomize: &ApplicationSourceKustomize{
					Images: []KustomizeImage{
						"nginx:latest=nginx:1.20",
					},
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

	renderer := NewKustomizeRenderer()
	result, err := renderer.Execute(context.Background(), renderCtx, nil, false)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify that image was overridden from nginx:latest to nginx:1.20
	if !strings.Contains(result.Output, "image: nginx:1.20") {
		t.Errorf("Expected image override to nginx:1.20, but output was: %s", result.Output)
	}

	// Verify old image is not present
	if strings.Contains(result.Output, "image: nginx:latest") {
		t.Errorf("Expected original image nginx:latest to be replaced, but it's still present in output: %s", result.Output)
	}
}

func TestKustomizeRenderer_KustomizeOptions(t *testing.T) {
	// Skip if kustomize is not available
	if _, err := exec.LookPath("kustomize"); err != nil {
		t.Skip("kustomize not found in PATH")
	}

	// Create a temporary directory with a simple kustomization
	tempDir := t.TempDir()
	createTestKustomization(t, tempDir)

	// Create ArgoCD Application without kustomize options
	app := &Application{
		Spec: ApplicationSpec{
			Source: &ApplicationSource{
				Path: "",
				// No Kustomize options in the application spec
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

	// Create KustomizeOptions with BuildOptions
	kustomizeOpts := &KustomizeOptions{
		BuildOptions: "--load-restrictor LoadRestrictionsNone",
	}

	// Capture stderr to verify the build options are applied (verbose output goes to stderr)
	originalStderr := os.Stderr
	defer func() { os.Stderr = originalStderr }()

	r, w, _ := os.Pipe()
	os.Stderr = w

	renderer := NewKustomizeRenderer()
	_, err := renderer.Execute(context.Background(), renderCtx, kustomizeOpts, true)

	w.Close()
	os.Stderr = originalStderr

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify that build options were included in the kustomize command
	if !strings.Contains(output, "--load-restrictor LoadRestrictionsNone") {
		t.Errorf("Expected build options '--load-restrictor LoadRestrictionsNone' in kustomize command, but it was not found in output: %s", output)
	}
}

// createTestKustomization creates a minimal valid Kustomization for testing
func createTestKustomization(t *testing.T, dir string) {
	// Create kustomization.yaml
	kustomizationYaml := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- deployment.yaml
`
	err := os.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte(kustomizationYaml), 0644)
	if err != nil {
		t.Fatalf("Failed to create kustomization.yaml: %v", err)
	}

	// Create a simple deployment manifest
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
	err = os.WriteFile(filepath.Join(dir, "deployment.yaml"), []byte(deploymentYaml), 0644)
	if err != nil {
		t.Fatalf("Failed to create deployment.yaml: %v", err)
	}
}
