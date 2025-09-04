package internal

import (
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

type RenderRequest struct {
	Application      *v1alpha1.Application
	RepoPath         string
	KubeVersion      string
	HelmOptions      *HelmOptions
	KustomizeOptions *KustomizeOptions
}

type HelmOptions struct {
	SkipCrds    bool
	SkipTests   bool
	IncludeCrds bool
}

type KustomizeOptions struct {
	BuildOptions string
	BinaryPath   string
}
