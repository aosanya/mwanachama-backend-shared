package dispatch_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

const callerSpecJSON = `{
  "base": "/v1",
  "operations": {
    "issue": {"method":"POST","path":"/things/{slug}/links","call":"Issue","action":"t.link.issue","status":201,
              "title":"Issue link","description":"Mint a key for one thing.",
              "args":[{"from":"path","as":"slug"},{"from":"caller","as":"label"}],
              "returns":[{"as":"link"},{"as":"key","once":true}]}
  },
  "errors": {}
}`

func callerDeps(t *testing.T, caller dispatch.Caller) (*dispatch.Spec, dispatch.Deps) {
	t.Helper()
	s, err := dispatch.Parse([]byte(callerSpecJSON))
	if err != nil {
		t.Fatalf("precondition: parse: %v", err)
	}
	return s, dispatch.Deps{Manager: &manager{}, Errors: map[string]error{}, Caller: caller}
}

func issuedKey(t *testing.T, body []byte) string {
	t.Helper()
	var out struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("precondition: decode %s: %v", body, err)
	}
	return out.Key
}

func TestACallerArgumentIsNeverReadFromTheRequest(t *testing.T) {
	for name, tc := range map[string]struct {
		caller dispatch.Caller
		want   string
	}{
		"a mount that names its caller": {func(context.Context) string { return "operator-a" }, "raw-operator-a"},
		"a mount that names nobody":     {nil, "raw-"},
	} {
		t.Run(name, func(t *testing.T) {
			s, d := callerDeps(t, tc.caller)
			routes, err := dispatch.Dispatch(s, d)
			if err != nil {
				t.Fatalf("precondition: dispatch: %v", err)
			}
			mux := http.NewServeMux()
			for _, r := range routes {
				mux.Handle(r.Method+" "+r.Path, r.Handler)
			}

			rec := do(t, mux, http.MethodPost, "/v1/things/alpha/links?label=forged-query", `{"label":"forged-body"}`)
			if rec.Code != http.StatusCreated {
				t.Fatalf("precondition: issue answered %d %s", rec.Code, rec.Body.String())
			}
			if got := issuedKey(t, rec.Body.Bytes()); got != tc.want {
				t.Errorf("the caller argument bound %q; it should bind %q whatever the query and body claim", got, tc.want)
			}
		})
	}
}

func TestACallerArgumentIsNeitherOfferedNorAcceptedByATool(t *testing.T) {
	s, d := callerDeps(t, func(context.Context) string { return "operator-a" })
	tools, err := dispatch.Tools(s, d)
	if err != nil || len(tools) != 1 {
		t.Fatalf("precondition: expected one tool, got %d (%v)", len(tools), err)
	}
	issue := tools[0]
	if strings.Contains(string(issue.InputSchema), `"label"`) {
		t.Errorf("the tool schema offers the caller argument: %s", issue.InputSchema)
	}
	if _, err := issue.Invoke(context.Background(), json.RawMessage(`{"slug":"alpha","label":"forged"}`)); err == nil {
		t.Error("a tool call supplying the caller argument was accepted; it should be refused")
	}
	out, err := issue.Invoke(context.Background(), json.RawMessage(`{"slug":"alpha"}`))
	if err != nil {
		t.Fatalf("precondition: invoke: %v", err)
	}
	raw, _ := json.Marshal(out)
	if got := issuedKey(t, raw); got != "raw-operator-a" {
		t.Errorf("the tool bound %q; it should bind the mount's caller", got)
	}
}

func TestARequiredCallerArgumentIsRefusedAtLoad(t *testing.T) {
	raw := strings.Replace(callerSpecJSON, `{"from":"caller","as":"label"}`, `{"from":"caller","as":"label","required":true}`, 1)
	if _, err := dispatch.Parse([]byte(raw)); err == nil {
		t.Error("a spec requiring a request to supply its caller argument loaded; it should be refused")
	}
}
