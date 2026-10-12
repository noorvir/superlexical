// Package store keeps a Superlexical workspace in a Git repository. The store
// is the only writer of the workspace. It applies one change at a time, and a
// commit exists only for a workspace that passed validation, so the history is
// linear and the newest commit is always the last known good version.
package store

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"superlexical/core"
)

// ErrNotFound reports that a path is not in the last commit.
var ErrNotFound = errors.New("path is not in the last commit")

// Store reads the last commit of a workspace and adds commits to it.
type Store struct {
	root string

	// writes serializes Apply. changed holds, for each path, the last change
	// that this process applied to it.
	writes  sync.Mutex
	changed map[string]fileChange

	// head is the hash of the last commit.
	head atomic.Pointer[string]
}

type fileChange struct {
	at     time.Time
	author string
}

// Open returns the store for the workspace at root. It creates the Git
// repository and the first commit when they do not exist. The workspace must
// be valid for the first commit.
func Open(root string) (*Store, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	store := &Store{root: root, changed: make(map[string]fileChange)}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		if _, err := store.git("init", "--quiet", "--initial-branch=main"); err != nil {
			return nil, err
		}
	}

	if head, err := store.git("rev-parse", "--verify", "--quiet", "HEAD"); err == nil {
		hash := strings.TrimSpace(string(head))
		store.head.Store(&hash)
		return store, nil
	}

	diagnostics, err := core.CompileWorkspace(root)
	if err != nil {
		return nil, err
	}

	if len(diagnostics) != 0 {
		return nil, fmt.Errorf("workspace has %d validation errors; the first commit needs a valid workspace", len(diagnostics))
	}

	if _, err := store.commit("Superlexical", "Add workspace"); err != nil {
		return nil, err
	}

	return store, nil
}

// Change is a set of edits that the store applies as one commit.
type Change struct {
	// Author names the agent or person that made the change.
	Author  string
	Message string
	// SentAt is the time when the author sent the change. The store rejects
	// the change when one of its files changed after that time.
	SentAt       time.Time
	Creates      []Create
	Replacements []Replacement
}

// Create adds a new Markdown file. The file must not exist.
type Create struct {
	Path    string
	Content string
}

// Replacement replaces one piece of text in an existing Markdown file.
// OldText must occur exactly once in the file.
type Replacement struct {
	Path    string
	OldText string
	NewText string
}

// Result is the outcome of Apply. A change with rejections or diagnostics was
// not applied; Commit is then empty.
type Result struct {
	Commit string
	// Rejections are edits that the store cannot apply to the current files.
	Rejections []Rejection
	// Diagnostics are the validation errors of the workspace with the change applied.
	Diagnostics []core.Diagnostic
}

// Rejection tells the author why one edit cannot be applied.
type Rejection struct {
	Path    string
	Message string
}

// Apply applies a change, validates the workspace, and commits it. When an
// edit does not fit the current files or the result is not valid, the store
// changes nothing and reports the reason. Calls run one at a time.
func (s *Store) Apply(change Change) (Result, error) {
	s.writes.Lock()
	defer s.writes.Unlock()

	rejections, err := s.check(change)
	if err != nil || len(rejections) != 0 {
		return Result{Rejections: rejections}, err
	}

	result, err := s.write(change)
	if err != nil || len(result.Diagnostics) != 0 {
		if discardErr := s.discard(); discardErr != nil {
			return Result{}, errors.Join(err, discardErr)
		}

		return result, err
	}

	now := time.Now()
	for _, path := range change.paths() {
		s.changed[path] = fileChange{at: now, author: change.Author}
	}

	return result, nil
}

func (c Change) paths() []string {
	var paths []string
	for _, create := range c.Creates {
		paths = append(paths, create.Path)
	}
	for _, replacement := range c.Replacements {
		paths = append(paths, replacement.Path)
	}

	return paths
}

// check finds the edits that cannot be applied to the current files.
func (s *Store) check(change Change) ([]Rejection, error) {
	var rejections []Rejection

	for _, path := range change.paths() {
		if !validPath(path) || !strings.HasSuffix(path, ".md") {
			rejections = append(rejections, Rejection{path, "not a Markdown file path in the workspace"})
			continue
		}

		last, found := s.changed[path]
		if found && last.at.After(change.SentAt) {
			message := fmt.Sprintf("%s changed this file after you sent your change; read the file again and send a new change", last.author)
			rejections = append(rejections, Rejection{path, message})
		}
	}

	if len(rejections) != 0 {
		return rejections, nil
	}

	for _, create := range change.Creates {
		if _, err := os.Stat(s.filename(create.Path)); err == nil {
			rejections = append(rejections, Rejection{create.Path, "the file exists already"})
		}
	}

	for _, replacement := range change.Replacements {
		source, err := os.ReadFile(s.filename(replacement.Path))
		if errors.Is(err, os.ErrNotExist) {
			rejections = append(rejections, Rejection{replacement.Path, "the file does not exist; link only to pages that exist"})
			continue
		}
		if err != nil {
			return nil, err
		}

		if count := strings.Count(string(source), replacement.OldText); count != 1 {
			message := fmt.Sprintf("the old text occurs %d times in the file; it must occur exactly once", count)
			rejections = append(rejections, Rejection{replacement.Path, message})
		}
	}

	return rejections, nil
}

