package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ListIndex is the file in a list folder that names the list and holds the
// schema of its entries.
const ListIndex = "index.md"

const listIndexSchema = `{
	"type": "object",
	"required": ["name"],
	"properties": {
		"name": {"type": "string", "pattern": "\\S"},
		"description": {"type": "string"},
		"schema": {"type": "object"}
	}
}`

// resolveLists returns two scopes for each folder in lists/: one for the index
// file and one that applies the schema in the index file to every entry.
func resolveLists(root string) ([]scope, []Diagnostic, error) {
	index, err := compileSchema([]byte(listIndexSchema))
	if err != nil {
		return nil, nil, fmt.Errorf("List index schema: %w", err)
	}

	folders, err := os.ReadDir(filepath.Join(root, "lists"))
	if err != nil {
		issue := Diagnostic{"lists", 1, "file", "cannot read record folder: " + fileError(err)}
		return nil, []Diagnostic{issue}, nil
	}

	var scopes []scope
	var diagnostics []Diagnostic

	for _, folder := range folders {
		if !folder.IsDir() {
			continue
		}

		folderPath := path.Join("lists", folder.Name())
		indexPath := path.Join(folderPath, ListIndex)
		isIndex := func(candidate string) bool { return candidate == indexPath }
		isEntry := func(candidate string) bool { return candidate != indexPath }

		source, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(indexPath)))
		if err != nil {
			message := "list has no readable index.md; add one with a name"
			diagnostics = append(diagnostics, Diagnostic{folderPath, 1, "file", message})
			continue
		}

		scopes = append(scopes, scope{path: folderPath, schema: index, match: isIndex})

		entries, err := entrySchema(source)
		if err != nil {
			message := "cannot use this schema for the entries: " + err.Error()
			diagnostics = append(diagnostics, Diagnostic{indexPath, 2, "schema", message})
			continue
		}

		scopes = append(scopes, scope{path: folderPath, schema: entries, match: isEntry})
	}

	return scopes, diagnostics, nil
}

// entrySchema compiles the schema field of a list index. A list without a
// schema accepts any front matter.
func entrySchema(index []byte) (*jsonschema.Schema, error) {
	metadata, _ := parseFrontmatter(index)

	var fields struct {
		Schema json.RawMessage `json:"schema"`
	}
	if metadata.json != nil {
		if err := json.Unmarshal(metadata.json, &fields); err != nil {
			return nil, err
		}
	}

	if fields.Schema == nil {
		fields.Schema = json.RawMessage(`{"type":"object"}`)
	}

	return compileSchema(fields.Schema)
}
