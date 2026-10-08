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

	"superlexical/extensions/directory"
)

func TestWatchRevalidatesWholeDirectory(t *testing.T) {
	for _, mode := range []string{"schema", "workspace"} {
		t.Run(mode, func(t *testing.T) {

			root := t.TempDir()
			dataRoot := root

			if mode == "workspace" {
				dataRoot = filepath.Join(root, "directory")
			}
			if err := os.CopyFS(dataRoot, os.DirFS("../../core/testdata/valid")); err != nil {
				t.Fatal(err)
			}

			schema := filepath.Join(t.TempDir(), "schema.json")
			if err := os.WriteFile(schema, directory.Schema, 0600); err != nil {
				t.Fatal(err)
			}

			schemaArg := schema
			if mode == "workspace" {
				configDir := filepath.Join(root, ".superlexical")
				if err := os.MkdirAll(configDir, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(configDir, "extensions.json"), []byte(`{"extensions":[{"name":"directory","dependencies":[]}]}`), 0600); err != nil {
					t.Fatal(err)
				}
				schemaArg = ""
			}

			ctx, cancel := context.WithCancel(context.Background())
			reader, writer := io.Pipe()
			lines := make(chan string, 64)
			finished := make(chan int, 1)
			scanDone := make(chan struct{})

			go func() {
				defer close(scanDone)
				defer close(lines)

				scanner := bufio.NewScanner(reader)
				for scanner.Scan() {
					select {
					case lines <- scanner.Text():
					case <-ctx.Done():
						return
					}
				}

				if err := scanner.Err(); err != nil && ctx.Err() == nil {
					t.Errorf("scan watch output: %v", err)
				}
			}()

			go func() {
				status := watch(ctx, root, schemaArg, writer, writer)
				writer.Close()
				finished <- status
			}()

			t.Cleanup(func() {
				cancel()
				reader.Close()
				writer.Close()
				<-scanDone

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

			bob := filepath.Join(dataRoot, "people", "bob.md")
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

			folder := filepath.Join(dataRoot, "new")
			if err := os.Mkdir(folder, 0700); err != nil {
				t.Fatal(err)
			}

			newFile := filepath.Join(folder, "person.md")
			record := "---\n'@context': https://schema.org\n'@id': person:AIDHk0djXA_dqkF0R2UHK\n'@type': Person\nname: 42\n---\n"
			if err := os.WriteFile(newFile, []byte(record), 0600); err != nil {
				t.Fatal(err)
			}

			waitFor("Validation failed: 1 error.")

			if err := os.WriteFile(newFile, []byte(strings.Replace(record, "42", "New Person", 1)), 0600); err != nil {
				t.Fatal(err)
			}

			waitFor("Valid:")

			if mode == "schema" {
				if err := os.WriteFile(schema+".tmp", []byte(`{"required":["extra"]}`), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(schema+".tmp", schema); err != nil {
					t.Fatal(err)
				}

				waitFor("Validation failed: 4 errors.")

				if err := os.WriteFile(schema, directory.Schema, 0600); err != nil {
					t.Fatal(err)
				}

				waitFor("Valid:")

			}

			if mode == "workspace" {
				configPath := filepath.Join(root, ".superlexical", "extensions.json")
				original, err := os.ReadFile(configPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(configPath, []byte(`{"extensions":[{"name":"directory","dependencies":["photos"]}]}`), 0600); err != nil {
					t.Fatal(err)
				}

				waitFor("Validation failed: 1 error.")

				if err := os.WriteFile(configPath, original, 0600); err != nil {
					t.Fatal(err)
				}
				waitFor("Valid:")
			}
		})
	}
}
