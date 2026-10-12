package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"superlexical/store"
)

func TestListsAndChanges(t *testing.T) {
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../examples/workspace")); err != nil {
		t.Fatal(err)
	}

	workspace, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}

	handler, _ := New(workspace)
	request := func(method, target, body string) (int, string) {
		response := httptest.NewRecorder()
		call := httptest.NewRequest(method, target, strings.NewReader(body))
		call.Header.Set("Content-Type", "application/json")
		handler.ServeHTTP(response, call)
		return response.Code, response.Body.String()
	}

	if status, body := request(http.MethodGet, "/lists/movies", ""); status != http.StatusOK || !strings.Contains(body, `"title":"Gladiator"`) {
		t.Fatalf("status=%d body=%s", status, body)
	}

	read := func(etag string) (int, string) {
		response := httptest.NewRecorder()
		call := httptest.NewRequest(http.MethodGet, "/lists", nil)
		call.Header.Set("If-None-Match", etag)
		handler.ServeHTTP(response, call)
		return response.Code, response.Header().Get("ETag")
	}

	_, etag := read("")
	if status, _ := read(etag); etag == "" || status != http.StatusNotModified {
		t.Fatalf("etag=%q status=%d", etag, status)
	}

	sentAt := time.Now().UTC().Format(time.RFC3339Nano)
	change := func(body string) (int, string) {
		prefix := `{"author":"test","message":"Change","sentAt":"` + sentAt + `",`
		return request(http.MethodPost, "/changes", prefix+body+"}")
	}

	invalid := `---\ntitle: Alien\ndirector: Ridley Scott\n---\n\n# Alien\n`
	status, body := change(`"creates":[{"path":"lists/movies/alien.md","content":"` + invalid + `"}]`)
	if status != http.StatusOK || !strings.Contains(body, `"applied":false`) || !strings.Contains(body, "rotten_tomatoes") {
		t.Fatalf("status=%d body=%s", status, body)
	}

	if _, body := request(http.MethodGet, "/lists/movies", ""); strings.Contains(body, "Alien") {
		t.Fatalf("an invalid entry is served: %s", body)
	}

	valid := `---\ntitle: Alien\ndirector: Ridley Scott\nimdb: 8.5\nrotten_tomatoes: 93\n---\n\n# Alien\n`
	edits := `"creates":[{"path":"lists/movies/alien.md","content":"` + valid + `"}],` +
		`"replacements":[{"path":"lists/movies/gladiator.md","oldText":"A Roman general","newText":"A Roman general, in a film by the director of [Alien](alien.md),"}]`
	status, body = change(edits)
	if status != http.StatusOK || !strings.Contains(body, `"applied":true`) {
		t.Fatalf("status=%d body=%s", status, body)
	}

	if _, body := request(http.MethodGet, "/page?path=lists/movies/alien.md", ""); !strings.Contains(body, `"title":"Alien"`) || !strings.Contains(body, `"value":"93"`) {
		t.Fatalf("body=%s", body)
	}

	if status, changed := read(etag); status != http.StatusOK || changed == etag {
		t.Fatalf("a commit did not change the ETag: status=%d", status)
	}

	// The change was sent before the last change to gladiator.md, so Core rejects it.
	status, body = change(`"replacements":[{"path":"lists/movies/gladiator.md","oldText":"Commodus","newText":"Commodus the emperor"}]`)
	if status != http.StatusOK || !strings.Contains(body, `"applied":false`) || !strings.Contains(body, "changed this file after you sent your change") {
		t.Fatalf("status=%d body=%s", status, body)
	}

	sentAt = time.Now().UTC().Format(time.RFC3339Nano)
	status, body = change(`"replacements":[{"path":"wiki/does-not-exist.md","oldText":"a","newText":"b"}]`)
	if status != http.StatusOK || !strings.Contains(body, "the file does not exist") {
		t.Fatalf("status=%d body=%s", status, body)
	}

	if status, _ := request(http.MethodGet, "/page?path=../secret.md", ""); status != http.StatusNotFound {
		t.Fatalf("status=%d", status)
	}
}
