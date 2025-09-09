package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

type helmRenderer struct{}

// NewHelmRenderer creates a new Helm renderer
func NewHelmRenderer() HelmRenderer {
	return &helmRenderer{}
}

func (hr *helmRenderer) Execute(ctx context.Context, renderCtx *RenderContext, opts *HelmOptions, verbose bool) error {
	if renderCtx.Source.Helm == nil {
		return fmt.Errorf("helm configuration not found in application source")
	}

	args, tmpFiles, err := hr.buildHelmArgs(renderCtx, opts)
	if err != nil {
		return err
	}

	// Clean up temporary files after command execution
	defer func() {
		for _, tmpFile := range tmpFiles {
			os.RemoveAll(tmpFile)
		}
	}()

	return hr.runHelmCommand(ctx, args, renderCtx.RepoPath, verbose)
}

func (hr *helmRenderer) buildHelmArgs(renderCtx *RenderContext, opts *HelmOptions) ([]string, []string, error) {
	args := []string{"template", hr.getReleaseName(renderCtx), hr.getChartPath(renderCtx)}
	var tmpFiles []string

	args = hr.addNamespace(args, renderCtx)
	args = hr.addKubeVersion(args, renderCtx)

	valueArgs, err := hr.addValueFiles(args, renderCtx)
	if err != nil {
		return nil, nil, err
	}
	args = valueArgs

	inlineArgs, inlineTmpFiles, err := hr.addInlineValues(args, renderCtx)
	if err != nil {
		return nil, nil, err
	}
	args = inlineArgs
	tmpFiles = append(tmpFiles, inlineTmpFiles...)

	args = hr.addParameters(args, renderCtx)

	fileArgs, err := hr.addFileParameters(args, renderCtx)
	if err != nil {
		return nil, nil, err
	}
	args = fileArgs

	return hr.addSkipOptions(args, renderCtx, opts), tmpFiles, nil
}

func (hr *helmRenderer) getReleaseName(renderCtx *RenderContext) string {
	if renderCtx.Source.Helm.ReleaseName != "" {
		return renderCtx.Source.Helm.ReleaseName
	}
	return renderCtx.AppName
}

func (hr *helmRenderer) getChartPath(renderCtx *RenderContext) string {
	if renderCtx.Source.Path == "" {
		return "."
	}
	return renderCtx.Source.Path
}

func (hr *helmRenderer) addNamespace(args []string, renderCtx *RenderContext) []string {
	namespace := renderCtx.Namespace
	if namespace != "" {
		args = append(args, "--namespace", namespace)
	}
	return args
}

func (hr *helmRenderer) addKubeVersion(args []string, renderCtx *RenderContext) []string {
	if renderCtx.KubeVersion != "" {
		args = append(args, "--kube-version", renderCtx.KubeVersion)
	}
	return args
}

func (hr *helmRenderer) addValueFiles(args []string, renderCtx *RenderContext) ([]string, error) {
	for _, valueFile := range renderCtx.Source.Helm.ValueFiles {
		resolvedPath := hr.resolveValueFilePath(renderCtx.Source.Path, renderCtx.RepoPath, valueFile)
		if _, err := os.Stat(resolvedPath); err != nil {
			if renderCtx.Source.Helm.IgnoreMissingValueFiles {
				continue
			}
			return nil, fmt.Errorf("error resolving helm value file %s: %w", valueFile, err)
		}
		args = append(args, "--values", resolvedPath)
	}
	return args, nil
}

func (hr *helmRenderer) addInlineValues(args []string, renderCtx *RenderContext) ([]string, []string, error) {
	if renderCtx.Source.Helm.ValuesIsEmpty() {
		return args, nil, nil
	}

	rand, err := uuid.NewRandom()
	if err != nil {
		return nil, nil, fmt.Errorf("error generating random filename for Helm values file: %w", err)
	}

	tmpFile := path.Join(os.TempDir(), rand.String())

	err = os.WriteFile(tmpFile, renderCtx.Source.Helm.ValuesYAML(), 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("error writing helm values file: %w", err)
	}

	return append(args, "--values", tmpFile), []string{tmpFile}, nil
}

func (hr *helmRenderer) addParameters(args []string, renderCtx *RenderContext) []string {
	for _, param := range renderCtx.Source.Helm.Parameters {
		flag := "--set"
		if param.ForceString {
			flag = "--set-string"
		}
		args = append(args, flag, fmt.Sprintf("%s=%s", param.Name, param.Value))
	}
	return args
}

func (hr *helmRenderer) addFileParameters(args []string, renderCtx *RenderContext) ([]string, error) {
	for _, param := range renderCtx.Source.Helm.FileParameters {
		resolvedPath := hr.resolveValueFilePath(renderCtx.Source.Path, renderCtx.RepoPath, param.Path)
		if _, err := os.Stat(resolvedPath); err != nil {
			return nil, fmt.Errorf("error resolving helm file parameter %s: %w", param.Path, err)
		}
		args = append(args, "--set-file", fmt.Sprintf("%s=%s", param.Name, resolvedPath))
	}
	return args, nil
}

func (hr *helmRenderer) addSkipOptions(args []string, renderCtx *RenderContext, opts *HelmOptions) []string {
	if renderCtx.Source.Helm.SkipCrds || (opts != nil && opts.SkipCrds) {
		args = append(args, "--skip-crds")
	}
	if opts != nil && opts.SkipTests {
		args = append(args, "--skip-tests")
	}

	if opts != nil && opts.IncludeCrds {
		args = hr.removeArg(args, "--skip-crds")
	}

	return args
}

func (hr *helmRenderer) resolveValueFilePath(sourcePath, repoPath, valueFile string) string {
	if filepath.IsAbs(valueFile) {
		return valueFile
	}
	if sourcePath != "" {
		return filepath.Join(repoPath, sourcePath, valueFile)
	}
	return filepath.Join(repoPath, valueFile)
}

func (hr *helmRenderer) removeArg(args []string, argToRemove string) []string {
	for i, arg := range args {
		if arg == argToRemove {
			return append(args[:i], args[i+1:]...)
		}
	}
	return args
}

func (hr *helmRenderer) runHelmCommand(ctx context.Context, args []string, workDir string, verbose bool) error {
	cmd := exec.CommandContext(ctx, "helm", args...)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if verbose {
		hr.printVerboseInfo(args, workDir)
	}

	return cmd.Run()
}

func (hr *helmRenderer) printVerboseInfo(args []string, workDir string) {
	fmt.Fprintf(os.Stderr, "Source Type: helm\n")
	fmt.Fprintf(os.Stderr, "Command: %s\n", strings.Join(append([]string{"helm"}, args...), " "))
	fmt.Fprintf(os.Stderr, "Working Directory: %s\n", workDir)
	fmt.Fprintf(os.Stderr, "---\n")
}
