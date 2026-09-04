package core

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

func parseFrontmatter(source []byte) (frontmatter, *Diagnostic) {
	source = bytes.TrimPrefix(source, []byte{0xef, 0xbb, 0xbf})
	metadata := frontmatter{body: source, bodyLine: 1}
	if !utf8.Valid(source) {
		return metadata, &Diagnostic{Line: 1, Field: "frontmatter", Message: "file is not valid UTF-8"}
	}
	lines := bytes.SplitAfter(source, []byte{'\n'})
	if strings.TrimRight(string(lines[0]), "\r\n") != "---" {
		return metadata, &Diagnostic{Line: 1, Field: "frontmatter", Message: "missing YAML front matter; start the file with a --- block"}
	}
	start := len(lines[0])
	end := start
	for _, line := range lines[1:] {
		if strings.TrimRight(string(line), "\r\n") == "---" {
			break
		}
		end += len(line)
	}
	if end == len(source) {
		return metadata, &Diagnostic{Line: 1, Field: "frontmatter", Message: "missing closing front matter delimiter; add --- after the YAML fields"}
	}
	bodyStart := end + len(lines[lineNumber(source, end)-1])
	metadata.body = source[bodyStart:]
	metadata.bodyLine = lineNumber(source, bodyStart)
	data := source[start:end]

	file, err := parser.ParseBytes(data, 0)
	if err == nil && (len(file.Docs) != 1 || file.Docs[0].Body == nil || file.Docs[0].Body.Type() != ast.MappingType) {
		return metadata, &Diagnostic{Line: 2, Field: "frontmatter", Message: "front matter must be one YAML mapping"}
	}
	if err == nil {
		metadata.syntax = file.Docs[0].Body
		decoder := yaml.NewDecoder(bytes.NewReader(nil), yaml.UseOrderedMap())
		var value yaml.MapSlice
		err = decoder.DecodeFromNode(metadata.syntax, &value)
		if err == nil {
			metadata.json, err = yaml.MarshalWithOptions(value, yaml.JSON())
		}
	}

	if err != nil {
		line := 2
		message := err.Error()
		var yamlError yaml.Error
		if errors.As(err, &yamlError) {
			line = yamlError.GetToken().Position.Line + 1
			message = yamlError.GetMessage()
		}
		metadata.json = nil
		return metadata, &Diagnostic{Line: line, Field: "frontmatter", Message: "invalid YAML: " + message + "; fix the syntax before checking these fields"}
	}
	return metadata, nil
}

type frontmatter struct {
	json     []byte
	body     []byte
	bodyLine int
	syntax   ast.Node
}

// lineAt follows schema path segments through the original YAML tree. Missing
// properties and alias contents point to the nearest enclosing source location.
func (m frontmatter) lineAt(path []string) int {
	node := m.syntax
	line := 2
	for _, segment := range path {
	unwrap:
		for {
			switch wrapper := node.(type) {
			case *ast.AnchorNode:
				node = wrapper.Value
			case *ast.TagNode:
				node = wrapper.Value
			default:
				break unwrap
			}
		}
		switch current := node.(type) {
		case ast.MapNode:
			entries := current.MapRange()
			found := false
			for entries.Next() {
				key, ok := entries.Key().(ast.ScalarNode)
				if ok && fmt.Sprint(key.GetValue()) == segment {
					line = entries.Key().GetToken().Position.Line + 1
					node = entries.Value()
					found = true
					break
				}
			}
			if !found {
				return line
			}
		case *ast.SequenceNode:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(current.Values) {
				return line
			}
			node = current.Values[index]
			line = node.GetToken().Position.Line + 1
		default:
			return line
		}
	}
	return line
}
