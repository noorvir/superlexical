package core

import (
	"errors"

	"github.com/goccy/go-yaml"
)

// Page is the content of one Markdown file: its front matter fields in source
// order and its Markdown body.
type Page struct {
	Fields []Field
	Body   string
}

// Field is one top-level front matter field.
type Field struct {
	Name  string
	Value any
}

// ParsePage splits a Markdown file into front matter fields and body. It does
// not validate the fields against a schema.
func ParsePage(source []byte) (Page, error) {
	metadata, issue := parseFrontmatter(source)
	if issue != nil {
		return Page{}, errors.New(issue.Message)
	}

	var values yaml.MapSlice
	if err := yaml.UnmarshalWithOptions(metadata.json, &values, yaml.UseOrderedMap()); err != nil {
		return Page{}, err
	}

	page := Page{Body: string(metadata.body)}
	for _, item := range values {
		name, ok := item.Key.(string)
		if !ok {
			continue
		}

		page.Fields = append(page.Fields, Field{Name: name, Value: item.Value})
	}

	return page, nil
}
