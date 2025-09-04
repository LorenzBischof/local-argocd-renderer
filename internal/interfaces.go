package internal

import (
	"context"
)

type HelmRenderer interface {
	Execute(ctx context.Context, renderCtx *RenderContext, opts *HelmOptions, verbose bool) error
}

type KustomizeRenderer interface {
	Execute(ctx context.Context, renderCtx *RenderContext, opts *KustomizeOptions, verbose bool) error
}

type DirectoryRenderer interface {
	Execute(ctx context.Context, renderCtx *RenderContext, verbose bool) error
}
