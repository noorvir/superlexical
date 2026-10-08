package core

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func readDocuments(root, folder string) ([]document, []Diagnostic, error) {
	directory := filepath.Join(root, filepath.FromSlash(folder))
	info, err := os.Lstat(directory)
	if err != nil {
		return nil, nil, err
	}

	if !info.IsDir() {
		return nil, nil, fmt.Errorf("record folder must be a directory, not a file or symbolic link")
	}

	var documents []document
	var diagnostics []Diagnostic

	err = filepath.WalkDir(directory, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if filename == root {
				return walkErr
			}

			path, err := filepath.Rel(root, filename)
			if err != nil {
				return err
			}

			message := "cannot read this path: " + fileError(walkErr)
			path = filepath.ToSlash(path)
			diagnostics = append(diagnostics, Diagnostic{path, 1, "file", message})
			return nil
		}

		if entry.IsDir() {
			if entry.Name() == ".git" || entry.Name() == ".superlexical" {
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
			message := "Markdown must be a regular file, not a symbolic link"
			diagnostics = append(diagnostics, Diagnostic{path, 1, "", message})
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
