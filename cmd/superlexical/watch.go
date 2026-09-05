package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

func watch(ctx context.Context, root, schemaPath string, stdout, stderr io.Writer) int {
	root, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(stderr, "Cannot watch directory: %s\n", err)
		return 2
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		fmt.Fprintf(stderr, "Cannot watch directory: %s\n", err)
		return 2
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(stderr, "Cannot watch %q: select an existing directory.\n", root)
		return 2
	}
	if schemaPath != "" {
		schemaPath, err = filepath.Abs(schemaPath)
		if err != nil {
			fmt.Fprintf(stderr, "Cannot watch schema: %s\n", err)
			return 2
		}
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(stderr, "Cannot start file watcher: %s\n", err)
		return 2
	}
	defer watcher.Close()
	if err := syncWatches(watcher, root, schemaPath); err != nil {
		fmt.Fprintf(stderr, "Cannot watch directory: %s\n", err)
		return 2
	}
	return watchChanges(ctx, watcher, root, schemaPath, stdout, stderr)
}

// watchChanges validates after a quiet interval and returns when the caller
// cancels or the watcher fails. The caller owns and closes the watcher.
func watchChanges(ctx context.Context, watcher *fsnotify.Watcher, root, schemaPath string, stdout, stderr io.Writer) int {
	fmt.Fprintf(stdout, "Watching %s. Press Ctrl+C to stop.\n", root)
	status := validate(root, schemaPath, stdout, stderr)

	timer := time.NewTimer(150 * time.Millisecond)
	timer.Stop()
	defer timer.Stop()
	var changed <-chan time.Time
	for {
		select {
		case <-ctx.Done():
			return status
		case event, open := <-watcher.Events:
			if !open {
				fmt.Fprintln(stderr, "File watcher stopped. Validation is no longer automatic.")
				return 2
			}
			if !validationEvent(root, schemaPath, event.Name) {
				continue
			}
			timer.Reset(150 * time.Millisecond)
			changed = timer.C
		case <-changed:
			changed = nil
			if err := syncWatches(watcher, root, schemaPath); err != nil {
				fmt.Fprintf(stderr, "Cannot update file watches: %s\n", err)
				return 2
			}
			fmt.Fprintln(stdout, "\nChanges detected. Validating the full directory...")
			status = validate(root, schemaPath, stdout, stderr)
		case err, open := <-watcher.Errors:
			if !open {
				fmt.Fprintln(stderr, "File watcher stopped. Validation is no longer automatic.")
				return 2
			}
			fmt.Fprintf(stderr, "File watcher failed: %s. Run validation again.\n", err)
			return 2
		}
	}
}

func syncWatches(watcher *fsnotify.Watcher, root, schemaPath string) error {
	wanted := map[string]bool{filepath.Dir(root): true}
	if schemaPath != "" {
		wanted[filepath.Dir(schemaPath)] = true
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if errors.Is(walkErr, os.ErrNotExist) {
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if entry.Name() == ".git" {
			return filepath.SkipDir
		}
		wanted[path] = true
		return nil
	})
	if err != nil {
		return err
	}
	for _, path := range watcher.WatchList() {
		if wanted[path] {
			delete(wanted, path)
			continue
		}
		if err := watcher.Remove(path); err != nil && !errors.Is(err, fsnotify.ErrNonExistentWatch) {
			return err
		}
	}
	for path := range wanted {
		if err := watcher.Add(path); err != nil {
			return err
		}
	}
	return nil
}

// validationEvent excludes Git metadata and changes outside the watched roots.
func validationEvent(root, schemaPath, filename string) bool {
	if filename == schemaPath {
		return true
	}
	relative, err := filepath.Rel(root, filename)
	if err != nil || !filepath.IsLocal(relative) {
		return false
	}
	path := "/" + filepath.ToSlash(relative) + "/"
	return !strings.Contains(path, "/.git/")
}
