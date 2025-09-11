package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	renderer "github.com/lorenzbischof/local-argocd-renderer"
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

	r := renderer.NewRenderer()
	result, err := r.ExecuteCommand(context.Background(), req, opts.verbose)
	exitOnError(err, "executing command")

	// Print the output to stdout
	fmt.Print(result.Output)

	// Print any error output to stderr
	if result.Error != "" {
		fmt.Fprint(os.Stderr, result.Error)
	}
}

func parseFlags() *options {
	opts := &options{}
	flag.StringVar(&opts.appFile, "app", "", "Path to ArgoCD Application YAML file (use '-' for stdin)")
	flag.StringVar(&opts.repoPath, "repo", "", "Path to local repository containing manifests (required)")
	flag.StringVar(&opts.kubeVersion, "kube-version", "", "Kubernetes version to use for rendering (optional)")
	flag.BoolVar(&opts.helmSkipCrds, "helm-skip-crds", false, "Skip CRDs when rendering Helm charts")
	flag.BoolVar(&opts.helmSkipTests, "helm-skip-tests", false, "Skip tests when rendering Helm charts")
	flag.StringVar(&opts.kustomizeBuild, "kustomize-build-options", "", "Additional kustomize build options")
	flag.BoolVar(&opts.verbose, "verbose", false, "Verbose output showing commands")
	flag.Parse()

	// If no --app flag specified or it's "-", use stdin
	if opts.appFile == "" || opts.appFile == "-" {
		// Check if stdin has data
		stat, err := os.Stdin.Stat()
		if err != nil || (stat.Mode()&os.ModeCharDevice) != 0 {
			exitWithUsage("--app flag is required or provide application YAML via stdin")
		}
		opts.appFile = "-"
	}

	if opts.repoPath == "" {
		exitWithUsage("--repo flag is required")
	}

	return opts
}

func buildRenderRequest(app *renderer.Application, opts *options) *renderer.RenderRequest {
	req := &renderer.RenderRequest{
		Application: app,
		RepoPath:    opts.repoPath,
		KubeVersion: opts.kubeVersion,
	}

	if opts.helmSkipCrds || opts.helmSkipTests {
		req.HelmOptions = &renderer.HelmOptions{
			SkipCrds:  opts.helmSkipCrds,
			SkipTests: opts.helmSkipTests,
		}
	}

	if opts.kustomizeBuild != "" {
		req.KustomizeOptions = &renderer.KustomizeOptions{
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

func loadApplication(filePath string) (*renderer.Application, error) {
	var data []byte
	var err error

	if filePath == "-" {
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("failed to read from stdin: %w", err)
		}
	} else {
		data, err = os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read application file: %w", err)
		}
	}

	var appYaml struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Metadata   struct {
			Name string `yaml:"name"`
		} `yaml:"metadata"`
		Spec struct {
			Source      *renderer.ApplicationSource     `yaml:"source"`
			Destination renderer.ApplicationDestination `yaml:"destination"`
		} `yaml:"spec"`
	}

	if err := yaml.Unmarshal(data, &appYaml); err != nil {
		return nil, fmt.Errorf("failed to unmarshal application YAML: %w", err)
	}

	if appYaml.Kind != "Application" {
		return nil, fmt.Errorf("expected kind 'Application', got '%s'", appYaml.Kind)
	}

	app := &renderer.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name: appYaml.Metadata.Name,
		},
		Spec: renderer.ApplicationSpec{
			Source:      appYaml.Spec.Source,
			Destination: appYaml.Spec.Destination,
		},
	}

	return app, nil
}
