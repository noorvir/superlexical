// Package api serves a Superlexical workspace over HTTP. All data comes from
// the last valid commit of the workspace.
package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"superlexical/core"
	"superlexical/store"
)

// New returns the HTTP API for a workspace. The handler also serves the
// OpenAPI specification at /openapi.json and its documentation at /docs.
// The workspace can be nil when the caller needs only the specification.
func New(workspace *store.Store) (http.Handler, huma.API) {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Superlexical", "0.2.0"))

	huma.Register(api, huma.Operation{
		OperationID: "get-health",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "Report that the server is running",
	}, func(ctx context.Context, input *struct{}) (*healthOutput, error) {
		output := &healthOutput{}
		output.Body.Status = "ok"
		return output, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "list-lists",
		Method:      http.MethodGet,
		Path:        "/lists",
		Summary:     "List the lists in the workspace",
	}, func(ctx context.Context, input *struct{}) (*listsOutput, error) {
		folders, err := workspace.List("lists")
		if err != nil {
			return nil, internalError(ctx, "cannot read the lists", err)
		}

		output := &listsOutput{}
		output.Body.Lists = []list{}

		for _, folder := range folders {
			slug, isFolder := strings.CutSuffix(folder, "/")
			if !isFolder {
				continue
			}

			found, _, err := readList(workspace, slug)
			if err != nil {
				return nil, internalError(ctx, "cannot read the list "+slug, err)
			}

			output.Body.Lists = append(output.Body.Lists, found)
		}

		return output, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-list",
		Method:      http.MethodGet,
		Path:        "/lists/{list}",
		Summary:     "Get one list and its entries",
	}, func(ctx context.Context, input *struct {
		List string `path:"list"`
	}) (*listOutput, error) {
		found, entries, err := readList(workspace, input.List)
		if errors.Is(err, store.ErrNotFound) {
			return nil, huma.Error404NotFound("list not found")
		}
		if err != nil {
			return nil, internalError(ctx, "cannot read the list", err)
		}

		output := &listOutput{}
		output.Body.List = found
		output.Body.Entries = entries
		return output, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "get-page",
		Method:      http.MethodGet,
		Path:        "/page",
		Summary:     "Get one Markdown page",
	}, func(ctx context.Context, input *struct {
		Path string `query:"path" required:"true" doc:"Workspace-relative path of the Markdown file"`
	}) (*pageOutput, error) {
		found, err := readPage(workspace, input.Path)
		if errors.Is(err, store.ErrNotFound) {
			return nil, huma.Error404NotFound("page not found")
		}
		if err != nil {
			return nil, internalError(ctx, "cannot read the page", err)
		}

		output := &pageOutput{}
		output.Body = found
		return output, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "apply-change",
		Method:      http.MethodPost,
		Path:        "/changes",
		Summary:     "Apply a set of edits as one commit",
		Description: "Core applies one change at a time. It rejects a change when an edit does not fit the current files, when a file changed after the change was sent, or when the result is not valid. A rejected change modifies nothing.",
	}, func(ctx context.Context, input *struct {
		Body changeInput
	}) (*changeOutput, error) {
		change := store.Change{
			Author:  input.Body.Author,
			Message: input.Body.Message,
			SentAt:  input.Body.SentAt,
		}
		for _, create := range input.Body.Creates {
			change.Creates = append(change.Creates, store.Create{Path: create.Path, Content: create.Content})
		}
		for _, replacement := range input.Body.Replacements {
			edit := store.Replacement{Path: replacement.Path, OldText: replacement.OldText, NewText: replacement.NewText}
			change.Replacements = append(change.Replacements, edit)
		}

		result, err := workspace.Apply(change)
		if err != nil {
			return nil, internalError(ctx, "cannot apply the change", err)
		}

		output := &changeOutput{}
		output.Body.Applied = result.Commit != ""
		output.Body.Commit = result.Commit
		output.Body.Rejections = make([]rejection, len(result.Rejections))
		for i, r := range result.Rejections {
			output.Body.Rejections[i] = rejection{Path: r.Path, Message: r.Message}
		}

		output.Body.Diagnostics = make([]diagnostic, len(result.Diagnostics))
		for i, d := range result.Diagnostics {
			output.Body.Diagnostics[i] = diagnostic{Path: d.Path, Line: d.Line, Field: d.Field, Message: d.Message}
		}

		return output, nil
	})

	if workspace == nil {
		return mux, api
	}

	return revalidate(workspace, mux), api
}

