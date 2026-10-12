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

func toolSpecWith(t *testing.T, op, tool string) string {
	t.Helper()
	needle := `"` + op + `": {`
	if !strings.Contains(toolSpec, needle) {
		needle = `"` + op + `":  {`
	}
	if !strings.Contains(toolSpec, needle) {
		needle = `"` + op + `":   {`
	}
	if !strings.Contains(toolSpec, needle) {
		t.Fatalf("no operation %q in the tool spec", op)
	}
	return strings.Replace(toolSpec, needle, needle+`"tool":`+tool+`,`, 1)
}

func toolsAndRoutes(t *testing.T, raw string) (map[string]dispatch.Tool, *toolManager, http.Handler) {
	t.Helper()
	s, err := dispatch.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m := &toolManager{}
	deps := dispatch.Deps{Manager: m, Errors: map[string]error{"ErrMissing": errMissing}, Fields: noteFields}
	tools, err := dispatch.Tools(s, deps)
	if err != nil {
		t.Fatalf("tools: %v", err)
	}
	byName := map[string]dispatch.Tool{}
	for _, tool := range tools {
		byName[tool.Name] = tool
	}
	routes, err := dispatch.Dispatch(s, deps)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.Handle(r.Pattern(""), r.Handler)
	}
	return byName, m, mux
}

func TestAnOperationOptedOutOfToolsPublishesNoTool(t *testing.T) {
	all, _, _ := toolsAndRoutes(t, toolSpec)
	tools, _, _ := toolsAndRoutes(t, toolSpecWith(t, "forget", "false"))
	if _, ok := tools["t_note_forget"]; ok {
		t.Fatalf("an operation declared \"tool\": false still published a tool")
	}
	if len(tools) != len(all)-1 {
		t.Fatalf("opting one operation out moved the tool count from %d to %d", len(all), len(tools))
	}
	for name := range all {
		if _, ok := tools[name]; !ok && name != "t_note_forget" {
			t.Fatalf("opting forget out also removed %s", name)
		}
	}
}

func TestAnOperationOptedOutOfToolsStillAnswersOverREST(t *testing.T) {
	_, m, mux := toolsAndRoutes(t, toolSpecWith(t, "forget", "false"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/notes/old", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if m.forgot != "old" {
		t.Fatalf("the route did not reach the manager: forgot %q", m.forgot)
	}
}

func TestAnOperationOptedOutOfToolsNeedsNoToolProse(t *testing.T) {
	raw := strings.Replace(toolSpecWith(t, "forget", "false"), `"description":"Delete one note.",`, "", 1)
	tools, _, _ := toolsAndRoutes(t, raw)
	if _, ok := tools["t_note_forget"]; ok {
		t.Fatalf("an opted-out operation published a tool")
	}
}

func TestAnOperationOptedOutOfToolsIsStillGated(t *testing.T) {
	s, err := dispatch.Parse([]byte(toolSpecWith(t, "forget", "false")))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m := &toolManager{}
	var asked []string
	routes, err := dispatch.Dispatch(s, dispatch.Deps{
		Manager: m,
		Errors:  map[string]error{"ErrMissing": errMissing},
		Authorize: func(_ context.Context, action string) error {
			asked = append(asked, action)
			return dispatch.ErrForbidden
		},
	})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.Handle(r.Pattern(""), r.Handler)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/notes/old", nil))
	if rec.Code != http.StatusForbidden || m.forgot != "" {
		t.Fatalf("status = %d, forgot %q: an opted-out operation escaped its gate", rec.Code, m.forgot)
	}
	if len(asked) != 1 || asked[0] != "t.note.forget" {
		t.Fatalf("authorizer asked about %v", asked)
	}
}

func TestToolFalseBesideAToolNameIsRefused(t *testing.T) {
	s, err := dispatch.Parse([]byte(toolSpec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	op := s.Operations["forget"]
	op.Tool, op.NoTool = "t_forget", true
	s.Operations["forget"] = op
	err = s.Validate()
	if err == nil || !strings.Contains(err.Error(), "opts out of the tool surface") {
		t.Fatalf("validate = %v", err)
	}
}

func TestToolIsANameOrFalse(t *testing.T) {
	for _, bad := range []string{"true", "7", `{"name":"x"}`} {
		if _, err := dispatch.Parse([]byte(toolSpecWith(t, "forget", bad))); err == nil {
			t.Fatalf(`"tool": %s was accepted`, bad)
		}
	}
	s, err := dispatch.Parse([]byte(toolSpecWith(t, "forget", `"t_drop"`)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if op := s.Operations["forget"]; op.Tool != "t_drop" || op.NoTool {
		t.Fatalf("a named tool decoded as %+v", op)
	}
	s, err = dispatch.Parse([]byte(toolSpecWith(t, "forget", "null")))
	if err != nil || s.Operations["forget"].NoTool || s.Operations["forget"].Tool != "" {
		t.Fatalf("a null tool decoded as %+v, %v", s.Operations["forget"], err)
	}
}

func TestToolFalseSurvivesARoundTrip(t *testing.T) {
	s, err := dispatch.Parse([]byte(toolSpecWith(t, "forget", "false")))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	raw, err := json.Marshal(s.Operations["forget"])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"tool":false`) {
		t.Fatalf("marshalled without the opt-out: %s", raw)
	}
	var back dispatch.Operation
	if err := json.Unmarshal(raw, &back); err != nil || !back.NoTool {
		t.Fatalf("round trip lost the opt-out: %+v, %v", back, err)
	}
	plain, _ := json.Marshal(s.Operations["read"])
	if strings.Contains(string(plain), `"tool"`) {
		t.Fatalf("an operation that never set tool marshals one: %s", plain)
	}
}
