package core_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"superlexical/core"
	"superlexical/extensions/directory"
)

func TestCompileDirectory(t *testing.T) {
	root := workspace(t, map[string]string{
		"people/alice.md": record("Person", "person:zt507zg4MOyd_2TGcpFB0", "Alice") + `# Alice

[Employer](../organizations/acme.md#team)
[Reference][acme]
[acme]: ../organizations/acme.md
![Logo](/assets/logo.png)
[Self](#alice)
[Website](https://example.com/path)
[Email](mailto:alice@example.com)
<https://example.com>

` + "`[example](missing.md)`\n\n```md\n[example](missing.md)\n```\n",
		"organizations/acme.md": record("Organization", "organization:DZO8p_bVF1ENdFAYHmxc7", "Acme") + "# Team\n",
		"assets/logo.png":       "image",
	})

	diagnostics, err := core.Compile(root, directory.Schema)
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("Compile = %v, %v", diagnostics, err)
	}

	before, err := os.ReadFile(filepath.Join(root, "people/alice.md"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = core.Compile(root, directory.Schema)
	if err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, "people/alice.md"))
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("compiler changed source: %v", err)
	}
}

func TestCompileMetadata(t *testing.T) {
	valid := record("Person", "person:zt507zg4MOyd_2TGcpFB0", "Alice")
	tests := []struct {
		name, source, field, message string
		line                         int
	}{
		{"missing metadata", "# Alice", "frontmatter", "missing YAML", 1},
		{"unclosed metadata", "---\nname: Alice\n", "frontmatter", "closing", 1},
		{"invalid yaml", "---\nname: [Alice\n---\n", "frontmatter", "", 2},
		{"duplicate yaml key", strings.Replace(valid, "name: Alice", "name: Alice\nname: Bob", 1), "frontmatter", "", 6},
		{"missing context", strings.Replace(valid, "'@context': https://schema.org\n", "", 1), "@context", "required", 2},
		{"missing id", strings.Replace(valid, "'@id': person:zt507zg4MOyd_2TGcpFB0\n", "", 1), "@id", "required", 2},
		{"missing type", strings.Replace(valid, "'@type': Person\n", "", 1), "@type", "required", 2},
		{"missing name", strings.Replace(valid, "name: Alice\n", "", 1), "name", "required", 2},
		{"wrong type", strings.Replace(valid, "name: Alice", "name: 123", 1), "name", "expected string; found number", 5},
		{"blank name", strings.Replace(valid, "name: Alice", "name: '  '", 1), "name", "cannot be empty", 5},
		{"wrong context", strings.Replace(valid, "https://schema.org", "https://example.com", 1), "@context", "", 2},
		{"unknown type", strings.Replace(valid, "Person", "Place", 1), "@type", "", 4},
		{"invalid id", strings.Replace(valid, "person:zt507zg4MOyd_2TGcpFB0", "Alice Smith with a long name", 1), "@id", "", 3},
		{"relative id", strings.Replace(valid, "person:zt507zg4MOyd_2TGcpFB0", "zt507zg4MOyd_2TGcpFB0x", 1), "@id", "uri", 3},
		{"short id", strings.Replace(valid, "person:zt507zg4MOyd_2TGcpFB0", "person:alice", 1), "@id", "at least 21 characters; found 12", 3},
		{"invalid utf8", valid + "\xff", "frontmatter", "UTF-8", 1},
		{"multiple yaml documents", "---\nname: Alice\n...\nname: Bob\n---\n", "frontmatter", "", 2},
		{"sequence metadata", "---\n- Alice\n---\n", "frontmatter", "mapping", 2},
		{"empty metadata", "---\n---\n", "frontmatter", "mapping", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := workspace(t, map[string]string{"alice.md": tt.source})

			diagnostics, err := core.Compile(root, directory.Schema)
			if err != nil {
				t.Fatal(err)
			}

			if len(diagnostics) != 1 {
				t.Fatalf("expected one diagnostic, got %v", diagnostics)
			}

			for _, d := range diagnostics {
				if d.Path == "alice.md" && d.Field == tt.field && d.Line == tt.line && strings.Contains(d.Message, tt.message) {
					return
				}
			}

			t.Fatalf("missing expected diagnostic for %s at line %d: %v", tt.field, tt.line, diagnostics)
		})
	}
}

