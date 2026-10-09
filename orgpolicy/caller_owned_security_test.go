package orgpolicy_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/orgpolicy"
)

func TestPolicyWriteTakesItsEditorFromTheCallerAndRefusesASmuggledSecondValue(t *testing.T) {
	mux := http.NewServeMux()
	for _, rt := range orgpolicy.Routes(newTestStore(t, nil), func(*http.Request) string { return "alice" }) {
		mux.HandleFunc(rt.Pattern("/v1/org-policy"), rt.Handler)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	call := func(method, body string) (int, map[string]any) {
		t.Helper()
		req, err := http.NewRequest(method, srv.URL+"/v1/org-policy", strings.NewReader(body))
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

	if status, out := call(http.MethodPut, `{"public_address_cap":7}`); status != http.StatusOK || out["updated_by"] != "alice" {
		t.Fatalf("precondition: PUT answered %d %v, want 200 recorded as alice", status, out)
	}

	for _, body := range []string{
		`{"public_address_cap":3,"updated_by":"mallory"}`,
		`{"public_address_cap":3,"updated_at":"1999-01-01T00:00:00Z"}`,
	} {
		status, out := call(http.MethodPut, body)
		if out["updated_by"] == "mallory" || strings.HasPrefix(fmtAny(out["updated_at"]), "1999") {
			t.Fatalf("PUT %s answered %d %v: a body-set editor or clock was recorded", body, status, out)
		}
		if status == http.StatusOK {
			call(http.MethodPut, `{"public_address_cap":7}`)
		}
	}

	for _, body := range []string{
		`{"public_address_cap":3} {"public_address_cap":-9}`,
		`{"public_address_cap":3,"PUBLIC_ADDRESS_CAP":-9}`,
		`{"Public_Address_Cap":-9}`,
	} {
		if status, out := call(http.MethodPut, body); status != http.StatusBadRequest {
			t.Fatalf("PUT %s answered %d %v, want 400", body, status, out)
		}
	}

	status, out := call(http.MethodGet, "")
	if status != http.StatusOK || out["public_address_cap"] != float64(7) || out["updated_by"] != "alice" {
		t.Fatalf("after the refused writes GET answered %d %v, want cap 7 recorded as alice", status, out)
	}
}

func fmtAny(v any) string { return fmt.Sprint(v) }
