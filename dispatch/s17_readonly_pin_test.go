package dispatch_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

type s17Record struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	Name      string `json:"name"`
}

type s17Manager struct {
	got s17Record
}

func (m *s17Manager) Create(ctx context.Context, rec s17Record) (s17Record, error) {
	m.got = rec
	return rec, nil
}

const s17Spec = `{
  "operations": {
    "create": {"method":"POST","path":"/records","call":"Create","action":"t.record.create",
               "description":"create a record",
               "args":[{"from":"body","whole":true}],
               "returns":[{"body":true}]}
  }
}`

func s17Deps(m *s17Manager) dispatch.Deps {
	return dispatch.Deps{
		Manager: m,
		Fields: map[string]dispatch.FieldDoc{
			"s17Record.id":         {ReadOnly: true, Description: "server-minted id"},
			"s17Record.created_at": {ReadOnly: true, Description: "server-stamped time"},
			"s17Record.name":       {Description: "the record's name"},
		},
	}
}

// TestPinsS17_MCPRefusesAReadOnlyFieldTheHTTPRouteAccepts pins board row S17:
// the same Spec+Deps, including the same Fields ReadOnly map, is handed to
// both dispatch.Tools (MCP) and dispatch.Dispatch (HTTP). MCP's Invoke
// correctly refuses a caller-supplied "id"/"created_at" ("takes no argument
// %q") because bindTool checks against the props flatten() already excluded
// for ReadOnly fields. dispatch.Dispatch's own bind()/wholeBody() never
// receives Deps.Fields at all (grep dispatch.go: Fields is referenced only
// in the Deps struct's own field declaration), so the HTTP route built from
// the identical Deps has no such check and forwards a forged id/created_at
// straight to the manager. This test pins the current (broken) HTTP-side
// behaviour: once S17 is fixed, the manager must stop receiving
// "attacker-chosen-id"/"1999-01-01T00:00:00Z" over HTTP the same way the MCP
// tool already refuses them, and this assertion should go red (status should
// no longer be 200 with the forged values echoed back, or the route should
// refuse the request the way MCP does).
func TestPinsS17_MCPRefusesAReadOnlyFieldTheHTTPRouteAccepts(t *testing.T) {
	s, err := dispatch.Parse([]byte(s17Spec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	mcpMgr := &s17Manager{}
	tools, err := dispatch.Tools(s, s17Deps(mcpMgr))
	if err != nil {
		t.Fatalf("tools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("tools = %d, want 1", len(tools))
	}
	if strings.Contains(string(tools[0].InputSchema), `"id"`) {
		t.Fatalf("MCP schema leaks the read-only id field: %s", tools[0].InputSchema)
	}
	_, mcpErr := tools[0].Invoke(context.Background(), json.RawMessage(
		`{"id":"attacker-chosen-id","created_at":"1999-01-01T00:00:00Z","name":"Alpha"}`))
	if mcpErr == nil {
		t.Fatalf("MCP invoke with a forged id/created_at unexpectedly succeeded")
	}
	if !strings.Contains(mcpErr.Error(), `"id"`) {
		t.Fatalf("MCP invoke error = %q, want it naming the refused id argument", mcpErr.Error())
	}

	httpMgr := &s17Manager{}
	routes, err := dispatch.Dispatch(s, s17Deps(httpMgr))
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.Handle(r.Method+" "+r.Path, r.Handler)
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/records", "application/json",
		strings.NewReader(`{"id":"attacker-chosen-id","created_at":"1999-01-01T00:00:00Z","name":"Alpha"}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	// S17: today this succeeds and the manager receives the forged id and
	// created_at verbatim, the exact injection MCP's ReadOnly check refuses.
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (pinning today's broken behaviour)", resp.StatusCode)
	}
	if httpMgr.got.ID != "attacker-chosen-id" {
		t.Fatalf("manager.got.ID = %q, want the forged id (pinning today's broken behaviour)", httpMgr.got.ID)
	}
	if httpMgr.got.CreatedAt != "1999-01-01T00:00:00Z" {
		t.Fatalf("manager.got.CreatedAt = %q, want the forged timestamp (pinning today's broken behaviour)", httpMgr.got.CreatedAt)
	}
}