func TestCompileIDLength(t *testing.T) {
	for _, length := range []int{20, 21} {
		id := "person:" + strings.Repeat("a", length-len("person:"))
		root := workspace(t, map[string]string{"alice.md": record("Person", id, "Alice")})

		diagnostics, err := core.Compile(root, directory.Schema)
		if err != nil {
			t.Fatal(err)
		}

		if length == 21 {
			if len(diagnostics) != 0 {
				t.Fatalf("minimum-length ID rejected: %v", diagnostics)
			}
		} else if len(diagnostics) != 1 || diagnostics[0].Field != "@id" || !strings.Contains(diagnostics[0].Message, "at least 21 characters; found 20") {
			t.Fatalf("expected short ID diagnostic, got %v", diagnostics)
		}
	}
}

func TestCompileDuplicateIDs(t *testing.T) {
	root := workspace(t, map[string]string{
		"a.md": record("Person", "person:zt507zg4MOyd_2TGcpFB0", "Alice"),
		"b.md": record("Person", "person:zt507zg4MOyd_2TGcpFB0", "Alice Jones"),
	})

	diagnostics, err := core.Compile(root, directory.Schema)
	if err != nil || len(diagnostics) != 1 || diagnostics[0].Field != "@id" || diagnostics[0].Line != 3 || !strings.Contains(diagnostics[0].Message, "a.md") {
		t.Fatalf("Compile = %v, %v", diagnostics, err)
	}
}

func TestCompileLinks(t *testing.T) {
	tests := []struct {
		name, body string
		valid      bool
	}{
		{"relative", "[Bob](nested/bob.md#hello-world)", true},
		{"root relative", "[Bob](/nested/bob.md)", true},
		{"fragment", "[Self](#alice)", true},
		{"empty destination", "[Self]()", true},
		{"duplicate heading", "[Bob](nested/bob.md#hello-world-1)", true},
		{"encoded path", "[File](a%20b.md#hello)", true},
		{"escaped parentheses", `[File](a\(b\).md)`, true},
		{"entity", "[File](a&amp;b.md)", true},
		{"reference", "[Bob][b]\n\n[b]: nested/bob.md#hello-world", true},
		{"collapsed reference", "[b][]\n\n[b]: nested/bob.md", true},
		{"shortcut reference", "[b]\n\n[b]: nested/bob.md", true},
		{"code", "`[Missing](missing.md)`", true},
		{"escaped link", `\[Missing](missing.md)`, true},
		{"missing file", "[Missing](missing.md)", false},
		{"missing image", "![Missing](missing.png)", false},
		{"missing reference target", "[Missing][m]\n\n[m]: missing.md", false},
		{"missing heading", "[Bob](nested/bob.md#missing)", false},
		{"missing self heading", "[Self](#missing)", false},
		{"directory", "[Folder](nested/)", false},
		{"escape root", "[Outside](../outside.md)", false},
		{"encoded escape", "[Outside](%2e%2e/outside.md)", false},
		{"file URL", "[Outside](file:///tmp/outside.md)", false},
		{"invalid URL", "[Invalid](https://example.com/%zz)", false},
		{"missing host", "[Invalid](https:/example.com)", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := workspace(t, map[string]string{
				"alice.md":      record("Person", "person:zt507zg4MOyd_2TGcpFB0", "Alice") + "# Alice\n\n" + tt.body + "\n",
				"nested/bob.md": record("Person", "person:PkzrP9vKxncFbh78ydJOc", "Bob") + "# Hello World\n\n# Hello World\n",
				"a b.md":        record("Person", "person:4CcWQiPT6wzl1CaoqNKPQ", "Space") + "# Hello\n",
				"a(b).md":       record("Person", "person:68YDbHOw_ftdD_S18kJLM", "Bracket"),
				"a&b.md":        record("Person", "person:gZq34KLSI5rG6pSt6USDW", "Ampersand"),
			})

			diagnostics, err := core.Compile(root, directory.Schema)
			if err != nil {
				t.Fatal(err)
			}

			if tt.valid {
				if len(diagnostics) != 0 {
					t.Fatalf("unexpected diagnostics: %v", diagnostics)
				}

				return
			}

			for _, d := range diagnostics {
				if d.Path == "alice.md" && d.Field == "link" && d.Line == 9 {
					return
				}
			}

			t.Fatalf("missing link diagnostic at line 9: %v", diagnostics)
		})
	}
}

func TestCompileCustomSchema(t *testing.T) {
	schema := []byte(`{"$schema":"http://json-schema.org/draft-07/schema#","type":"object","required":["@type","name"],"properties":{"@type":{"const":"Place"},"name":{"type":"string"}},"additionalProperties":false}`)
	root := workspace(t, map[string]string{"place.md": "---\n'@type': Place\nname: London\n---\n# London\n"})
	diagnostics, err := core.Compile(root, schema)
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("Compile = %v, %v", diagnostics, err)
	}
}