// revalidate lets a client skip a read when the workspace did not change. It
// sets the hash of the last commit as the ETag of each list and page response.
// A request that sends the same hash in If-None-Match gets 304 Not Modified
// with no body, so a client can poll at low cost.
func revalidate(workspace *store.Store, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isRead := r.Method == http.MethodGet && (r.URL.Path == "/page" || strings.HasPrefix(r.URL.Path, "/lists"))
		if !isRead {
			next.ServeHTTP(w, r)
			return
		}

		etag := `"` + workspace.Head() + `"`
		w.Header().Set("ETag", etag)
		w.Header().Set("Cache-Control", "no-cache")

		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func readList(workspace *store.Store, slug string) (list, []entry, error) {
	folder := path.Join("lists", slug)
	index, err := readPage(workspace, path.Join(folder, core.ListIndex))
	if err != nil {
		return list{}, nil, err
	}

	names, err := workspace.List(folder)
	if err != nil {
		return list{}, nil, err
	}

	entries := []entry{}
	for _, name := range names {
		if name == core.ListIndex || !strings.HasSuffix(name, ".md") {
			continue
		}

		entryPath := path.Join(folder, name)
		found, err := readPage(workspace, entryPath)
		if err != nil {
			return list{}, nil, err
		}

		entries = append(entries, entry{Path: entryPath, Title: found.Title})
	}

	description := ""
	for _, field := range index.Fields {
		if field.Name == "description" {
			description = field.Value
		}
	}

	return list{Slug: slug, Name: index.Title, Description: description, EntryCount: len(entries)}, entries, nil
}

func readPage(workspace *store.Store, pagePath string) (page, error) {
	source, err := workspace.ReadFile(pagePath)
	if err != nil {
		return page{}, err
	}

	parsed, err := core.ParsePage(source)
	if err != nil {
		return page{}, err
	}

	found := page{Path: pagePath, Body: parsed.Body, Fields: []field{}}
	found.Title = strings.TrimSuffix(path.Base(pagePath), ".md")
	titled := false

	for _, item := range parsed.Fields {
		if item.Name == "schema" {
			continue
		}

		value := displayValue(item.Value)
		found.Fields = append(found.Fields, field{Name: item.Name, Value: value})

		if !titled && (item.Name == "title" || item.Name == "name") {
			found.Title = value
			titled = true
		}
	}

	return found, nil
}

func displayValue(value any) string {
	items, ok := value.([]any)
	if !ok {
		return fmt.Sprint(value)
	}

	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = fmt.Sprint(item)
	}

	return strings.Join(parts, ", ")
}

func internalError(ctx context.Context, message string, err error) error {
	slog.ErrorContext(ctx, message, "error", err)
	return huma.Error500InternalServerError(message)
}

type healthOutput struct {
	Body struct {
		Status string `json:"status" enum:"ok"`
	}
}

type listsOutput struct {
	Body struct {
		Lists []list `json:"lists" nullable:"false"`
	}
}

type listOutput struct {
	Body struct {
		List    list    `json:"list"`
		Entries []entry `json:"entries" nullable:"false"`
	}
}

type pageOutput struct {
	Body page
}

type changeInput struct {
	Author       string        `json:"author" minLength:"1" doc:"Agent or person that made the change"`
	Message      string        `json:"message" minLength:"1" doc:"Commit message"`
	SentAt       time.Time     `json:"sentAt" doc:"Time when the author sent the change"`
	Creates      []create      `json:"creates,omitempty" doc:"New Markdown files"`
	Replacements []replacement `json:"replacements,omitempty" doc:"Text replacements in existing Markdown files"`
}

type create struct {
	Path    string `json:"path" doc:"Workspace-relative path of the new file"`
	Content string `json:"content"`
}

type replacement struct {
	Path    string `json:"path" doc:"Workspace-relative path of an existing file"`
	OldText string `json:"oldText" minLength:"1" doc:"Text that occurs exactly once in the file"`
	NewText string `json:"newText"`
}

type changeOutput struct {
	Body struct {
		Applied     bool         `json:"applied" doc:"False when Core rejected the change; nothing was modified"`
		Commit      string       `json:"commit" doc:"Hash of the new commit; empty when Core rejected the change"`
		Rejections  []rejection  `json:"rejections" nullable:"false" doc:"Edits that do not fit the current files"`
		Diagnostics []diagnostic `json:"diagnostics" nullable:"false" doc:"Validation errors of the workspace with the change applied"`
	}
}

type rejection struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type list struct {
	Slug        string `json:"slug" doc:"Folder name of the list in lists/"`
	Name        string `json:"name"`
	Description string `json:"description"`
	EntryCount  int    `json:"entryCount"`
}

type entry struct {
	Path  string `json:"path" doc:"Workspace-relative path of the entry file"`
	Title string `json:"title"`
}

type page struct {
	Path   string  `json:"path" doc:"Workspace-relative path of the Markdown file"`
	Title  string  `json:"title" doc:"The title or name field; the file name when the page has neither"`
	Fields []field `json:"fields" nullable:"false" doc:"Front matter fields in source order"`
	Body   string  `json:"body" doc:"Markdown text after the front matter"`
}

type field struct {
	Name  string `json:"name"`
	Value string `json:"value" doc:"Display text of the value; list items are separated by commas"`
}

type diagnostic struct {
	Path    string `json:"path" doc:"Workspace-relative path of the source file"`
	Line    int    `json:"line" doc:"One-based line number"`
	Field   string `json:"field" doc:"Display label of the field; empty when the error is not about a field"`
	Message string `json:"message"`
}
