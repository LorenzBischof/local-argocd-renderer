package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	renderer "github.com/lorenzbischof/local-argocd-renderer"
	"sigs.k8s.io/yaml"
)

func main() {
	var applicationFile = flag.String("app", "", "Path to Application CRD YAML file (use '-' for stdin) (required)")
	flag.Parse()

	if *applicationFile == "" {
		fmt.Fprintf(os.Stderr, "Error: --application flag is required\n")
		fmt.Fprintf(os.Stderr, "Usage: %s --application <file> | --application -\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  %s --application app.yaml\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  cat app.yaml | %s --application -\n", os.Args[0])
		os.Exit(1)
	}

	ctx := context.Background()
	opts := renderer.TemplateOptions{
		ApplicationFile: *applicationFile,
		RepoRoot:        ".",
	}

	result, err := renderer.TemplateFromApplication(ctx, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Report any warnings
	for _, warning := range result.Warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", warning)
	}

	fmt.Printf("# Generated %d manifests\n", len(result.Objects))
	fmt.Println("---")

	// Parse and output manifests
	for i, object := range result.Objects {
		if i > 0 {
			fmt.Println("---")
		}

		yamlBytes, err := yaml.Marshal(object)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		fmt.Printf("%s", yamlBytes)
	}
}
