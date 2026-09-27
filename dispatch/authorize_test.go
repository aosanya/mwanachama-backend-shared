package dispatch_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

func authorizedDeps(t *testing.T, allow ...string) (*manager, dispatch.Deps, *[]string) {
	t.Helper()
	permitted := map[string]bool{}
	for _, a := range allow {
		permitted[a] = true
	}
	var asked []string
	m := &manager{}
	return m, dispatch.Deps{
		Manager: m,
		Errors:  map[string]error{"ErrMissing": errMissing, "ErrConflict": errConflict},
		Authorize: func(ctx context.Context, action string) error {
			asked = append(asked, action)
			if permitted[action] {
				return nil
			}
			return errors.New("nope")
		},
	}, &asked
}

func serve(t *testing.T, routes []dispatch.Route, method, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.HandleFunc(r.Method+" "+r.Path, r.Handler)
	}
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestAuthorize_RefusedRouteAnswers403AndNeverReachesTheManager(t *testing.T) {
	s, err := dispatch.Parse([]byte(specJSON))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	m, deps, asked := authorizedDeps(t, "t.thing.list")

	routes, err := dispatch.Dispatch(s, deps)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	rec := serve(t, routes, http.MethodGet, "/v1/things/abc", "")
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
	if m.opened != "" {
		t.Errorf("the manager was called anyway, with %q", m.opened)
	}
	if len(*asked) != 1 || (*asked)[0] != "t.thing.open" {
		t.Errorf("authorizer was asked %v, want the route's own action", *asked)
	}
}

func TestAuthorize_PermittedRouteRunsNormally(t *testing.T) {
	s, err := dispatch.Parse([]byte(specJSON))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	m, deps, _ := authorizedDeps(t, "t.thing.open")

	routes, err := dispatch.Dispatch(s, deps)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	rec := serve(t, routes, http.MethodGet, "/v1/things/abc", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if m.opened != "abc" {
		t.Errorf("the manager was not reached, opened = %q", m.opened)
	}
}

func TestAuthorize_NilAuthorizerChangesNothing(t *testing.T) {
	s, err := dispatch.Parse([]byte(specJSON))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	m := &manager{}
	routes, err := dispatch.Dispatch(s, dispatch.Deps{
		Manager: m,
		Errors:  map[string]error{"ErrMissing": errMissing, "ErrConflict": errConflict},
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	rec := serve(t, routes, http.MethodGet, "/v1/things/abc", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("a module mounting without an authorizer must behave as before, got %d", rec.Code)
	}
	if m.opened != "abc" {
		t.Errorf("opened = %q, want abc", m.opened)
	}
}

func TestAuthorize_RefusalDoesNotSayWhichActionWasRefused(t *testing.T) {
	s, err := dispatch.Parse([]byte(specJSON))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, deps, _ := authorizedDeps(t)

	routes, err := dispatch.Dispatch(s, deps)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}

	rec := serve(t, routes, http.MethodGet, "/v1/things/abc", "")
	if body := rec.Body.String(); strings.Contains(body, "t.thing.open") || strings.Contains(body, "nope") {
		t.Errorf("the refusal leaks what was asked or why: %s", body)
	}
}

func TestAuthorize_GatesToolsAsWellAsRoutes(t *testing.T) {
	s, err := dispatch.Parse([]byte(toolSpec))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	m := &toolManager{}
	var asked []string
	deps := dispatch.Deps{
		Manager: m,
		Errors:  map[string]error{"ErrMissing": errMissing},
		Authorize: func(ctx context.Context, action string) error {
			asked = append(asked, action)
			if action == "t.note.find" {
				return nil
			}
			return errors.New("nope")
		},
	}

	tools, err := dispatch.Tools(s, deps)
	if err != nil {
		t.Fatalf("Tools: %v", err)
	}

	byAction := map[string]dispatch.Tool{}
	for _, tool := range tools {
		byAction[tool.Action] = tool
	}

	refused, ok := byAction["t.note.read"]
	if !ok {
		t.Fatal("no tool for t.note.read")
	}
	if _, err := refused.Invoke(context.Background(), json.RawMessage(`{"slug":"abc"}`)); !errors.Is(err, dispatch.ErrForbidden) {
		t.Errorf("Invoke error = %v, want ErrForbidden", err)
	}

	permitted, ok := byAction["t.note.find"]
	if !ok {
		t.Fatal("no tool for t.note.find")
	}
	if _, err := permitted.Invoke(context.Background(), json.RawMessage(`{}`)); err != nil {
		t.Errorf("a permitted tool should run, got %v", err)
	}
	if len(asked) != 2 {
		t.Errorf("authorizer was asked %v, want one question per invoke", asked)
	}
}

func TestAuthorize_IsAskedBeforeTheArgumentsAreEvenRead(t *testing.T) {
	s, err := dispatch.Parse([]byte(toolSpec))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	var asked []string
	deps := dispatch.Deps{
		Manager: &toolManager{},
		Errors:  map[string]error{"ErrMissing": errMissing},
		Authorize: func(ctx context.Context, action string) error {
			asked = append(asked, action)
			return errors.New("nope")
		},
	}

	tools, err := dispatch.Tools(s, deps)
	if err != nil {
		t.Fatalf("Tools: %v", err)
	}
	for _, tool := range tools {
		if tool.Action != "t.note.read" {
			continue
		}
		if _, err := tool.Invoke(context.Background(), json.RawMessage(`{"nonsense":1}`)); !errors.Is(err, dispatch.ErrForbidden) {
			t.Errorf("a caller with no permission should be refused before its arguments are judged, got %v", err)
		}
	}
	if len(asked) != 1 {
		t.Errorf("authorizer asked %v", asked)
	}
}
