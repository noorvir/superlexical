package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateSamples(t *testing.T) {
	var stdout, stderr bytes.Buffer
	valid := filepath.FromSlash("../../core/testdata/valid")
	if status := run([]string{"validate", "--schema", "../../extensions/directory/schema.json", valid}, &stdout, &stderr); status != 0 || stderr.Len() != 0 || stdout.String() != "Valid: "+valid+"\n" {
		t.Fatalf("status=%d, stdout=%q, stderr=%q", status, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	invalid := filepath.FromSlash("../../core/testdata/invalid")
	status := run([]string{"validate", "--schema", "../../extensions/directory/schema.json", invalid}, &stdout, &stderr)
	diagnostics, err := os.ReadFile("../../core/testdata/invalid.errors.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("Errors in %s:\n\n%s\nValidation failed: 8 errors. Fix them and run validation again.\n", invalid, diagnostics)
	if status != 1 || stdout.Len() != 0 || stderr.String() != want {
		t.Fatalf("status=%d, stdout=%q, stderr=%q; want %q", status, stdout.String(), stderr.String(), want)
	}
}

func TestValidateCustomSchema(t *testing.T) {
	schema := filepath.Join(t.TempDir(), "schema.json")
	if err := os.WriteFile(schema, []byte(`{"type":"object","required":["extra"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	status := run([]string{"validate", "--schema", schema, "../../core/testdata/valid"}, &stdout, &stderr)
	if status != 1 || strings.Count(stderr.String(), "extra: required field is missing; add it") != 3 || !strings.Contains(stderr.String(), "3 errors") {
		t.Fatalf("status=%d, stderr=%q", status, stderr.String())
	}
}

func TestValidateWorkspace(t *testing.T) {
	var stdout, stderr bytes.Buffer
	root := "../../core/testdata/workspace"
	if status := run([]string{"validate", root}, &stdout, &stderr); status != 0 || stderr.Len() != 0 || stdout.String() != "Valid: "+root+"\n" {
		t.Fatalf("status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
}

func TestCommandErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"validate"}, {"validate", "--unknown"}, {"validate", "does-not-exist"}, {"validate", "--schema", "missing.json", "../../core/testdata/valid"}} {
		var stdout, stderr bytes.Buffer
		if status := run(args, &stdout, &stderr); status != 2 || stderr.Len() == 0 || stdout.Len() != 0 {
			t.Errorf("args=%v: status=%d, stdout=%q, stderr=%q", args, status, stdout.String(), stderr.String())
		}
	}
	var stdout, stderr bytes.Buffer
	if status := run([]string{"--help"}, &stdout, &stderr); status != 0 || !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("help: status=%d stdout=%q", status, stdout.String())
	}
}
