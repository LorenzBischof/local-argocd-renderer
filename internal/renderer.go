package internal

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

type Renderer interface {
	ExecuteCommand(ctx context.Context, req *RenderRequest, verbose bool) error
}

type renderer struct {
	helm      HelmRenderer
	kustomize KustomizeRenderer
	directory DirectoryRenderer
}

func NewRenderer() Renderer {
	return &renderer{
		helm:      NewHelmRenderer(),
		kustomize: NewKustomizeRenderer(),
		directory: NewDirectoryRenderer(),
	}
}

func (r *renderer) ExecuteCommand(ctx context.Context, req *RenderRequest, verbose bool) error {
	if err := r.validateRequest(req); err != nil {
		return err
	}

	source := r.getSource(req.Application)
	if source == nil {
		return fmt.Errorf("no source found in application")
	}

	sourceType := r.detectSourceType(source)
	renderCtx := r.buildRenderContext(req, source, sourceType)

	return r.executeByType(ctx, renderCtx, req, verbose)
}

func (r *renderer) validateRequest(req *RenderRequest) error {
	if req.Application == nil {
		return fmt.Errorf("application is required")
	}
	if req.RepoPath == "" {
		return fmt.Errorf("repoPath is required")
	}
	return nil
}

func (r *renderer) getSource(app *v1alpha1.Application) *v1alpha1.ApplicationSource {
	if app.Spec.HasMultipleSources() {
		return app.Spec.GetSourcePtrByIndex(0)
	}
	return app.Spec.Source
}

func (r *renderer) detectSourceType(source *v1alpha1.ApplicationSource) v1alpha1.ApplicationSourceType {
	if source.Helm != nil {
		return v1alpha1.ApplicationSourceTypeHelm
	}
	if source.Kustomize != nil {
		return v1alpha1.ApplicationSourceTypeKustomize
	}
	if source.Directory != nil {
		return v1alpha1.ApplicationSourceTypeDirectory
	}
	if source.Plugin != nil {
		return v1alpha1.ApplicationSourceTypePlugin
	}
	return v1alpha1.ApplicationSourceTypeDirectory
}

func (r *renderer) buildRenderContext(req *RenderRequest, source *v1alpha1.ApplicationSource, sourceType v1alpha1.ApplicationSourceType) *RenderContext {
	return &RenderContext{
		Application: req.Application,
		Source:      source,
		RepoPath:    req.RepoPath,
		AppName:     req.Application.Name,
		Namespace:   req.Application.Spec.Destination.Namespace,
		KubeVersion: req.KubeVersion,
		SourceType:  sourceType,
	}
}

func (r *renderer) executeByType(ctx context.Context, renderCtx *RenderContext, req *RenderRequest, verbose bool) error {
	switch renderCtx.SourceType {
	case v1alpha1.ApplicationSourceTypeHelm:
		return r.helm.Execute(ctx, renderCtx, req.HelmOptions, verbose)
	case v1alpha1.ApplicationSourceTypeKustomize:
		return r.kustomize.Execute(ctx, renderCtx, req.KustomizeOptions, verbose)
	case v1alpha1.ApplicationSourceTypeDirectory:
		return r.directory.Execute(ctx, renderCtx, verbose)
	default:
		return fmt.Errorf("unsupported source type: %s", renderCtx.SourceType)
	}
}

type RenderContext struct {
	Application *v1alpha1.Application
	Source      *v1alpha1.ApplicationSource
	RepoPath    string
	AppName     string
	Namespace   string
	KubeVersion string
	SourceType  v1alpha1.ApplicationSourceType
}
