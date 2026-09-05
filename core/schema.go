package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
)

func validateRecords(documents []document, schema *jsonschema.Schema, ids map[string]string) []Diagnostic {
	var diagnostics []Diagnostic
	for _, doc := range documents {
		if doc.metadata.json == nil {
			continue
		}
		value, err := jsonschema.UnmarshalJSON(bytes.NewReader(doc.metadata.json))
		if err != nil {
			message := "cannot read front matter as JSON: " + err.Error()
			diagnostics = append(diagnostics, Diagnostic{doc.path, 2, "frontmatter", message})
			continue
		}
		if err := schema.Validate(value); err != nil {
			var failure *jsonschema.ValidationError
			if !errors.As(err, &failure) {
				message := "cannot validate this record: " + err.Error()
				diagnostics = append(diagnostics, Diagnostic{doc.path, 2, "frontmatter", message})
				continue
			}
			diagnostics = append(diagnostics, schemaDiagnostics(doc, failure)...)
		}

		var record struct {
			ID string `json:"@id"`
		}
		if err := json.Unmarshal(doc.metadata.json, &record); err != nil || record.ID == "" {
			continue
		}
		previous, exists := ids[record.ID]
		if !exists {
			ids[record.ID] = doc.path
			continue
		}
		line := doc.metadata.lineAt([]string{"@id"})
		message := fmt.Sprintf("duplicate ID %q; first declared in %s", record.ID, previous)
		diagnostics = append(diagnostics, Diagnostic{doc.path, line, "@id", message})
	}
	return diagnostics
}

func schemaDiagnostics(doc document, failure *jsonschema.ValidationError) []Diagnostic {
	var diagnostics []Diagnostic
	if len(failure.Causes) != 0 {
		for _, cause := range failure.Causes {
			diagnostics = append(diagnostics, schemaDiagnostics(doc, cause)...)
		}
		return diagnostics
	}
	location := failure.InstanceLocation
	switch detail := failure.ErrorKind.(type) {
	case *kind.Required:
		for _, property := range detail.Missing {
			path := append(slices.Clone(location), property)
			field := strings.Join(path, ".")
			line := doc.metadata.lineAt(location)
			diagnostics = append(diagnostics, Diagnostic{doc.path, line, field, "required field is missing; add it"})
		}
	case *kind.AdditionalProperties:
		for _, property := range detail.Properties {
			path := append(slices.Clone(location), property)
			field := strings.Join(path, ".")
			line := doc.metadata.lineAt(path)
			message := "field is not allowed by the schema; remove it"
			diagnostics = append(diagnostics, Diagnostic{doc.path, line, field, message})
		}
	default:
		field := strings.Join(location, ".")
		line := doc.metadata.lineAt(location)
		message := schemaMessage(failure)
		diagnostics = append(diagnostics, Diagnostic{doc.path, line, field, message})
	}
	return diagnostics
}

func schemaMessage(failure *jsonschema.ValidationError) string {
	switch detail := failure.ErrorKind.(type) {
	case *kind.Type:
		return fmt.Sprintf("expected %s; found %s", strings.Join(detail.Want, " or "), detail.Got)
	case *kind.MinLength:
		return fmt.Sprintf("value must contain at least %d characters; found %d", detail.Want, detail.Got)
	case *kind.Pattern:
		switch detail.Want {
		case `\S`:
			return "value must contain text; it cannot be empty or only whitespace"
		case `^[^\s]+$`:
			return "value must not be empty or contain whitespace"
		}
	}
	output := failure.BasicOutput()
	return output.Error.String()
}

func compileSchema(schema []byte) (*jsonschema.Schema, error) {
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
	return compiled, nil
}
