package httpwire_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/httpwire"
)

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	httpwire.WriteJSON(w, http.StatusCreated, map[string]string{"id": "abc"})

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
	if got := strings.TrimSpace(w.Body.String()); got != `{"id":"abc"}` {
		t.Errorf("body = %q, want %q", got, `{"id":"abc"}`)
	}
}

func TestWriteErr(t *testing.T) {
	w := httptest.NewRecorder()
	httpwire.WriteErr(w, http.StatusBadRequest, "bad request")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if got := strings.TrimSpace(w.Body.String()); got != `{"error":"bad request"}` {
		t.Errorf("body = %q, want %q", got, `{"error":"bad request"}`)
	}
}

func TestReadJSON(t *testing.T) {
	var out struct {
		Name string `json:"name"`
	}
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"gear"}`))
	if err := httpwire.ReadJSON(r, &out); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if out.Name != "gear" {
		t.Errorf("Name = %q, want %q", out.Name, "gear")
	}
}

func TestReadJSON_RejectsUnknownFields(t *testing.T) {
	var out struct {
		Name string `json:"name"`
	}
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"gear","extra":true}`))
	if err := httpwire.ReadJSON(r, &out); err == nil {
		t.Fatal("ReadJSON with unknown field: got nil error, want one")
	}
}

func TestReadJSONFields(t *testing.T) {
	var out struct {
		Name  string  `json:"name"`
		Email *string `json:"email"`
	}
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"gear","email":null}`))
	present, err := httpwire.ReadJSONFields(r, &out)
	if err != nil {
		t.Fatalf("ReadJSONFields: %v", err)
	}
	if !present["name"] || !present["email"] {
		t.Errorf("present = %v, want name and email both true", present)
	}
	if present["missing"] {
		t.Error("present[\"missing\"] = true, want false")
	}
}

func TestBearer(t *testing.T) {
	cases := []struct {
		header string
		want   string
	}{
		{"", ""},
		{"Bearer abc123", "abc123"},
		{"abc123", "abc123"},
	}
	for _, c := range cases {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if c.header != "" {
			r.Header.Set("Authorization", c.header)
		}
		if got := httpwire.Bearer(r); got != c.want {
			t.Errorf("Bearer(%q) = %q, want %q", c.header, got, c.want)
		}
	}
}

func TestRoute_Pattern(t *testing.T) {
	rt := httpwire.Route{Method: http.MethodPost, Path: "/{id}"}
	if got, want := rt.Pattern("/widgets"), "POST /widgets/{id}"; got != want {
		t.Errorf("Pattern = %q, want %q", got, want)
	}
}

func TestStatusFor(t *testing.T) {
	errNotFound := errors.New("not found")
	errConflict := errors.New("conflict")
	table := map[error]int{
		errNotFound: http.StatusNotFound,
		errConflict: http.StatusConflict,
	}

	if got := httpwire.StatusFor(errNotFound, table, http.StatusInternalServerError); got != http.StatusNotFound {
		t.Errorf("StatusFor(errNotFound) = %d, want %d", got, http.StatusNotFound)
	}
	if got := httpwire.StatusFor(errors.New("other"), table, http.StatusInternalServerError); got != http.StatusInternalServerError {
		t.Errorf("StatusFor(other) = %d, want %d (fallback)", got, http.StatusInternalServerError)
	}

	wrapped := fmt.Errorf("wrapping: %w", errConflict)
	if got := httpwire.StatusFor(wrapped, table, http.StatusInternalServerError); got != http.StatusConflict {
		t.Errorf("StatusFor(wrapped errConflict) = %d, want %d", got, http.StatusConflict)
	}
}
