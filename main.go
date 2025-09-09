package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"sigs.k8s.io/yaml"

	"github.com/lorenzbischof/local-argocd-renderer/internal"
)

type options struct {
	appFile        string
	repoPath       string
	kubeVersion    string
	helmSkipCrds   bool
	helmSkipTests  bool
	kustomizeBuild string
	verbose        bool
}

func main() {
	opts := parseFlags()

	app, err := loadApplication(opts.appFile)
	exitOnError(err, "loading application")

	req := buildRenderRequest(app, opts)

	r := internal.NewRenderer()
	err = r.ExecuteCommand(context.Background(), req, opts.verbose)
	exitOnError(err, "executing command")
}

func parseFlags() *options {
	opts := &options{}
	flag.StringVar(&opts.appFile, "app", "", "Path to ArgoCD Application YAML file (required)")
	flag.StringVar(&opts.repoPath, "repo", "", "Path to local repository containing manifests (required)")
	flag.StringVar(&opts.kubeVersion, "kube-version", "", "Kubernetes version to use for rendering (optional)")
	flag.BoolVar(&opts.helmSkipCrds, "helm-skip-crds", false, "Skip CRDs when rendering Helm charts")
	flag.BoolVar(&opts.helmSkipTests, "helm-skip-tests", false, "Skip tests when rendering Helm charts")
	flag.StringVar(&opts.kustomizeBuild, "kustomize-build-options", "", "Additional kustomize build options")
	flag.BoolVar(&opts.verbose, "verbose", false, "Verbose output showing commands")
	flag.Parse()

	if opts.appFile == "" {
		exitWithUsage("--app flag is required")
	}
	if opts.repoPath == "" {
		exitWithUsage("--repo flag is required")
	}

	return opts
}

func buildRenderRequest(app *internal.Application, opts *options) *internal.RenderRequest {
	req := &internal.RenderRequest{
		Application: app,
		RepoPath:    opts.repoPath,
		KubeVersion: opts.kubeVersion,
	}

	if opts.helmSkipCrds || opts.helmSkipTests {
		req.HelmOptions = &internal.HelmOptions{
			SkipCrds:  opts.helmSkipCrds,
			SkipTests: opts.helmSkipTests,
		}
	}

	if opts.kustomizeBuild != "" {
		req.KustomizeOptions = &internal.KustomizeOptions{
			BuildOptions: opts.kustomizeBuild,
		}
	}

	return req
}

func exitOnError(err error, context string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error %s: %v\n", context, err)
		os.Exit(1)
	}
}

func exitWithUsage(msg string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	flag.Usage()
	os.Exit(1)
}

func loadApplication(filePath string) (*internal.Application, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read application file: %w", err)
	}

	var appYaml struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Metadata   struct {
			Name string `yaml:"name"`
		} `yaml:"metadata"`
		Spec struct {
			Source      *internal.ApplicationSource     `yaml:"source"`
			Destination internal.ApplicationDestination `yaml:"destination"`
		} `yaml:"spec"`
	}

	if err := yaml.Unmarshal(data, &appYaml); err != nil {
		return nil, fmt.Errorf("failed to unmarshal application YAML: %w", err)
	}

	if appYaml.Kind != "Application" {
		return nil, fmt.Errorf("expected kind 'Application', got '%s'", appYaml.Kind)
	}

	app := &internal.Application{
		Name: appYaml.Metadata.Name,
		Spec: internal.ApplicationSpec{
			Source:      appYaml.Spec.Source,
			Destination: appYaml.Spec.Destination,
		},
	}

	return app, nil
}
