package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type kustomizeRenderer struct{}

// NewKustomizeRenderer creates a new Kustomize renderer
func NewKustomizeRenderer() KustomizeRenderer {
	return &kustomizeRenderer{}
}

// kustomizationYaml represents the structure of kustomization.yaml file
type kustomizationYaml struct {
	APIVersion        string             `yaml:"apiVersion,omitempty"`
	Kind              string             `yaml:"kind,omitempty"`
	Resources         []string           `yaml:"resources,omitempty"`
	Images            []kustomizeImage   `yaml:"images,omitempty"`
	CommonLabels      map[string]string  `yaml:"commonLabels,omitempty"`
	CommonAnnotations map[string]string  `yaml:"commonAnnotations,omitempty"`
	NamePrefix        string             `yaml:"namePrefix,omitempty"`
	NameSuffix        string             `yaml:"nameSuffix,omitempty"`
	Namespace         string             `yaml:"namespace,omitempty"`
	Replicas          []kustomizeReplica `yaml:"replicas,omitempty"`
	Patches           []KustomizePatch   `yaml:"patches,omitempty"`
	Components        []string           `yaml:"components,omitempty"`
	GeneratorOptions  *generatorOptions  `yaml:"generatorOptions,omitempty"`
}

// kustomizeImage represents an image override in kustomization.yaml
type kustomizeImage struct {
	Name    string `yaml:"name,omitempty"`
	NewName string `yaml:"newName,omitempty"`
	NewTag  string `yaml:"newTag,omitempty"`
	Digest  string `yaml:"digest,omitempty"`
}

// kustomizeReplica represents a replica override in kustomization.yaml
type kustomizeReplica struct {
	Name  string `yaml:"name"`
	Count int    `yaml:"count"`
}

type generatorOptions struct {
	Labels                map[string]string `yaml:"labels,omitempty"`
	Annotations           map[string]string `yaml:"annotations,omitempty"`
	DisableNameSuffixHash bool              `yaml:"disableNameSuffixHash,omitempty"`
}

func (kr *kustomizeRenderer) Execute(ctx context.Context, renderCtx *RenderContext, opts *KustomizeOptions, verbose bool) error {
	kustomizeBinary := kr.getBinaryPath(opts)
	kustomizePath := kr.getKustomizePath(renderCtx)

	workDir, cleanup, err := kr.prepareWorkDir(kustomizePath, renderCtx)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	args := kr.buildKustomizeArgs(workDir, opts)
	return kr.runKustomizeCommand(ctx, kustomizeBinary, args, workDir, verbose)
}

func (kr *kustomizeRenderer) getBinaryPath(opts *KustomizeOptions) string {
	if opts != nil && opts.BinaryPath != "" {
		return opts.BinaryPath
	}
	return "kustomize"
}

func (kr *kustomizeRenderer) getKustomizePath(renderCtx *RenderContext) string {
	if renderCtx.Source.Path != "" {
		return path.Join(renderCtx.RepoPath, renderCtx.Source.Path)
	}
	return renderCtx.RepoPath
}

func (kr *kustomizeRenderer) prepareWorkDir(kustomizePath string, renderCtx *RenderContext) (string, func(), error) {
	if !kr.needsOverlay(renderCtx) {
		return kustomizePath, nil, nil
	}

	workDir, err := kr.createKustomizationOverlay(kustomizePath, renderCtx.Source.Kustomize)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create kustomization overlay: %w", err)
	}

	cleanup := func() { os.RemoveAll(workDir) }
	return workDir, cleanup, nil
}

func (kr *kustomizeRenderer) needsOverlay(renderCtx *RenderContext) bool {
	return renderCtx.Source.Kustomize != nil && kr.hasKustomizeOptions(renderCtx.Source.Kustomize)
}

func (kr *kustomizeRenderer) buildKustomizeArgs(workDir string, opts *KustomizeOptions) []string {
	args := []string{"build", workDir}

	if opts != nil && opts.BuildOptions != "" {
		buildOpts := strings.Fields(opts.BuildOptions)
		args = append(args, buildOpts...)
	}

	return args
}

func (kr *kustomizeRenderer) runKustomizeCommand(ctx context.Context, binary string, args []string, workDir string, verbose bool) error {
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if verbose {
		kr.printVerboseInfo(binary, args, workDir)
	}

	return cmd.Run()
}

func (kr *kustomizeRenderer) printVerboseInfo(binary string, args []string, workDir string) {
	fmt.Fprintf(os.Stderr, "Source Type: kustomize\n")
	fmt.Fprintf(os.Stderr, "Command: %s\n", strings.Join(append([]string{binary}, args...), " "))
	fmt.Fprintf(os.Stderr, "Working Directory: %s\n", workDir)
	fmt.Fprintf(os.Stderr, "---\n")
}

