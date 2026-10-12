package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWorkspaceDiagnostics(t *testing.T) {
	handler, _ := New("../core/testdata/workspace")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/workspace/diagnostics", nil))

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"diagnostics":[]`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body)
	}
}

func TestWorkspaceDiagnosticsMissingRoot(t *testing.T) {
	handler, _ := New("does-not-exist")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/workspace/diagnostics", nil))

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", response.Code, response.Body)
	}
}
