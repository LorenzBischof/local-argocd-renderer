package renderer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/controller"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/reposerver/repository"
	"github.com/argoproj/argo-cd/v3/util/git"
)

// TemplateOptions contains options for the templating process
type TemplateOptions struct {
	ApplicationFile string
	RepoRoot        string
	MaxManifestSize string
}

// TemplateResult contains the results of the templating process
type TemplateResult struct {
	Objects          []*unstructured.Unstructured
	Warnings         []string
	SourcesProcessed int
}

// TemplateFromApplication processes an ArgoCD Application and returns templated manifests
func TemplateFromApplication(ctx context.Context, opts TemplateOptions) (*TemplateResult, error) {
	requests, err := buildRequestFromApplication(opts.ApplicationFile)
	if err != nil {
		return nil, fmt.Errorf("error parsing Application CRD: %w", err)
	}

	var allManifests []string
	var warnings []string

	// Process each source
	for sourceIndex, q := range requests {
		appPath := q.ApplicationSource.Path
		repoRoot := opts.RepoRoot
		if repoRoot == "" {
			repoRoot = "."
		}

		appSourceType, err := repository.GetAppSourceType(ctx, q.ApplicationSource, appPath, repoRoot, q.AppName, q.EnabledSourceTypes, []string{}, []string{})
		if err != nil {
			return nil, fmt.Errorf("error getting app source type: %w", err)
		}

		// For Kustomize sources, create a temporary overlay to avoid modifying the original
		if appSourceType == v1alpha1.ApplicationSourceTypeKustomize {
			tempDir, err := os.MkdirTemp(".", "kustomize-overlay-*")
			if err != nil {
				return nil, fmt.Errorf("error creating temp directory for Kustomize overlay: %w", err)
			}
			defer os.RemoveAll(tempDir)

			relPath, err := filepath.Rel(tempDir, appPath)
			if err != nil {
				os.RemoveAll(tempDir)
				return nil, fmt.Errorf("error calculating relative path: %w", err)
			}

			// Create a kustomization.yaml that references the original path
			kustomizationContent := fmt.Sprintf(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- %s
`, relPath)

			kustomizationPath := filepath.Join(tempDir, "kustomization.yaml")
			if err := os.WriteFile(kustomizationPath, []byte(kustomizationContent), 0644); err != nil {
				os.RemoveAll(tempDir)
				return nil, fmt.Errorf("error writing kustomization.yaml: %w", err)
			}

			appPath = tempDir
		}

		maxSize := resource.MustParse("10Mi")
		if opts.MaxManifestSize != "" {
			maxSize = resource.MustParse(opts.MaxManifestSize)
		}

		// Call the core GenerateManifests function directly
		response, err := repository.GenerateManifests(
			ctx,
			appPath,               // app path within repo
			repoRoot,              // repo root (current directory)
			"",                    // revision (empty for local files)
			q,                     // manifest request
			true,                  // isLocal=true - crucial for local operation!
			&git.NoopCredsStore{}, // no git credentials needed
			maxSize,               // max combined manifest size
			nil,                   // no temp paths needed for local operation
		)

		if err != nil {
			return nil, fmt.Errorf("error generating manifests for source %d: %w", sourceIndex+1, err)
		}

		// Collect manifests from this source
		allManifests = append(allManifests, response.Manifests...)
	}

	// Parse manifests into unstructured objects for deduplication
	var targetObjects []*unstructured.Unstructured
	for _, manifest := range allManifests {
		var obj unstructured.Unstructured
		if err := json.Unmarshal([]byte(manifest), &obj); err != nil {
			warnings = append(warnings, fmt.Sprintf("Failed to parse manifest as JSON: %v", err))
			continue
		}
		targetObjects = append(targetObjects, &obj)
	}

	// Deduplicate target objects using the library function
	infoProvider := &resourceInfoProviderStub{}
	dedupedObjects, conditions, err := controller.DeduplicateTargetObjects(requests[0].Namespace, targetObjects, infoProvider)
	if err != nil {
		return nil, fmt.Errorf("error deduplicating target objects: %w", err)
	}

	// Collect duplicate warnings
	for _, condition := range conditions {
		warnings = append(warnings, condition.Message)
	}

	return &TemplateResult{
		Objects:          dedupedObjects,
		Warnings:         warnings,
		SourcesProcessed: len(requests),
	}, nil
}

// TemplateFromApplicationYAML processes an ArgoCD Application from YAML content
func TemplateFromApplicationYAML(ctx context.Context, yamlContent string, repoRoot string) (*TemplateResult, error) {
	// Write YAML to temporary file
	tempFile, err := os.CreateTemp("", "argocd-app-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("error creating temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	if _, err := tempFile.WriteString(yamlContent); err != nil {
		return nil, fmt.Errorf("error writing to temp file: %w", err)
	}

	opts := TemplateOptions{
		ApplicationFile: tempFile.Name(),
		RepoRoot:        repoRoot,
	}

	return TemplateFromApplication(ctx, opts)
}

func buildRequestFromApplication(filePath string) ([]*apiclient.ManifestRequest, error) {
	var data []byte
	var err error

	if filePath == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(filePath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read application file: %w", err)
	}

	var app v1alpha1.Application
	if err := yaml.Unmarshal(data, &app); err != nil {
		return nil, fmt.Errorf("failed to parse Application YAML: %w", err)
	}

	if app.Kind != "Application" {
		return nil, fmt.Errorf("expected kind 'Application', got '%s'", app.Kind)
	}

	sources := app.Spec.GetSources()
	if len(sources) == 0 {
		return nil, fmt.Errorf("no sources found in application spec")
	}

	var requests []*apiclient.ManifestRequest

	for i, source := range sources {
		if source.RepoURL == "" {
			return nil, fmt.Errorf("source[%d].repoURL is required", i)
		}

		// Handle remote Helm charts by downloading them to a temporary directory
		modifiedSource := sources[i]
		if source.IsHelm() {
			chartDir, err := downloadHelmChart(source.RepoURL, source.Chart, source.TargetRevision)
			if err != nil {
				return nil, fmt.Errorf("failed to download Helm chart for source[%d]: %w", i, err)
			}

			// Modify the source to point to the local directory
			modifiedSource = sources[i]
			modifiedSource.Path = chartDir
			modifiedSource.Chart = "" // Clear chart field since we're now using a local path
		}

		req := &apiclient.ManifestRequest{
			Repo: &v1alpha1.Repository{
				Repo: source.RepoURL,
			},
			ApplicationSource: &modifiedSource, // Use the potentially modified source
			AppName:           app.Name,
			Namespace:         app.Spec.Destination.Namespace,
			Revision:          source.TargetRevision,
			EnabledSourceTypes: map[string]bool{
				string(v1alpha1.ApplicationSourceTypeHelm):      true,
				string(v1alpha1.ApplicationSourceTypeKustomize): true,
				string(v1alpha1.ApplicationSourceTypeDirectory): true,
			},
			AppLabelKey:        "app.kubernetes.io/instance",
			TrackingMethod:     string(v1alpha1.TrackingMethodLabel),
			InstallationID:     "local-cli",
			ProjectName:        app.Spec.Project,
			HasMultipleSources: len(sources) > 1,
		}

		requests = append(requests, req)
	}

	return requests, nil
}

// downloadHelmChart downloads a remote Helm chart to the XDG cache directory with reproducible naming
func downloadHelmChart(repoURL, chartName, version string) (string, error) {
	// Get XDG cache directory
	cacheDir, err := getCacheDir()
	if err != nil {
		return "", fmt.Errorf("failed to get cache directory: %w", err)
	}

	// Create subdirectory for helm charts
	helmCacheDir := filepath.Join(cacheDir, "local-argocd-renderer")
	if err := os.MkdirAll(helmCacheDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create helm cache directory: %w", err)
	}

	// Generate reproducible filename based on repoURL, chartName, and version
	hashInput := fmt.Sprintf("%s|%s|%s", repoURL, chartName, version)
	hash := sha256.Sum256([]byte(hashInput))
	hashStr := hex.EncodeToString(hash[:])
	chartDir := filepath.Join(helmCacheDir, fmt.Sprintf("chart-%s", hashStr))

	// Check if chart is already cached
	if _, err := os.Stat(chartDir); err == nil {
		return chartDir, nil
	}

	// Download the chart
	args := []string{"pull", fmt.Sprintf("%s/%s", repoURL, chartName)}
	if version != "" {
		args = append(args, "--version", version)
	}
	args = append(args, "--destination", helmCacheDir)
	args = append(args, "--untar")

	cmd := exec.Command("helm", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("helm pull failed: %w\nOutput: %s", err, string(output))
	}

	// Find the extracted chart directory (helm pull creates a directory with the chart name)
	extractedDir := filepath.Join(helmCacheDir, chartName)

	// Rename to our reproducible name
	if err := os.Rename(extractedDir, chartDir); err != nil {
		return "", fmt.Errorf("failed to rename chart directory: %w", err)
	}

	return chartDir, nil
}

// getCacheDir returns the XDG cache directory
func getCacheDir() (string, error) {
	if cacheDir := os.Getenv("XDG_CACHE_HOME"); cacheDir != "" {
		return cacheDir, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".cache"), nil
}

// resourceInfoProviderStub is a simple implementation of kubeutil.ResourceInfoProvider
// that treats all resources as cluster-scoped (returns false for IsNamespaced)
type resourceInfoProviderStub struct{}

func (r *resourceInfoProviderStub) IsNamespaced(_ schema.GroupKind) (bool, error) {
	return true, nil
}
