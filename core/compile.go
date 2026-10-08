// Package core validates Markdown workspaces without modifying their files.
package core

import (
	"bytes"
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// Compile validates every .md file under root against a JSON Schema, then checks
// IDs and Markdown links. Document errors are diagnostics; unusable schemas and root directories
// return an error. Callers must keep the workspace stable during the call.
func Compile(root string, schema []byte) ([]Diagnostic, error) {
	compiled, err := compileSchema(schema)
	if err != nil {
		return nil, err
	}

	root, err = workspaceRoot(root)
	if err != nil {
		return nil, err
	}

	return compile(root, []scope{{path: ".", schema: compiled}}, nil), nil
}

func workspaceRoot(root string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}

	if !info.IsDir() {
		return "", fmt.Errorf("root is not a directory: %s", root)
	}

	return root, nil
}

func compile(root string, scopes []scope, diagnostics []Diagnostic) []Diagnostic {
	var documents []document
	ids := make(map[string]string)

	for _, scope := range scopes {
		batch, issues, err := readDocuments(root, scope.path)
		diagnostics = append(diagnostics, issues...)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{scope.path, 1, "file", "cannot read record folder: " + fileError(err)})
		}

		recordIssues := validateRecords(batch, scope.schema, ids)
		diagnostics = append(diagnostics, recordIssues...)
		documents = append(documents, batch...)
	}

	linkIssues := checkLinks(root, documents)
	diagnostics = append(diagnostics, linkIssues...)

	slices.SortFunc(diagnostics, func(a, b Diagnostic) int {
		return cmp.Or(
			cmp.Compare(a.Path, b.Path),
			cmp.Compare(a.Line, b.Line),
			cmp.Compare(a.Field, b.Field),
			cmp.Compare(a.Message, b.Message),
		)
	})

	return diagnostics
}

// Diagnostic identifies a validation issue in a workspace-relative source file.
// Line is one-based; Field is a display label, not a machine-readable path.
type Diagnostic struct {
	Path    string
	Line    int
	Field   string
	Message string
}

// String formats the diagnostic for command-line output.
func (d Diagnostic) String() string {
	if d.Field == "" {
		return fmt.Sprintf("%s:%d: %s", d.Path, d.Line, d.Message)
	}

	return fmt.Sprintf("%s:%d: %s: %s", d.Path, d.Line, d.Field, d.Message)
}

func lineNumber(source []byte, offset int) int {
	return bytes.Count(source[:offset], []byte{'\n'}) + 1
}
