package core

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type markdownLink struct {
	destination string
	line        int
}

func parseMarkdown(source []byte, startLine int) ([]markdownLink, map[string]bool) {
	markdown := goldmark.New(goldmark.WithParserOptions(parser.WithAutoHeadingID()))
	tree := markdown.Parser().Parse(text.NewReader(source))
	var links []markdownLink
	headings := make(map[string]bool)
	ast.Walk(tree, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		var destination []byte
		switch node := node.(type) {
		case *ast.Link:
			destination = node.Destination
		case *ast.Image:
			destination = node.Destination
		case *ast.AutoLink:
			destination = node.URL(source)
		case *ast.LinkReferenceDefinition:
			destination = node.Destination
		case *ast.Heading:
			value, _ := node.AttributeString("id")
			if id, ok := value.([]byte); ok {
				headings[string(id)] = true
			}
			return ast.WalkContinue, nil
		default:
			return ast.WalkContinue, nil
		}
		destination = util.UnescapePunctuations(destination)
		destination = util.ResolveNumericReferences(destination)
		destination = util.ResolveEntityNames(destination)
		offset := max(node.Pos(), 0)
		line := startLine + lineNumber(source, offset) - 1
		links = append(links, markdownLink{string(destination), line})
		return ast.WalkContinue, nil
	})
	return links, headings
}

func checkLinks(root string, documents []document) []Diagnostic {
	var diagnostics []Diagnostic
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
			if message == "" {
				continue
			}
			diagnostics = append(diagnostics, Diagnostic{doc.path, link.line, "link", message})
		}
	}
	return diagnostics
}

func checkLink(root, source, destination string, documents map[string]document) (string, error) {
	target, err := url.Parse(destination)
	if err != nil {
		return fmt.Sprintf("invalid URL %q: %s", destination, err), nil
	}
	if target.Scheme != "" || target.Host != "" {
		http := strings.EqualFold(target.Scheme, "http") || strings.EqualFold(target.Scheme, "https")
		if http && target.Hostname() == "" {
			return fmt.Sprintf("URL has no host: %q", destination), nil
		}
		if strings.EqualFold(target.Scheme, "file") {
			return "use a workspace-relative link instead of a file URL", nil
		}
		return "", nil
	}
	if strings.ContainsAny(target.Path, "\\\x00") {
		return fmt.Sprintf("invalid local path: %q", destination), nil
	}

	resolved := linkPath(source, target.Path)
	if !filepath.IsLocal(filepath.FromSlash(resolved)) {
		return fmt.Sprintf("link leaves the workspace: %q", destination), nil
	}
	filename := filepath.Join(root, filepath.FromSlash(resolved))
	canonical, err := filepath.EvalSymlinks(filename)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR) {
		return fmt.Sprintf("link target does not exist: %q", destination), nil
	}
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, canonical)
	if err != nil {
		return "", err
	}
	if !filepath.IsLocal(relative) {
		return fmt.Sprintf("link leaves the workspace: %q", destination), nil
	}
	info, err := os.Stat(canonical)
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return fmt.Sprintf("link target is not a regular file: %q", destination), nil
	}
	if !strings.EqualFold(path.Ext(resolved), ".md") {
		return "", nil
	}
	doc, exists := documents[filepath.ToSlash(relative)]
	if !exists {
		return fmt.Sprintf("Markdown target was not compiled: %q", destination), nil
	}
	if target.Fragment != "" && !doc.headings[target.Fragment] {
		return fmt.Sprintf("heading does not exist: %q", destination), nil
	}
	return "", nil
}

func linkPath(source, target string) string {
	if target == "" {
		return source
	}
	if strings.HasPrefix(target, "/") {
		return path.Clean(strings.TrimPrefix(target, "/"))
	}
	return path.Join(path.Dir(source), target)
}
