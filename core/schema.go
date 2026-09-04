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

func validateRecords(documents []document, schema *jsonschema.Schema) []Diagnostic {
	var diagnostics []Diagnostic
	ids := make(map[string]string)
	for _, doc := range documents {
		if doc.metadata.json == nil {
			continue
		}
		value, err := jsonschema.UnmarshalJSON(bytes.NewReader(doc.metadata.json))
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{doc.path, 2, "frontmatter", "cannot read front matter as JSON: " + err.Error()})
			continue
		}
		if err := schema.Validate(value); err != nil {
			var failure *jsonschema.ValidationError
			if !errors.As(err, &failure) {
				diagnostics = append(diagnostics, Diagnostic{doc.path, 2, "frontmatter", "cannot validate this record: " + err.Error()})
				continue
			}
			diagnostics = append(diagnostics, schemaDiagnostics(doc, failure)...)
		}

		var record struct {
			ID string `json:"@id"`
		}
		if err := json.Unmarshal(doc.metadata.json, &record); err == nil && record.ID != "" {
			if previous, exists := ids[record.ID]; exists {
				line := doc.metadata.lineAt([]string{"@id"})
				diagnostics = append(diagnostics, Diagnostic{doc.path, line, "@id", fmt.Sprintf("duplicate ID %q; first declared in %s", record.ID, previous)})
			} else {
				ids[record.ID] = doc.path
			}
		}
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
			field := strings.Join(append(slices.Clone(location), property), ".")
			diagnostics = append(diagnostics, Diagnostic{doc.path, doc.metadata.lineAt(location), field, "required field is missing; add it"})
		}
	case *kind.AdditionalProperties:
		for _, property := range detail.Properties {
			path := append(slices.Clone(location), property)
			diagnostics = append(diagnostics, Diagnostic{doc.path, doc.metadata.lineAt(path), strings.Join(path, "."), "field is not allowed by the schema; remove it"})
		}
	default:
		diagnostics = append(diagnostics, Diagnostic{doc.path, doc.metadata.lineAt(location), strings.Join(location, "."), schemaMessage(failure)})
	}
	return diagnostics
}

func schemaMessage(failure *jsonschema.ValidationError) string {
	switch detail := failure.ErrorKind.(type) {
	case *kind.Type:
		return fmt.Sprintf("expected %s; found %s", strings.Join(detail.Want, " or "), detail.Got)
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
