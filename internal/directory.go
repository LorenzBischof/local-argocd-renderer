package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type directoryRenderer struct{}

// NewDirectoryRenderer creates a new directory renderer
func NewDirectoryRenderer() DirectoryRenderer {
	return &directoryRenderer{}
}

func (dr *directoryRenderer) Execute(ctx context.Context, renderCtx *RenderContext, verbose bool) error {
	searchPath := dr.getSearchPath(renderCtx)
	recurse := dr.shouldRecurse(renderCtx)

	if verbose {
		dr.printVerboseInfo(searchPath, recurse)
	}

	return dr.walkAndOutputFiles(ctx, searchPath, renderCtx.Source.Directory, recurse)
}

func (dr *directoryRenderer) getSearchPath(renderCtx *RenderContext) string {
	if renderCtx.Source.Path != "" {
		return filepath.Join(renderCtx.RepoPath, renderCtx.Source.Path)
	}
	return renderCtx.RepoPath
}

func (dr *directoryRenderer) shouldRecurse(renderCtx *RenderContext) bool {
	if renderCtx.Source.Directory != nil {
		return renderCtx.Source.Directory.Recurse
	}
	return true
}

func (dr *directoryRenderer) printVerboseInfo(searchPath string, recurse bool) {
	fmt.Fprintf(os.Stderr, "Source Type: directory\n")
	fmt.Fprintf(os.Stderr, "Search Path: %s\n", searchPath)
	fmt.Fprintf(os.Stderr, "Recursive: %t\n", recurse)
	fmt.Fprintf(os.Stderr, "---\n")
}

func (dr *directoryRenderer) walkAndOutputFiles(ctx context.Context, searchPath string, directory *ApplicationSourceDirectory, recurse bool) error {
	first := true

	return filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return dr.handleDirectory(path, searchPath, recurse)
		}

		if dr.shouldIncludeFile(path, searchPath, info, directory) {
			if !first {
				fmt.Println("---")
			}
			first = false
			return dr.outputFile(ctx, path)
		}

		return nil
	})
}

func (dr *directoryRenderer) handleDirectory(path, searchPath string, recurse bool) error {
	if !recurse && path != searchPath {
		return filepath.SkipDir
	}
	return nil
}

func (dr *directoryRenderer) shouldIncludeFile(path, searchPath string, info os.FileInfo, directory *ApplicationSourceDirectory) bool {
	ext := strings.ToLower(filepath.Ext(info.Name()))
	if !dr.isManifestFile(ext) {
		return false
	}

	if directory == nil {
		return true
	}

	relPath, err := filepath.Rel(searchPath, path)
	if err != nil {
		return false
	}

	return dr.matchesPattern(relPath, directory.Include, directory.Exclude)
}

func (dr *directoryRenderer) outputFile(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, "cat", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// isManifestFile checks if a file extension indicates a manifest file
func (dr *directoryRenderer) isManifestFile(ext string) bool {
	return ext == ".yaml" || ext == ".yml" || ext == ".json"
}

// matchesPattern checks if a file path matches the include/exclude patterns
func (dr *directoryRenderer) matchesPattern(path string, include string, exclude string) bool {
	// If include is specified, file must match include pattern
	if include != "" {
		matched, _ := filepath.Match(include, path)
		if !matched {
			return false
		}
	}

	// If exclude is specified, file must not match exclude pattern
	if exclude != "" {
		matched, _ := filepath.Match(exclude, path)
		if matched {
			return false
		}
	}

	return true
}
