package dispatch_test

import (
	"context"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

const handledOps = `{
  "operations": {
    "list_things": {
      "method": "GET",
      "path": "/things",
      "call": "ListThings",
      "action": "demo.thing.list",
      "description": "Every thing.",
      "status": 200,
      "returns": [{"body": true}]
    },
    "sign_in": {
      "method": "POST",
      "path": "/signin",
      "handled": true,
      "action": "demo.session.open",
      "description": "Open a session, which is several calls in a fixed order.",
      "status": 201
    }
  }
}`

type handledManager struct{}

func (handledManager) ListThings(ctx context.Context) ([]string, error) { return nil, nil }

func TestAHandledOperationIsDeclaredButNotDispatched(t *testing.T) {
	s, err := dispatch.Parse([]byte(handledOps))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	handled := dispatch.Handled(s)
	if len(handled) != 1 || handled[0].Action != "demo.session.open" {
		t.Fatalf("Handled = %+v, want only the declared-but-bound address", handled)
	}

	routes, err := dispatch.Dispatch(s, dispatch.Deps{Manager: handledManager{}})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	for _, r := range routes {
		if r.Action == "demo.session.open" {
			t.Fatal("a handled operation was dispatched; the module binds it, and a nil handler would 500")
		}
	}
	if len(routes) != 1 {
		t.Fatalf("got %d routes, want only the one that names a call", len(routes))
	}

	shape := dispatch.Shape(s)
	if len(shape) != 2 {
		t.Fatalf("Shape lists %d addresses, want both — a handled address is still part of the table", len(shape))
	}
}

func TestAnOperationIsEitherCalledOrHandled(t *testing.T) {
	for _, tc := range []struct{ name, op, want string }{
		{
			"neither",
			`{"method":"GET","path":"/x","action":"demo.thing.list","description":"d","returns":[{"body":true}]}`,
			"does not declare that the module handles it",
		},
		{
			"both",
			`{"method":"GET","path":"/x","call":"ListThings","handled":true,"action":"demo.thing.list","description":"d","returns":[{"body":true}]}`,
			"it is one or the other",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := dispatch.Parse([]byte(`{"operations":{"only":` + tc.op + `}}`))
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to name %q", err, tc.want)
			}
		})
	}
}

func TestAHandledOperationIsNotAnMCPTool(t *testing.T) {
	s, err := dispatch.Parse([]byte(handledOps))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tools, err := dispatch.Tools(s, dispatch.Deps{Manager: handledManager{}})
	if err != nil {
		t.Fatalf("tools: %v", err)
	}
	for _, tool := range tools {
		if strings.Contains(tool.Name, "session") {
			t.Fatalf("a handled operation became the tool %q, which has no call behind it", tool.Name)
		}
	}
}
