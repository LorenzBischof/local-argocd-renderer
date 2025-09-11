package renderer

import "context"

// ApplicationSourceType represents the type of application source
type ApplicationSourceType string

const (
	ApplicationSourceTypeHelm      ApplicationSourceType = "Helm"
	ApplicationSourceTypeKustomize ApplicationSourceType = "Kustomize"
	ApplicationSourceTypeDirectory ApplicationSourceType = "Directory"
)

// Application represents a simplified ArgoCD Application
type Application struct {
	Name string
	Spec ApplicationSpec
}

// ApplicationSpec represents the specification of an Application
type ApplicationSpec struct {
	Source      *ApplicationSource
	Destination ApplicationDestination
}

// ApplicationSource represents a source for an application
type ApplicationSource struct {
	RepoURL        string
	Path           string
	TargetRevision string
	Helm           *ApplicationSourceHelm
	Kustomize      *ApplicationSourceKustomize
	Directory      *ApplicationSourceDirectory
}

// ApplicationDestination represents the destination for an application
type ApplicationDestination struct {
	Namespace string
}

// ApplicationSourceHelm represents Helm source options
type ApplicationSourceHelm struct {
	ReleaseName             string
	ValueFiles              []string
	Values                  string
	Parameters              []HelmParameter
	FileParameters          []HelmFileParameter
	IgnoreMissingValueFiles bool
	SkipCrds                bool
}

// HelmParameter represents a Helm parameter
type HelmParameter struct {
	Name        string
	Value       string
	ForceString bool
}

// HelmFileParameter represents a Helm file parameter
type HelmFileParameter struct {
	Name string
	Path string
}

// ApplicationSourceKustomize represents Kustomize source options
type ApplicationSourceKustomize struct {
	Images                 []KustomizeImage
	CommonLabels           map[string]string
	CommonAnnotations      map[string]string
	NamePrefix             string
	NameSuffix             string
	Namespace              string
	Replicas               []KustomizeReplica
	Patches                []KustomizePatch
	Components             []string
	ForceCommonLabels      bool
	ForceCommonAnnotations bool
}

// KustomizeImage represents a Kustomize image override
type KustomizeImage string

// KustomizeReplica represents a Kustomize replica override
type KustomizeReplica struct {
	Name  string
	Count string
}

// GetIntCount returns the replica count as an integer
func (r KustomizeReplica) GetIntCount() (int, error) {
	// Simple implementation - in real ArgoCD this is more complex
	if r.Count == "" {
		return 1, nil
	}
	// For simplicity, assume it's already an int or return 1
	return 1, nil
}

// KustomizePatch represents a Kustomize patch
type KustomizePatch struct {
	Patch  string
	Path   string
	Target *KustomizePatchTarget
}

// KustomizePatchTarget represents a patch target
type KustomizePatchTarget struct {
	Group     string
	Version   string
	Kind      string
	Name      string
	Namespace string
}

// ApplicationSourceDirectory represents directory source options
type ApplicationSourceDirectory struct {
	Recurse bool
	Include string
	Exclude string
}

// ValuesYAML returns the Values as YAML bytes
func (h *ApplicationSourceHelm) ValuesYAML() []byte {
	return []byte(h.Values)
}

// ValuesIsEmpty returns true if Values is empty
func (h *ApplicationSourceHelm) ValuesIsEmpty() bool {
	return h.Values == ""
}

// RenderRequest represents a request to render manifests
type RenderRequest struct {
	Application      *Application
	RepoPath         string
	KubeVersion      string
	HelmOptions      *HelmOptions
	KustomizeOptions *KustomizeOptions
}

// HelmOptions represents options for Helm rendering
type HelmOptions struct {
	SkipCrds    bool
	SkipTests   bool
	IncludeCrds bool
}

// KustomizeOptions represents options for Kustomize rendering
type KustomizeOptions struct {
	BuildOptions string
	BinaryPath   string
}

// RenderContext represents the context for rendering
type RenderContext struct {
	Application *Application
	Source      *ApplicationSource
	RepoPath    string
	AppName     string
	Namespace   string
	KubeVersion string
	SourceType  ApplicationSourceType
}

// Renderer interfaces
type HelmRenderer interface {
	Execute(ctx context.Context, renderCtx *RenderContext, opts *HelmOptions, verbose bool) error
}

type KustomizeRenderer interface {
	Execute(ctx context.Context, renderCtx *RenderContext, opts *KustomizeOptions, verbose bool) error
}

type DirectoryRenderer interface {
	Execute(ctx context.Context, renderCtx *RenderContext, verbose bool) error
}
