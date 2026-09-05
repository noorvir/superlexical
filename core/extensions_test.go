package core_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"superlexical/core"
)

func TestCompileWorkspace(t *testing.T) {
	root := workspace(t, map[string]string{
		".superlexical/extensions.json":   `{"extensions":[{"name":"directory","dependencies":[]}]}`,
		"directory/alice.md":              record("Person", "person:zt507zg4MOyd_2TGcpFB0", "Alice") + "[Acme](organizations/acme.md#team)\n![Photo](/assets/alice.png)\n",
		"directory/organizations/acme.md": record("Organization", "organization:DZO8p_bVF1ENdFAYHmxc7", "Acme") + "# Team\n[Alice](../alice.md)\n",
		"assets/alice.png":                "image",
		"README.md":                       "# Unowned note\nNo record metadata here.\n",
	})
	diagnostics, err := core.CompileWorkspace(root)
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("CompileWorkspace = %v, %v", diagnostics, err)
	}
	// Independent configuration and record errors must all reach the caller.
	if err := os.WriteFile(filepath.Join(root, ".superlexical/extensions.json"), []byte(`{"extensions":[{"name":"directory","dependencies":["photos"]},{"name":"sharing","dependencies":[]}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "directory/bob.md"), []byte(record("Person", "person:zt507zg4MOyd_2TGcpFB0", "42")+"[Missing](missing.md)"), 0600); err != nil {
		t.Fatal(err)
	}
	diagnostics, err = core.CompileWorkspace(root)
	if err != nil || len(diagnostics) != 5 {
		t.Fatalf("CompileWorkspace = %v, %v", diagnostics, err)
	}
	for i, message := range []string{"dependencies are not supported", "unsupported extension", "duplicate ID", "expected string", "link target does not exist"} {
		if !strings.Contains(diagnostics[i].Message, message) {
			t.Errorf("diagnostic %d = %v; want %q", i, diagnostics[i], message)
		}
	}
}

func TestCompileWorkspaceConfiguration(t *testing.T) {
	for _, tt := range []struct{ config, message string }{
		{`{`, "invalid configuration"},
		{`{} {}`, "one JSON object"},
		{`{}`, "extensions must be an array"},
		{`{"extensions":null}`, "extensions must be an array"},
		{`{"extensions":[{"name":"photos","dependencies":[]}]}`, "unsupported extension"},
		{`{"extensions":[{"name":"directory"}]}`, "dependencies must be an empty array"},
		{`{"extensions":[{"name":"directory","dependencies":["photos"]}]}`, "dependencies are not supported"},
		{`{"extensions":[{"name":"directory","dependencies":[]},{"name":"directory","dependencies":[]}]}`, "listed more than once"},
		{`{"extensions":[{"name":"directory","dependencies":[],"path":"elsewhere"}]}`, "unknown field"},
	} {
		t.Run(tt.config, func(t *testing.T) {
			root := workspace(t, map[string]string{
				".superlexical/extensions.json": tt.config,
				"directory/alice.md":            record("Person", "person:zt507zg4MOyd_2TGcpFB0", "Alice"),
			})
			diagnostics, err := core.CompileWorkspace(root)
			if err != nil || len(diagnostics) != 1 || diagnostics[0].Path != ".superlexical/extensions.json" || !strings.Contains(diagnostics[0].Message, tt.message) {
				t.Fatalf("CompileWorkspace = %v, %v", diagnostics, err)
			}
		})
	}
}

func TestCompileWorkspaceFolders(t *testing.T) {
	root := t.TempDir()
	if _, err := core.CompileWorkspace(root); err == nil {
		t.Fatal("missing configuration passed")
	}
	root = workspace(t, map[string]string{".superlexical/extensions.json": `{"extensions":[{"name":"directory","dependencies":[]}]}`})
	for _, kind := range []string{"missing", "file", "symlink"} {
		folder := filepath.Join(root, "directory")
		switch kind {
		case "file":
			if err := os.WriteFile(folder, []byte("not a directory"), 0600); err != nil {
				t.Fatal(err)
			}
		case "symlink":
			if err := os.Remove(folder); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(t.TempDir(), folder); err != nil {
				t.Fatal(err)
			}
		}
		diagnostics, err := core.CompileWorkspace(root)
		if err != nil || len(diagnostics) != 1 || diagnostics[0].Path != "directory" {
			t.Fatalf("%s: CompileWorkspace = %v, %v", kind, diagnostics, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, ".superlexical/extensions.json"), []byte(`{"extensions":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if diagnostics, err := core.CompileWorkspace(root); err != nil || len(diagnostics) != 0 {
		t.Fatalf("empty extension list = %v, %v", diagnostics, err)
	}
}
