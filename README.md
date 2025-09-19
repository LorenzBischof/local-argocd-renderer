# Local Argo CD Renderer

A standalone tool that renders Argo CD applications locally without requiring a running Argo CD server. This addresses the common need for debugging and studying potential Kubernetes manifests during development.

**Solves**: The problem described in [argoproj/argo-cd#11722](https://github.com/argoproj/argo-cd/issues/11722) and [argoproj/argo-cd#11129](https://github.com/argoproj/argo-cd/issues/11129) where developers need to render Argo CD applications locally for testing and debugging purposes.

## Problem Statement

When working with large Argo CD installations with frequent chart changes, developers often need to:
- Debug potential manifests before deployment
- Study the rendered Kubernetes resources
- Validate applications locally without server access
- Test changes to Helm charts, Kustomize configurations, or plain manifests

The official Argo CD CLI requires a server connection (`argocd app manifests` fails with "Argo CD server address unspecified"), making local development and testing difficult.

## Features

- **🚀 No Server Required**: Render applications completely offline
- **🎯 Full Source Type Support**: 
  - Helm charts with values, parameters, and custom options
  - Kustomize applications with overlays and patches
  - Plain YAML/JSON manifest directories
- **🔧 CLI Tool**: Simple command-line interface matching Argo CD patterns
- **📚 Library API**: Go package for integration into other tools

## Usage

### CLI

```bash
# Build the CLI
go build ./cmd/local-argocd-renderer

# Run the CLI
./local-argocd-renderer --app examples/directory/app.yaml

# Or pipe from stdin
cat examples/directory/app.yaml | ./local-argocd-renderer --app -
```

## Library

```go
import (
    "context"
    "log"

    "github.com/lorenzbischof/local-argocd-renderer/pkg/renderer"
)

ctx := context.Background()
result, err := renderer.TemplateFromApplication(ctx, renderer.TemplateOptions{
    ApplicationFile: "my-app.yaml",
    RepoRoot:        ".",
})
if err != nil {
    log.Fatal(err)
}

// Process result.Objects
```

## Examples

The `examples/` directory contains sample applications.

## Comparison with Argo CD CLI

| Feature | `argocd app manifests` | `local-argocd-renderer` |
|---------|------------------------|-------------------------|
| Server Required | ❌ Yes | ✅ No |
| Offline Usage | ❌ No | ✅ Yes |
| Local Development | ❌ Limited | ✅ Full |
| CI/CD Friendly | ❌ No | ✅ Yes |
| Setup Complexity | 🟡 Medium | ✅ Simple |

## Limitations

- **Local repositories only**: Remote Git repositories must be cloned first
- **No server-side plugins**: Custom Argo CD plugins are not currently supported
- **Simplified validation**: Some advanced Argo CD validation rules are not applied
- **No diff capabilities**: Only renders manifests, doesn't compare with cluster state

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.
