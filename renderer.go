package renderer

import (
	"context"
	"fmt"
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

	// Use our lightweight GetAppSourceType for source type detection
	appPath := req.RepoPath
	if source.Path != "" {
		appPath = source.Path
	}

	sourceType, err := GetAppSourceType(ctx, source, appPath, req.RepoPath, req.Application.Name)
	if err != nil {
		return fmt.Errorf("error determining source type: %w", err)
	}

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

func (r *renderer) getSource(app *Application) *ApplicationSource {
	// Simplified - we only support single source for now
	return app.Spec.Source
}

func (r *renderer) buildRenderContext(req *RenderRequest, source *ApplicationSource, sourceType ApplicationSourceType) *RenderContext {
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
	case ApplicationSourceTypeHelm:
		return r.helm.Execute(ctx, renderCtx, req.HelmOptions, verbose)
	case ApplicationSourceTypeKustomize:
		return r.kustomize.Execute(ctx, renderCtx, req.KustomizeOptions, verbose)
	case ApplicationSourceTypeDirectory:
		return r.directory.Execute(ctx, renderCtx, verbose)
	default:
		return fmt.Errorf("unsupported source type: %s", renderCtx.SourceType)
	}
}
