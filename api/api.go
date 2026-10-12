// Package api serves a Superlexical workspace over HTTP.
package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"superlexical/core"
)

// New returns the HTTP API for the workspace at root. The handler also serves
// the OpenAPI specification at /openapi.json and its documentation at /docs.
func New(root string) (http.Handler, huma.API) {
	mux := http.NewServeMux()
	api := humago.New(mux, huma.DefaultConfig("Superlexical", "0.1.0"))

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
		OperationID: "list-workspace-diagnostics",
		Method:      http.MethodGet,
		Path:        "/workspace/diagnostics",
		Summary:     "Validate the workspace and list its errors",
	}, func(ctx context.Context, input *struct{}) (*diagnosticsOutput, error) {
		found, err := core.CompileWorkspace(root)
		if err != nil {
			slog.ErrorContext(ctx, "cannot validate the workspace", "root", root, "error", err)
			return nil, huma.Error500InternalServerError("cannot validate the workspace")
		}

		output := &diagnosticsOutput{}
		output.Body.Diagnostics = make([]diagnostic, len(found))
		for i, d := range found {
			output.Body.Diagnostics[i] = diagnostic{Path: d.Path, Line: d.Line, Field: d.Field, Message: d.Message}
		}

		return output, nil
	})

	return mux, api
}

type healthOutput struct {
	Body struct {
		Status string `json:"status" enum:"ok"`
	}
}

type diagnosticsOutput struct {
	Body struct {
		Diagnostics []diagnostic `json:"diagnostics" nullable:"false"`
	}
}

type diagnostic struct {
	Path    string `json:"path" doc:"Workspace-relative path of the source file"`
	Line    int    `json:"line" doc:"One-based line number"`
	Field   string `json:"field" doc:"Display label of the field; empty when the error is not about a field"`
	Message string `json:"message"`
}