func TestCompileOperationalErrors(t *testing.T) {
	root := t.TempDir()
	for _, schema := range [][]byte{[]byte(`{`), []byte(`{"type":"not-a-type"}`)} {
		if _, err := core.Compile(root, schema); err == nil {
			t.Fatal("expected schema error")
		}
	}

	if _, err := core.Compile(filepath.Join(root, "missing"), directory.Schema); err == nil {
		t.Fatal("expected root error")
	}
}

func TestCompileSymlinks(t *testing.T) {
	outside := workspace(t, map[string]string{"outside.md": record("Person", "person:JSB0O1wvuZxClXgtZrfJn", "Outside")})
	root := workspace(t, map[string]string{"alice.md": record("Person", "person:zt507zg4MOyd_2TGcpFB0", "Alice") + "[Outside](escape/outside.md)\n"})
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}

	diagnostics, err := core.Compile(root, directory.Schema)
	if err != nil || len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, "leaves the workspace") {
		t.Fatalf("Compile = %v, %v", diagnostics, err)
	}
}

func TestCompileLineEndingsAndOrder(t *testing.T) {
	content := record("Person", "person:zt507zg4MOyd_2TGcpFB0", "Alice") + "[Missing](missing.md)\n"
	root := workspace(t, map[string]string{
		"a.md": "\xef\xbb\xbf" + strings.ReplaceAll(content, "\n", "\r\n"),
		"z.md": strings.ReplaceAll(content, "person:zt507zg4MOyd_2TGcpFB0", "person:PkzrP9vKxncFbh78ydJOc"),
	})
	first, err := core.Compile(root, directory.Schema)
	if err != nil || len(first) != 2 || first[0].Path != "a.md" || first[0].Line != 7 || first[1].Path != "z.md" {
		t.Fatalf("Compile = %v, %v", first, err)
	}

	second, err := core.Compile(root, directory.Schema)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("unstable diagnostics: %v, %v", second, err)
	}
}

