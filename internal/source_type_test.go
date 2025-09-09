package internal

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGetAppSourceType_KustomizeDetection(t *testing.T) {
	tests := []struct {
		name         string
		files        []string
		expectedType ApplicationSourceType
	}{
		{
			name:         "detect kustomization.yaml",
			files:        []string{"kustomization.yaml"},
			expectedType: ApplicationSourceTypeKustomize,
		},
		{
			name:         "detect kustomization.yml",
			files:        []string{"kustomization.yml"},
			expectedType: ApplicationSourceTypeKustomize,
		},
		{
			name:         "detect Kustomization",
			files:        []string{"Kustomization"},
			expectedType: ApplicationSourceTypeKustomize,
		},
		{
			name:         "detect helm Chart.yaml",
			files:        []string{"Chart.yaml"},
			expectedType: ApplicationSourceTypeHelm,
		},
		{
			name:         "detect helm Chart.yml",
			files:        []string{"Chart.yml"},
			expectedType: ApplicationSourceTypeHelm,
		},
		{
			name:         "fallback to directory for plain yaml",
			files:        []string{"deployment.yaml"},
			expectedType: ApplicationSourceTypeDirectory,
		},
		{
			name:         "helm takes precedence over kustomize",
			files:        []string{"Chart.yaml", "kustomization.yaml"},
			expectedType: ApplicationSourceTypeHelm,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()

			// Create test files
			for _, file := range tt.files {
				content := "# test content"
				if file == "Chart.yaml" || file == "Chart.yml" {
					content = `name: test
version: 1.0.0
`
				} else if filepath.Base(file) == "kustomization.yaml" || filepath.Base(file) == "kustomization.yml" || filepath.Base(file) == "Kustomization" {
					content = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- deployment.yaml
`
				}

				err := os.WriteFile(filepath.Join(tmpDir, file), []byte(content), 0644)
				if err != nil {
					t.Fatalf("Failed to create test file %s: %v", file, err)
				}
			}

			// Test source type detection
			source := &ApplicationSource{}
			sourceType, err := GetAppSourceType(context.Background(), source, tmpDir, tmpDir, "test-app")
			if err != nil {
				t.Fatalf("GetAppSourceType failed: %v", err)
			}

			if sourceType != tt.expectedType {
				t.Errorf("Expected source type %s, got %s", tt.expectedType, sourceType)
			}
		})
	}
}

func TestGetAppSourceType_ExplicitSource(t *testing.T) {
	tmpDir := t.TempDir()

	// Create both Chart.yaml and kustomization.yaml
	err := os.WriteFile(filepath.Join(tmpDir, "Chart.yaml"), []byte("name: test\nversion: 1.0.0"), 0644)
	if err != nil {
		t.Fatalf("Failed to create Chart.yaml: %v", err)
	}

	err = os.WriteFile(filepath.Join(tmpDir, "kustomization.yaml"), []byte("apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization"), 0644)
	if err != nil {
		t.Fatalf("Failed to create kustomization.yaml: %v", err)
	}

	tests := []struct {
		name         string
		source       *ApplicationSource
		expectedType ApplicationSourceType
	}{
		{
			name: "explicit helm source",
			source: &ApplicationSource{
				Helm: &ApplicationSourceHelm{},
			},
			expectedType: ApplicationSourceTypeHelm,
		},
		{
			name: "explicit kustomize source",
			source: &ApplicationSource{
				Kustomize: &ApplicationSourceKustomize{},
			},
			expectedType: ApplicationSourceTypeKustomize,
		},
		{
			name: "explicit directory source",
			source: &ApplicationSource{
				Directory: &ApplicationSourceDirectory{},
			},
			expectedType: ApplicationSourceTypeDirectory,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sourceType, err := GetAppSourceType(context.Background(), tt.source, tmpDir, tmpDir, "test-app")
			if err != nil {
				t.Fatalf("GetAppSourceType failed: %v", err)
			}

			if sourceType != tt.expectedType {
				t.Errorf("Expected source type %s, got %s", tt.expectedType, sourceType)
			}
		})
	}
}

func TestGetAppSourceType_RepoPathAndAppPathCombination(t *testing.T) {
	tests := []struct {
		name         string
		appSubPath   string
		files        []string
		expectedType ApplicationSourceType
	}{
		{
			name:       "detect helm in subdirectory",
			appSubPath: "helm-app",
			files:      []string{"helm-app/Chart.yaml"},
			expectedType: ApplicationSourceTypeHelm,
		},
		{
			name:       "detect kustomize in subdirectory",
			appSubPath: "kustomize-app",
			files:      []string{"kustomize-app/kustomization.yaml"},
			expectedType: ApplicationSourceTypeKustomize,
		},
		{
			name:       "detect directory in nested path",
			appSubPath: "apps/frontend",
			files:      []string{"apps/frontend/deployment.yaml"},
			expectedType: ApplicationSourceTypeDirectory,
		},
		{
			name:       "helm in deep nested path",
			appSubPath: "charts/services/api",
			files:      []string{"charts/services/api/Chart.yaml"},
			expectedType: ApplicationSourceTypeHelm,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory as repo root
			repoDir := t.TempDir()

			// Create test files in the specified subdirectories
			for _, file := range tt.files {
				fullPath := filepath.Join(repoDir, file)
				
				// Create directory structure if needed
				if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
					t.Fatalf("Failed to create directory structure for %s: %v", file, err)
				}

				// Create appropriate content based on file type
				var content string
				switch filepath.Base(file) {
				case "Chart.yaml", "Chart.yml":
					content = `name: test-chart
version: 1.0.0
description: A test chart
`
				case "kustomization.yaml", "kustomization.yml", "Kustomization":
					content = `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- deployment.yaml
`
				default:
					content = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
spec:
  replicas: 1
`
				}

				if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to create test file %s: %v", fullPath, err)
				}
			}

			// Test source type detection with different repoPath and appPath
			source := &ApplicationSource{}
			sourceType, err := GetAppSourceType(context.Background(), source, tt.appSubPath, repoDir, "test-app")
			if err != nil {
				t.Fatalf("GetAppSourceType failed: %v", err)
			}

			if sourceType != tt.expectedType {
				t.Errorf("Expected source type %s, got %s", tt.expectedType, sourceType)
			}
		})
	}
}
