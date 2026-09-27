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

type Widget struct {
	Code string `json:"code"`
}

type WidgetFilter struct {
	WidgetCode string `json:"widget_code"`
}

type widgetManager struct {
	widgets []Widget
}

func (m *widgetManager) ListWidgets(ctx context.Context, f WidgetFilter) ([]Widget, error) {
	if f.WidgetCode == "" {
		return m.widgets, nil
	}
	var out []Widget
	for _, w := range m.widgets {
		if w.Code == f.WidgetCode {
			out = append(out, w)
		}
	}
	return out, nil
}

const listWidgetsOpsJSON = `{
  "base": "/v1/widgets",
  "operations": {
    "list_widgets": {
      "method": "GET", "path": "/widgets", "call": "ListWidgets", "action": "inventory.widget.list",
      "description": "List widgets, optionally filtered by widget_code.",
      "args": [{"from": "query", "whole": true}],
      "returns": [{"body": true}]
    }
  }
}`

func TestS24_OpenBug_QueryKeyIgnoresTheFieldsJSONTag(t *testing.T) {
	mgr := &widgetManager{widgets: []Widget{{Code: "a"}, {Code: "b"}}}

	ds, err := dispatch.Parse([]byte(listWidgetsOpsJSON))
	if err != nil {
		t.Fatalf("parse ops: %v", err)
	}
	routes, err := dispatch.Dispatch(ds, dispatch.Deps{
		Manager:   mgr,
		Authorize: func(context.Context, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	mux := http.NewServeMux()
	for _, rt := range routes {
		mux.Handle(rt.Pattern(""), rt.Handler)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/v1/widgets/widgets?widget_code=a")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	var got []Widget
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	t.Logf("GET ?widget_code=a -> status=%d widgets=%+v", resp.StatusCode, got)

	if len(got) != 1 || got[0].Code != "a" {
		t.Errorf("S24: GET /widgets?widget_code=a (the field's own json tag) answered %d widget(s) %+v, want exactly the 1 widget with code %q — wholeQuery's queryKey() derives the query parameter name from the lowercased Go field name (%q), not the json tag (%q), so a caller using the field's documented name gets an unfiltered list instead of a refusal or a match",
			len(got), got, "a", "widgetcode", "widget_code")
	}
}

type Thing struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type thingManager struct {
	things map[string]*Thing
}

func (m *thingManager) Rename(ctx context.Context, id string, name string) (Thing, error) {
	th := m.things[id]
	th.Name = name
	return *th, nil
}

const renameThingOpsJSON = `{
  "base": "/v1/things",
  "operations": {
    "rename_thing": {
      "method": "PATCH", "path": "/things/{id}", "call": "Rename", "action": "inventory.thing.rename",
      "description": "Rename a thing. name is required.",
      "args": [
        {"from": "path", "as": "id"},
        {"from": "body", "as": "name", "required": true}
      ],
      "returns": [{"body": true}]
    }
  }
}`

func TestS25_OpenBug_BindNeverEnforcesArgRequired(t *testing.T) {
	mgr := &thingManager{things: map[string]*Thing{"t1": {ID: "t1", Name: "original-name"}}}

	ds, err := dispatch.Parse([]byte(renameThingOpsJSON))
	if err != nil {
		t.Fatalf("parse ops: %v", err)
	}
	routes, err := dispatch.Dispatch(ds, dispatch.Deps{
		Manager:   mgr,
		Authorize: func(context.Context, string) error { return nil },
	})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	mux := http.NewServeMux()
	for _, rt := range routes {
		mux.Handle(rt.Pattern(""), rt.Handler)
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodPatch, srv.URL+"/v1/things/things/t1", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("patch: %v", err)
	}
	defer resp.Body.Close()
	var got Thing
	json.NewDecoder(resp.Body).Decode(&got)
	t.Logf("PATCH {} (missing required \"name\") -> status=%d thing=%+v", resp.StatusCode, got)

	if resp.StatusCode == http.StatusOK && got.Name == "" {
		t.Errorf("S25: PATCH /things/t1 with a body missing the declared-required %q field answered %d and silently renamed the thing to %q, want the request refused before the manager was ever called — the op declares {\"from\":\"body\",\"as\":\"name\",\"required\":true}, but bind()/bindOne() (dispatch.go) never reads Arg.Required at all: a missing named body key falls through to reflect.Zero(want) with no error, unlike tools.go's MCP invocation path, which does raise a needs-%q error for the identical declaration",
			"name", resp.StatusCode, got.Name, "name")
	}
}