func workspace(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()

	for name, content := range files {
		filename := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	return root
}

func record(kind, id, name string) string {
	return "---\n'@context': https://schema.org\n'@id': " + id + "\n'@type': " + kind + "\nname: " + name + "\n---\n"
}

func TestCompileContinuesAfterBrokenPath(t *testing.T) {
	root := workspace(t, map[string]string{
		"a.md":      record("Person", "person:zt507zg4MOyd_2TGcpFB0", "123") + "[Asset](asset.txt/child.md)\n",
		"b.md":      record("Person", "person:PkzrP9vKxncFbh78ydJOc", "Bob") + "[Missing](missing.md)\n",
		"asset.txt": "asset",
	})
	got, err := core.Compile(root, directory.Schema)
	if err != nil {
		t.Fatal(err)
	}

	want := []core.Diagnostic{
		{Path: "a.md", Line: 5, Field: "name", Message: "expected string; found number"},
		{Path: "a.md", Line: 7, Field: "link", Message: `link target does not exist: "asset.txt/child.md"`},
		{Path: "b.md", Line: 7, Field: "link", Message: `link target does not exist: "missing.md"`},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diagnostics = %#v, want %#v", got, want)
	}
}

func TestCompileSchemaLocations(t *testing.T) {
	tests := []struct {
		name, source, schema string
		want                 []core.Diagnostic
	}{
		{
			name:   "quoted property",
			source: "---\nname: Alice\n\"owner's.name\": 12\n---\n",
			schema: `{"type":"object","properties":{"owner's.name":{"type":"string"}}}`,
			want:   []core.Diagnostic{{Path: "record.md", Line: 3, Field: "owner's.name", Message: "expected string; found number"}},
		},
		{
			name:   "array item",
			source: "---\nnames:\n  - Alice\n  - 12\n---\n",
			schema: `{"definitions":{"name":{"type":"string"}},"type":"object","properties":{"names":{"type":"array","items":{"$ref":"#/definitions/name"}}}}`,
			want:   []core.Diagnostic{{Path: "record.md", Line: 4, Field: "names.1", Message: "expected string; found number"}},
		},
		{
			name:   "distinct paths with same displayed field",
			source: "---\n'a.b': 12\na:\n  b: 34\n---\n",
			schema: `{"type":"object","properties":{"a.b":{"type":"string"},"a":{"type":"object","properties":{"b":{"type":"string"}}}}}`,
			want: []core.Diagnostic{
				{Path: "record.md", Line: 2, Field: "a.b", Message: "expected string; found number"},
				{Path: "record.md", Line: 4, Field: "a.b", Message: "expected string; found number"},
			},
		},
		{
			name:   "nested array with punctuation",
			source: "---\ncontacts:\n  - 'a[0]/~': 12\n---\n",
			schema: `{"properties":{"contacts":{"items":{"properties":{"a[0]/~":{"type":"string"}}}}}}`,
			want:   []core.Diagnostic{{Path: "record.md", Line: 3, Field: "contacts.0.a[0]/~", Message: "expected string; found number"}},
		},
		{
			name:   "missing nested field uses containing object",
			source: "---\nname: Alice\naddress:\n  city: London\n---\n",
			schema: `{"properties":{"address":{"required":["street"]}}}`,
			want:   []core.Diagnostic{{Path: "record.md", Line: 3, Field: "address.street", Message: "required field is missing; add it"}},
		},
		{
			name:   "unknown quoted property",
			source: "---\nname: Alice\n\"owner's.name\": Bob\n---\n",
			schema: `{"properties":{"name":{"type":"string"}},"additionalProperties":false}`,
			want:   []core.Diagnostic{{Path: "record.md", Line: 3, Field: "owner's.name", Message: "field is not allowed by the schema; remove it"}},
		},
		{
			name:   "anchor and alias locations",
			source: "---\ndefault: &contact\n  name: 12\ncontact: *contact\n---\n",
			schema: `{"properties":{"default":{"properties":{"name":{"type":"string"}}},"contact":{"properties":{"name":{"type":"string"}}}}}`,
			want: []core.Diagnostic{
				{Path: "record.md", Line: 3, Field: "default.name", Message: "expected string; found number"},
				{Path: "record.md", Line: 4, Field: "contact.name", Message: "expected string; found number"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := workspace(t, map[string]string{"record.md": tt.source})
			got, err := core.Compile(root, []byte(tt.schema))
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("diagnostics = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestCompileSamples(t *testing.T) {
	valid, err := core.Compile("testdata/valid", directory.Schema)
	if err != nil || len(valid) != 0 {
		t.Fatalf("valid sample: %v, %v", valid, err)
	}
	diagnostics, err := core.Compile("testdata/invalid", directory.Schema)
	if err != nil {
		t.Fatal(err)
	}

	var actual strings.Builder
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(&actual, diagnostic)
	}

	expected, err := os.ReadFile("testdata/invalid.errors.txt")
	if err != nil {
		t.Fatal(err)
	}

	if actual.String() != string(expected) {
		t.Fatalf("diagnostics:\n%s\nwant:\n%s", actual.String(), expected)
	}
}

func TestCompileContinuesAfterUnreadableFile(t *testing.T) {
	root := workspace(t, map[string]string{
		"a.md": record("Person", "person:zt507zg4MOyd_2TGcpFB0", "Alice"),
		"b.md": record("Person", "person:PkzrP9vKxncFbh78ydJOc", "42"),
	})

	path := filepath.Join(root, "a.md")
	if err := os.Chmod(path, 0000); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { os.Chmod(path, 0600) })

	if _, err := os.ReadFile(path); err == nil {
		t.Skip("this environment can read files with no read permissions")
	}

	diagnostics, err := core.Compile(root, directory.Schema)
	if err != nil {
		t.Fatal(err)
	}

	if len(diagnostics) != 2 || diagnostics[0].Path != "a.md" || diagnostics[0].Field != "file" || !strings.Contains(diagnostics[0].Message, "cannot read file") || diagnostics[1].Path != "b.md" || diagnostics[1].Field != "name" {
		t.Fatalf("expected both file and schema errors: %v", diagnostics)
	}
}

func TestCompileReportsAllMissingFields(t *testing.T) {
	root := workspace(t, map[string]string{"empty.md": "---\n{}\n---\n"})

	diagnostics, err := core.Compile(root, directory.Schema)
	if err != nil {
		t.Fatal(err)
	}

	want := []core.Diagnostic{
		{Path: "empty.md", Line: 2, Field: "@context", Message: "required field is missing; add it"},
		{Path: "empty.md", Line: 2, Field: "@id", Message: "required field is missing; add it"},
		{Path: "empty.md", Line: 2, Field: "@type", Message: "required field is missing; add it"},
		{Path: "empty.md", Line: 2, Field: "name", Message: "required field is missing; add it"},
	}
	if !reflect.DeepEqual(diagnostics, want) {
		t.Fatalf("diagnostics = %#v, want %#v", diagnostics, want)
	}
}
