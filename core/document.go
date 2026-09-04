package core

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func readDocuments(root string) ([]document, []Diagnostic, error) {
	var documents []document
	var diagnostics []Diagnostic
	err := filepath.WalkDir(root, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if filename == root {
				return walkErr
			}
			path, err := filepath.Rel(root, filename)
			if err != nil {
				return err
			}
			diagnostics = append(diagnostics, Diagnostic{filepath.ToSlash(path), 1, "file", "cannot read this path: " + fileError(walkErr)})
			return nil
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.EqualFold(filepath.Ext(filename), ".md") {
			return nil
		}
		path, err := filepath.Rel(root, filename)
		if err != nil {
			return err
		}
		path = filepath.ToSlash(path)
		if !entry.Type().IsRegular() {
			diagnostics = append(diagnostics, Diagnostic{path, 1, "", "Markdown must be a regular file, not a symbolic link"})
			return nil
		}
		source, err := os.ReadFile(filename)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{path, 1, "file", "cannot read file: " + fileError(err)})
			return nil
		}
		metadata, issue := parseFrontmatter(source)
		links, headings := parseMarkdown(metadata.body, metadata.bodyLine)
		documents = append(documents, document{path, metadata, links, headings})
		if issue != nil {
			issue.Path = path
			diagnostics = append(diagnostics, *issue)
		}
		return nil
	})
	return documents, diagnostics, err
}

type document struct {
	path     string
	metadata frontmatter
	links    []markdownLink
	headings map[string]bool
}

func fileError(err error) string {
	var pathError *fs.PathError
	if errors.As(err, &pathError) {
		return pathError.Err.Error()
	}
	return err.Error()
}
