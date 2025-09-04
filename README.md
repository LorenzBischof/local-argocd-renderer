# Local Argo CD Renderer

> ⚠️ **DISCLAIMER**: This project was entirely "vibe coded" based on patterns observed in the Argo CD codebase. There are no guarantees about functionality, correctness, or compatibility. I make no promises about maintaining this codebase long-term. Use at your own risk and consider it experimental. If you need production-ready local rendering, consider contributing to the official Argo CD project instead.

A standalone tool that renders Argo CD applications locally without requiring a running Argo CD server. This addresses the common need for debugging and studying potential Kubernetes manifests during development.

**Solves**: The problem described in [argoproj/argo-cd#11722](https://github.com/argoproj/argo-cd/issues/11722) where developers need to render Argo CD applications locally for testing and debugging purposes.

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
- **🎛️ Flexible Configuration**: Support for Kubernetes versions, build options, and more

## Installation

### Pre-built Binary

```bash
# Download from releases or build locally
go build -o local-argocd-renderer .
```

### As a Go Module

```bash
go get github.com/lorenzbischof/local-argocd-renderer
```

## Quick Start

### 1. Create an Application YAML

Save your Argo CD Application configuration to a file:

```yaml
# app.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
spec:
  source:
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    path: helm-guestbook
    helm:
      values: |
        service:
          type: LoadBalancer
  destination:
    server: https://kubernetes.default.svc
    namespace: default
```

### 2. Clone the Repository Locally

```bash
git clone https://github.com/argoproj/argocd-example-apps.git
```

### 3. Render the Manifests

```bash
./local-argocd-renderer --app app.yaml --repo ./argocd-example-apps
```

This outputs the rendered Kubernetes manifests that Argo CD would apply.

## Usage

### CLI Options

```bash
# Basic usage
./local-argocd-renderer --app application.yaml --repo ./my-repo

# With Kubernetes version and Helm options
./local-argocd-renderer \
  --app application.yaml \
  --repo ./my-repo \
  --kube-version 1.28.0 \
  --helm-skip-crds \
  --verbose

# With Kustomize build options
./local-argocd-renderer \
  --app kustomize-app.yaml \
  --repo ./my-kustomize-repo \
  --kustomize-build-options "--enable-alpha-plugins"
```

### CLI Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--app` | ✅ | Path to Argo CD Application YAML file |
| `--repo` | ✅ | Path to local repository containing source code |
| `--kube-version` | ❌ | Kubernetes version for API compatibility |
| `--helm-skip-crds` | ❌ | Skip Custom Resource Definitions in Helm charts |
| `--helm-skip-tests` | ❌ | Skip test manifests in Helm charts |
| `--kustomize-build-options` | ❌ | Additional flags passed to `kustomize build` |
| `--verbose` | ❌ | Show detailed command execution |

### Go Library Usage

```go
package main

import (
    "context"
    "fmt"
    
    "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
    "github.com/lorenzbischof/local-argocd-renderer/internal"
)

func main() {
    // Define your Application
    app := &v1alpha1.Application{
        APIVersion: "argoproj.io/v1alpha1",
        Kind:       "Application",
        Spec: v1alpha1.ApplicationSpec{
            Source: &v1alpha1.ApplicationSource{
                RepoURL: "https://github.com/argoproj/argocd-example-apps.git",
                Path:    "helm-guestbook",
            },
            Destination: v1alpha1.ApplicationDestination{
                Server:    "https://kubernetes.default.svc",
                Namespace: "default",
            },
        },
    }
    
    // Create render request
    req := &internal.RenderRequest{
        Application: app,
        RepoPath:    "./argocd-example-apps",
        KubeVersion: "1.28.0",
        HelmOptions: &internal.HelmOptions{
            SkipCrds:  false,
            SkipTests: true,
        },
    }
    
    // Execute rendering
    renderer := internal.NewRenderer()
    err := renderer.ExecuteCommand(context.Background(), req, false)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
    }
}
```

## Examples

The `examples/` directory contains sample applications for testing different source types.

### Helm Application

```bash
# Render the Helm example
./local-argocd-renderer --app examples/helm/application.yaml --repo examples/helm

# With custom values and options
./local-argocd-renderer \
  --app examples/helm/application.yaml \
  --repo examples/helm \
  --helm-skip-tests \
  --verbose
```

Example `examples/helm/application.yaml`:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: helm-guestbook
spec:
  source:
    path: chart
    helm:
      values: |
        service:
          type: LoadBalancer
        replicaCount: 2
  destination:
    server: https://kubernetes.default.svc
    namespace: default
```

### Kustomize Application

```bash
# Render the Kustomize example
./local-argocd-renderer --app examples/kustomize/application.yaml --repo examples/kustomize

# With production overlay
./local-argocd-renderer \
  --app examples/kustomize/application.yaml \
  --repo examples/kustomize \
  --kustomize-build-options "--enable-helm"
```

Example `examples/kustomize/application.yaml`:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: kustomize-guestbook
spec:
  source:
    path: manifests/overlays/production
    kustomize:
      images:
      - gcr.io/heptio-images/ks-guestbook-demo:0.2
  destination:
    server: https://kubernetes.default.svc
    namespace: production
```

### Directory (Plain YAML) Application

```bash
# Render plain YAML manifests
./local-argocd-renderer --app examples/directory/application.yaml --repo examples/directory
```

Example `examples/directory/application.yaml`:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: directory-app
spec:
  source:
    path: manifests
    directory:
      recurse: true
      include: "*.yaml"
  destination:
    server: https://kubernetes.default.svc
    namespace: default
```

## Common Use Cases

### Debugging Helm Values

When your Helm application isn't rendering as expected:

```bash
# Test different value combinations
./local-argocd-renderer --app my-app.yaml --repo ./my-chart --verbose

# Skip CRDs that might cause issues
./local-argocd-renderer --app my-app.yaml --repo ./my-chart --helm-skip-crds
```

### Testing Kustomize Overlays

Validate your Kustomize patches before deployment:

```bash
# Check production overlay
./local-argocd-renderer --app prod-app.yaml --repo ./my-kustomize-app

# Test with different build options
./local-argocd-renderer \
  --app prod-app.yaml \
  --repo ./my-kustomize-app \
  --kustomize-build-options "--load-restrictor=LoadRestrictionsNone"
```

### CI/CD Integration

Use in your pipeline for validation:

```yaml
# .github/workflows/validate-apps.yml
- name: Validate Argo Applications
  run: |
    for app in apps/*.yaml; do
      echo "Validating $app..."
      ./local-argocd-renderer --app "$app" --repo ./charts > /dev/null
    done
```

## Comparison with Argo CD CLI

| Feature | `argocd app manifests` | `local-argocd-renderer` |
|---------|------------------------|-------------------------|
| Server Required | ✅ Yes | ❌ No |
| Offline Usage | ❌ No | ✅ Yes |
| Local Development | ❌ Limited | ✅ Full |
| CI/CD Friendly | ❌ No | ✅ Yes |
| Setup Complexity | 🟡 Medium | ✅ Simple |

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│ Application     │    │ Local Repository │    │ Rendered        │
│ YAML Definition │───▶│ Source Code      │───▶│ K8s Manifests   │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                              │
                              ▼
                       ┌─────────────────┐
                       │ Source Type     │
                       │ Detection &     │
                       │ Rendering       │
                       │ • Helm          │
                       │ • Kustomize     │
                       │ • Directory     │
                       └─────────────────┘
```

## Limitations

- **Local repositories only**: Remote Git repositories must be cloned first
- **No server-side plugins**: Custom Argo CD plugins are not supported
- **Simplified validation**: Some advanced Argo CD validation rules are not applied
- **No diff capabilities**: Only renders manifests, doesn't compare with cluster state

## Development

### Running Tests

```bash
go test ./...
```

### Building

```bash
go build -o local-argocd-renderer .
```

### Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## Related Projects

- [Argo CD](https://github.com/argoproj/argo-cd) - The upstream GitOps continuous delivery tool
- [argocd-example-apps](https://github.com/argoproj/argocd-example-apps) - Sample applications for testing

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.