// hasKustomizeOptions checks if any ArgoCD kustomize options are specified
func (kr *kustomizeRenderer) hasKustomizeOptions(kustomize *ApplicationSourceKustomize) bool {
	return len(kustomize.Images) > 0 ||
		len(kustomize.CommonLabels) > 0 ||
		len(kustomize.CommonAnnotations) > 0 ||
		kustomize.NamePrefix != "" ||
		kustomize.NameSuffix != "" ||
		kustomize.Namespace != "" ||
		len(kustomize.Replicas) > 0 ||
		len(kustomize.Patches) > 0 ||
		len(kustomize.Components) > 0 ||
		kustomize.ForceCommonLabels ||
		kustomize.ForceCommonAnnotations
}

// createKustomizationOverlay creates a temporary kustomization overlay with ArgoCD options
func (kr *kustomizeRenderer) createKustomizationOverlay(basePath string, kustomize *ApplicationSourceKustomize) (string, error) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "kustomize-overlay-")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Convert basePath to absolute path
	absoluteBasePath, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Create symlink to base path in temp directory
	baseName := "base"
	baseLink := filepath.Join(tempDir, baseName)
	err = os.Symlink(absoluteBasePath, baseLink)
	if err != nil {
		return "", fmt.Errorf("failed to create symlink: %w", err)
	}

	// Create kustomization.yaml with overlays
	kustomization := kustomizationYaml{
		APIVersion: "kustomize.config.k8s.io/v1beta1",
		Kind:       "Kustomization",
		Resources:  []string{baseName},
	}

	// Add images
	if len(kustomize.Images) > 0 {
		images := make([]kustomizeImage, len(kustomize.Images))
		for i, img := range kustomize.Images {
			images[i] = kr.parseKustomizeImage(string(img))
		}
		kustomization.Images = images
	}

	// Add commonLabels
	if len(kustomize.CommonLabels) > 0 {
		kustomization.CommonLabels = kustomize.CommonLabels
	}

	// Add commonAnnotations
	if len(kustomize.CommonAnnotations) > 0 {
		kustomization.CommonAnnotations = kustomize.CommonAnnotations
	}

	// Add namePrefix
	if kustomize.NamePrefix != "" {
		kustomization.NamePrefix = kustomize.NamePrefix
	}

	// Add nameSuffix
	if kustomize.NameSuffix != "" {
		kustomization.NameSuffix = kustomize.NameSuffix
	}

	// Add namespace
	if kustomize.Namespace != "" {
		kustomization.Namespace = kustomize.Namespace
	}

	// Add replicas
	if len(kustomize.Replicas) > 0 {
		replicas := make([]kustomizeReplica, len(kustomize.Replicas))
		for i, rep := range kustomize.Replicas {
			count, err := rep.GetIntCount()
			if err != nil {
				return "", fmt.Errorf("invalid replica count for %s: %w", rep.Name, err)
			}
			replicas[i] = kustomizeReplica{
				Name:  rep.Name,
				Count: count,
			}
		}
		kustomization.Replicas = replicas
	}

	// Add patches
	if len(kustomize.Patches) > 0 {
		kustomization.Patches = kustomize.Patches
	}

	// Add components
	if len(kustomize.Components) > 0 {
		kustomization.Components = kustomize.Components
	}

	// Handle force common labels/annotations via generatorOptions
	if kustomize.ForceCommonLabels || kustomize.ForceCommonAnnotations {
		kustomization.GeneratorOptions = &generatorOptions{}
		if kustomize.ForceCommonLabels && len(kustomize.CommonLabels) > 0 {
			kustomization.GeneratorOptions.Labels = kustomize.CommonLabels
		}
		if kustomize.ForceCommonAnnotations && len(kustomize.CommonAnnotations) > 0 {
			kustomization.GeneratorOptions.Annotations = kustomize.CommonAnnotations
		}
	}

	// Marshal kustomization to YAML
	yamlData, err := yaml.Marshal(&kustomization)
	if err != nil {
		return "", fmt.Errorf("failed to marshal kustomization.yaml: %w", err)
	}

	// Write kustomization.yaml to temp directory
	kustomizationPath := filepath.Join(tempDir, "kustomization.yaml")
	err = os.WriteFile(kustomizationPath, yamlData, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write kustomization.yaml: %w", err)
	}

	return tempDir, nil
}

// parseKustomizeImage parses ArgoCD KustomizeImage format into kustomize format
// Format: [old_image_name=]<image_name>:<image_tag>
func (kr *kustomizeRenderer) parseKustomizeImage(imageStr string) kustomizeImage {
	img := kustomizeImage{}

	// Check if there's an old image name specified (format: old=new)
	if strings.Contains(imageStr, "=") {
		parts := strings.SplitN(imageStr, "=", 2)
		img.Name = parts[0]
		imageStr = parts[1]
	}

	// Parse new image name and tag/digest
	if strings.Contains(imageStr, "@") {
		// Digest format: image@sha256:...
		parts := strings.SplitN(imageStr, "@", 2)
		img.NewName = parts[0]
		img.Digest = parts[1]
	} else if strings.Contains(imageStr, ":") {
		// Tag format: image:tag
		parts := strings.SplitN(imageStr, ":", 2)
		img.NewName = parts[0]
		img.NewTag = parts[1]
	} else {
		// Just image name
		img.NewName = imageStr
	}

	// If no original name specified, use the new name
	if img.Name == "" {
		img.Name = img.NewName
	}

	return img
}
