package internal

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// GetAppSourceType determines the source type by checking for specific files and configurations
// This is a lightweight version inspired by ArgoCD's source type detection
func GetAppSourceType(ctx context.Context, source *ApplicationSource, appPath, repoPath, appName string) (ApplicationSourceType, error) {
	// First check if explicitly set in the source
	if source.Helm != nil {
		return ApplicationSourceTypeHelm, nil
	}
	if source.Kustomize != nil {
		return ApplicationSourceTypeKustomize, nil
	}
	if source.Directory != nil {
		return ApplicationSourceTypeDirectory, nil
	}

	// If not explicit, detect based on files in the directory
	// Combine repoPath with appPath to get the full path to check
	var searchPath string
	if appPath == "" || appPath == repoPath {
		// If appPath is empty or same as repoPath, just use repoPath
		searchPath = repoPath
	} else {
		// Combine repoPath with appPath (assuming appPath is relative)
		searchPath = filepath.Join(repoPath, appPath)
	}

	return detectSourceTypeFromDirectory(searchPath)
}

// detectSourceTypeFromDirectory examines files in a directory to determine the source type
func detectSourceTypeFromDirectory(path string) (ApplicationSourceType, error) {
	// Check for Helm chart
	if isHelmChart(path) {
		return ApplicationSourceTypeHelm, nil
	}

	// Check for Kustomize
	if isKustomizeApp(path) {
		return ApplicationSourceTypeKustomize, nil
	}

	// Default to directory for plain YAML files
	return ApplicationSourceTypeDirectory, nil
}

// isHelmChart checks if the directory contains a Helm chart
func isHelmChart(path string) bool {
	chartFile := filepath.Join(path, "Chart.yaml")
	if _, err := os.Stat(chartFile); err == nil {
		return true
	}

	chartFileAlt := filepath.Join(path, "Chart.yml")
	if _, err := os.Stat(chartFileAlt); err == nil {
		return true
	}

	return false
}

// isKustomizeApp checks if the directory contains a Kustomize application
func isKustomizeApp(path string) bool {
	kustomizeFiles := []string{
		"kustomization.yaml",
		"kustomization.yml",
		"Kustomization",
	}

	for _, file := range kustomizeFiles {
		if _, err := os.Stat(filepath.Join(path, file)); err == nil {
			return true
		}
	}

	return false
}

// hasManifestFiles checks if directory contains YAML/JSON manifest files
func hasManifestFiles(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".yaml" || ext == ".yml" || ext == ".json" {
			return true
		}
	}

	return false
}
