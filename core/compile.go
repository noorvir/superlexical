// Package core validates Markdown workspaces without modifying their files.
package core

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Compile validates every .md file under root against a JSON Schema, then checks
// IDs and Markdown links. Document errors are diagnostics; unusable schemas and root directories
// return an error. Callers must keep the workspace stable during the call.
func Compile(root string, schema []byte) ([]Diagnostic, error) {
	definition, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema))
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)
	compiler.AssertFormat()
	if err := compiler.AddResource("urn:superlexical:schema", definition); err != nil {
		return nil, fmt.Errorf("load schema: %w", err)
	}
	compiled, err := compiler.Compile("urn:superlexical:schema")
	if err != nil {
		return nil, fmt.Errorf("compile schema: %w", err)
	}

	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root is not a directory: %s", root)
	}

	documents, diagnostics, err := readDocuments(root)
	if err != nil {
		return nil, fmt.Errorf("read workspace: %w", err)
	}
	recordIssues := validateRecords(documents, compiled)
	diagnostics = append(diagnostics, recordIssues...)

	byPath := make(map[string]document, len(documents))
	for _, doc := range documents {
		byPath[doc.path] = doc
	}

	for _, doc := range documents {
		for _, link := range doc.links {
			message, err := checkLink(root, doc.path, link.destination, byPath)
			if err != nil {
				message = fmt.Sprintf("cannot check link %q: %s", link.destination, fileError(err))
			}
			if message != "" {
				diagnostics = append(diagnostics, Diagnostic{doc.path, link.line, "link", message})
			}
		}
	}
	sort.Slice(diagnostics, func(i, j int) bool {
		a, b := diagnostics[i], diagnostics[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Field != b.Field {
			return a.Field < b.Field
		}
		return a.Message < b.Message
	})
	return diagnostics, nil
}

// DirectorySchema requires the minimal Schema.org Person or Organization record.
//
//go:embed schemas/directory.json
var DirectorySchema []byte

type Diagnostic struct {
	Path    string
	Line    int
	Field   string
	Message string
}

func (d Diagnostic) String() string {
	if d.Field == "" {
		return fmt.Sprintf("%s:%d: %s", d.Path, d.Line, d.Message)
	}
	return fmt.Sprintf("%s:%d: %s: %s", d.Path, d.Line, d.Field, d.Message)
}

func lineNumber(source []byte, offset int) int {
	return bytes.Count(source[:offset], []byte{'\n'}) + 1
}