// write applies the edits to the working files and commits a valid result.
// The caller discards the working files when it reports diagnostics or an error.
func (s *Store) write(change Change) (Result, error) {
	for _, create := range change.Creates {
		filename := s.filename(create.Path)
		if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
			return Result{}, err
		}

		if err := os.WriteFile(filename, []byte(create.Content), 0o644); err != nil {
			return Result{}, err
		}
	}

	// Replacements run in order, so that two of them can change the same file.
	for _, replacement := range change.Replacements {
		filename := s.filename(replacement.Path)
		source, err := os.ReadFile(filename)
		if err != nil {
			return Result{}, err
		}

		updated := strings.Replace(string(source), replacement.OldText, replacement.NewText, 1)
		if err := os.WriteFile(filename, []byte(updated), 0o644); err != nil {
			return Result{}, err
		}
	}

	diagnostics, err := core.CompileWorkspace(s.root)
	if err != nil || len(diagnostics) != 0 {
		return Result{Diagnostics: diagnostics}, err
	}

	head, err := s.commit(change.Author, change.Message)
	return Result{Commit: head}, err
}

func (s *Store) filename(path string) string {
	return filepath.Join(s.root, filepath.FromSlash(path))
}

// commit records the working files as a commit and returns its hash.
func (s *Store) commit(author, message string) (string, error) {
	if _, err := s.git("add", "--all"); err != nil {
		return "", err
	}

	status, err := s.git("status", "--porcelain")
	if err != nil {
		return "", err
	}

	if len(bytes.TrimSpace(status)) != 0 {
		identity := []string{"-c", "user.name=" + author, "-c", "user.email=superlexical@localhost"}
		args := append(identity, "commit", "--quiet", "--message", message)
		if _, err := s.git(args...); err != nil {
			return "", err
		}
	}

	head, err := s.git("rev-parse", "HEAD")
	if err != nil {
		return "", err
	}

	hash := strings.TrimSpace(string(head))
	s.head.Store(&hash)
	return hash, nil
}

// Head returns the hash of the last commit. All data that the store serves
// stays the same while this value stays the same.
func (s *Store) Head() string {
	return *s.head.Load()
}

// discard restores the working files to the last commit.
func (s *Store) discard() error {
	if _, err := s.git("reset", "--quiet", "--hard", "HEAD"); err != nil {
		return err
	}

	_, err := s.git("clean", "--quiet", "--force", "-d")
	return err
}

// ReadFile returns a file from the last commit. The path is relative to the
// workspace root and uses forward slashes.
func (s *Store) ReadFile(path string) ([]byte, error) {
	if !validPath(path) {
		return nil, ErrNotFound
	}

	source, err := s.git("show", "HEAD:"+path)
	if err != nil {
		return nil, ErrNotFound
	}

	return source, nil
}

// List returns the names of the files and folders in a folder of the last
// commit. Folder names end with a slash.
func (s *Store) List(folder string) ([]string, error) {
	if !validPath(folder) {
		return nil, ErrNotFound
	}

	output, err := s.git("ls-tree", "HEAD", folder+"/")
	if err != nil {
		return nil, err
	}

	var names []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		kind, path, found := strings.Cut(line, "\t")
		if !found {
			continue
		}

		name := strings.TrimPrefix(path, folder+"/")
		if strings.Contains(kind, " tree ") {
			name += "/"
		}

		names = append(names, name)
	}

	return names, nil
}

func validPath(path string) bool {
	if path == "" || strings.HasPrefix(path, "/") || strings.HasPrefix(path, "-") {
		return false
	}

	for _, segment := range strings.Split(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}

	return true
}

func (s *Store) git(args ...string) ([]byte, error) {
	command := exec.Command("git", append([]string{"-C", s.root, "-c", "core.quotePath=false"}, args...)...)
	var stderr bytes.Buffer
	command.Stderr = &stderr

	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}

	return output, nil
}
