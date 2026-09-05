package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"superlexical/extensions/directory"
)

// CompileWorkspace reads .superlexical/extensions.json and validates records
// for enabled built-in extensions. Configuration and document issues are
// diagnostics; an unreadable configuration or workspace returns an error.
// Callers must keep the workspace stable during the call.
func CompileWorkspace(root string) ([]Diagnostic, error) {
	root, err := workspaceRoot(root)
	if err != nil {
		return nil, err
	}
	source, err := os.ReadFile(filepath.Join(root, extensionConfigPath))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", extensionConfigPath, err)
	}

	extensions, err := decodeExtensions(source)
	if err != nil {
		issue := extensionDiagnostic("extensions", err.Error())
		return []Diagnostic{issue}, nil
	}
	scopes, diagnostics, err := resolveExtensions(extensions)
	if err != nil {
		return nil, err
	}
	return compile(root, scopes, diagnostics), nil
}

const extensionConfigPath = ".superlexical/extensions.json"

type extension struct {
	Name         string            `json:"name"`
	Dependencies []json.RawMessage `json:"dependencies"`
}

func decodeExtensions(source []byte) ([]extension, error) {
	var config struct {
		Extensions []extension `json:"extensions"`
	}
	decoder := json.NewDecoder(bytes.NewReader(source))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		return nil, fmt.Errorf("configuration must contain one JSON object")
	}
	if config.Extensions == nil {
		return nil, fmt.Errorf("extensions must be an array; use [] to enable none")
	}
	return config.Extensions, nil
}

func resolveExtensions(extensions []extension) ([]scope, []Diagnostic, error) {
	var scopes []scope
	var diagnostics []Diagnostic
	seen := make(map[string]bool)
	for i, extension := range extensions {
		field := fmt.Sprintf("extensions[%d]", i)
		if extension.Dependencies == nil {
			issue := extensionDiagnostic(field+".dependencies", "dependencies must be an empty array: []")
			diagnostics = append(diagnostics, issue)
		} else if len(extension.Dependencies) != 0 {
			issue := extensionDiagnostic(field+".dependencies", "extension dependencies are not supported; use []")
			diagnostics = append(diagnostics, issue)
		}
		if extension.Name != "directory" {
			message := fmt.Sprintf("unsupported extension %q; expected directory", extension.Name)
			issue := extensionDiagnostic(field+".name", message)
			diagnostics = append(diagnostics, issue)
			continue
		}
		if seen[extension.Name] {
			message := "extension is listed more than once; remove the duplicate"
			issue := extensionDiagnostic(field+".name", message)
			diagnostics = append(diagnostics, issue)
			continue
		}
		seen[extension.Name] = true
		compiled, err := compileSchema(directory.Schema)
		if err != nil {
			return nil, nil, fmt.Errorf("Directory schema: %w", err)
		}
		scopes = append(scopes, scope{path: "directory", schema: compiled})
	}
	return scopes, diagnostics, nil
}

type scope struct {
	path   string
	schema *jsonschema.Schema
}

func extensionDiagnostic(field, message string) Diagnostic {
	return Diagnostic{
		Path:    extensionConfigPath,
		Line:    1,
		Field:   field,
		Message: message,
	}
}
