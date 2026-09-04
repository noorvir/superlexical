package main

import (
	"bufio"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"superlexical/core"
)

func TestWatchRevalidatesWholeDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../core/testdata/valid")); err != nil {
		t.Fatal(err)
	}
	schema := filepath.Join(t.TempDir(), "schema.json")
	if err := os.WriteFile(schema, core.DirectorySchema, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	reader, writer := io.Pipe()
	lines := make(chan string, 64)
	finished := make(chan int, 1)
	go func() {
		defer close(lines)
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		status := watch(ctx, root, schema, writer, writer)
		writer.Close()
		finished <- status
	}()
	t.Cleanup(func() {
		cancel()
		reader.Close()
		writer.Close()
		select {
		case status := <-finished:
			if !t.Failed() && status != 0 {
				t.Errorf("watch stopped with status %d", status)
			}
		case <-time.After(5 * time.Second):
			t.Error("watch did not stop after cancellation")
		}
	})
	waitFor := func(prefix string) {
		t.Helper()
		deadline := time.NewTimer(5 * time.Second)
		defer deadline.Stop()
		var output []string
		for {
			select {
			case line, open := <-lines:
				if !open {
					t.Fatalf("watch ended before %q: %v", prefix, output)
				}
				output = append(output, line)
				if strings.HasPrefix(line, prefix) {
					return
				}
			case <-deadline.C:
				t.Fatalf("no %q: %v", prefix, output)
			}
		}
	}
	waitFor("Valid:")

	bob := filepath.Join(root, "people", "bob.md")
	original, err := os.ReadFile(bob)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(bob); err != nil {
		t.Fatal(err)
	}
	// Removing Bob must invalidate links in two other, unchanged documents.
	waitFor("Validation failed: 2 errors.")
	if err := os.WriteFile(bob+".tmp", original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(bob+".tmp", bob); err != nil {
		t.Fatal(err)
	}
	waitFor("Valid:")

	folder := filepath.Join(root, "new")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	newFile := filepath.Join(folder, "person.md")
	record := "---\n'@context': https://schema.org\n'@id': person:new\n'@type': Person\nname: 42\n---\n"
	if err := os.WriteFile(newFile, []byte(record), 0600); err != nil {
		t.Fatal(err)
	}
	waitFor("Validation failed: 1 error.")
	if err := os.WriteFile(newFile, []byte(strings.Replace(record, "42", "New Person", 1)), 0600); err != nil {
		t.Fatal(err)
	}
	waitFor("Valid:")

	if err := os.WriteFile(schema+".tmp", []byte(`{"required":["extra"]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(schema+".tmp", schema); err != nil {
		t.Fatal(err)
	}
	waitFor("Validation failed: 4 errors.")
	if err := os.WriteFile(schema, core.DirectorySchema, 0600); err != nil {
		t.Fatal(err)
	}
	waitFor("Valid:")
}
