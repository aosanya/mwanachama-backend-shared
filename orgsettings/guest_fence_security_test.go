package orgsettings_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/orgsettings"
)

func TestGuestRoutesNeverCarryAnUndeclaredAttributeOrABodySetSlug(t *testing.T) {
	mux := http.NewServeMux()
	for _, rt := range orgsettings.Routes(newTestStore(t), "acme") {
		mux.HandleFunc(rt.Pattern("/v1/org-settings"), rt.Handler)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	call := func(method, path, body string) (int, map[string]any) {
		t.Helper()
		req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		var out map[string]any
		_ = json.Unmarshal(raw, &out)
		return res.StatusCode, out
	}

	status, stored := call(http.MethodPut, "/v1/org-settings/acme",
		`{"slug":"other","attributes":{"display_name":"Acme","internal_note":"never for guests","owner_phone":"+254700000000"}}`)
	if status != http.StatusOK {
		t.Fatalf("precondition: PUT answered %d, want 200", status)
	}
	if attrs, _ := stored["attributes"].(map[string]any); attrs["internal_note"] != "never for guests" {
		t.Fatalf("precondition: the undeclared attribute was not stored, got %v", stored)
	}
	if stored["slug"] != "acme" {
		t.Fatalf("a body slug moved the write: stored under %v, want the path's acme", stored["slug"])
	}

	if status, _ := call(http.MethodGet, "/v1/org-settings/other", ""); status != http.StatusNotFound {
		t.Fatalf("GET /other answered %d after a PUT naming it only in the body, want 404", status)
	}

	for _, path := range []string{"/v1/org-settings", "/v1/org-settings/acme"} {
		status, guest := call(http.MethodGet, path, "")
		if status != http.StatusOK || guest["display_name"] != "Acme" {
			t.Fatalf("GET %s answered %d %v, want 200 carrying display_name Acme", path, status, guest)
		}
		for _, key := range []string{"attributes", "internal_note", "owner_phone"} {
			if _, leaked := guest[key]; leaked {
				t.Fatalf("GET %s, a guest route, carried %q: %v", path, key, guest)
			}
		}
	}
}
