package dispatch_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

// Pins board row S17 (documentation/3. implementation/todo.md): the HTTP
// route surface does not enforce Arg.Required, even though the MCP tool
// surface built from the identical spec does. Once S17's fix lands (bind
// checking Arg.Required the way tools.go's Invoke already does), this
// request should be refused with 400, and the assertion below must invert.
func TestS17_HTTPRouteIgnoresRequiredWhileMCPToolEnforcesIt(t *testing.T) {
	s, err := dispatch.Parse([]byte(toolSpec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m := &toolManager{}

	tools, err := dispatch.Tools(s, dispatch.Deps{
		Manager: m,
		Errors:  map[string]error{"ErrMissing": errMissing},
		Fields:  noteFields,
	})
	if err != nil {
		t.Fatalf("tools: %v", err)
	}
	var issueTool dispatch.Tool
	for _, tl := range tools {
		if tl.Name == "t_link_issue" {
			issueTool = tl
		}
	}
	if _, toolErr := call(t, issueTool, `{"slug":"one"}`); toolErr == nil {
		t.Fatalf("MCP tool surface should refuse a call missing required 'label'")
	}

	routes, err := dispatch.Dispatch(s, dispatch.Deps{
		Manager: m,
		Errors:  map[string]error{"ErrMissing": errMissing},
	})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.Handle(r.Pattern(""), r.Handler)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resp, err := http.Post(srv.URL+"/notes/one/links", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("S17 pin: expected the current (broken) behaviour of 200 OK, got %d -- "+
			"if this is now a 400, S17 is fixed and this test should be inverted/removed", resp.StatusCode)
	}
}
