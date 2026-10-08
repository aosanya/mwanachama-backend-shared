package dispatch_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

type roleCodeFilter struct {
	RoleCode string `json:"role_code,omitempty"`
}

type roleCodeManager struct{ seen roleCodeFilter }

func (m *roleCodeManager) Search(ctx context.Context, f roleCodeFilter) ([]string, error) {
	m.seen = f
	return nil, nil
}

const roleCodeSpec = `{
  "operations": {
    "search": {"method":"GET","path":"/records","call":"Search","action":"t.record.search",
               "args":[{"from":"query","whole":true}],"returns":[{"body":true}]}
  }
}`

func TestS54_OpenBug_UntaggedQueryFieldBindsOnlyAllLowercaseNotSnakeCase(t *testing.T) {
	s, err := dispatch.Parse([]byte(roleCodeSpec))
	if err != nil {
		t.Fatal(err)
	}
	m := &roleCodeManager{}
	routes, err := dispatch.Dispatch(s, dispatch.Deps{Manager: m})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.Handle(r.Method+" "+r.Path, r.Handler)
	}

	if rec := do(t, mux, http.MethodGet, "/records?role_code=writer", ""); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}

	if m.seen.RoleCode != "writer" {
		t.Fatalf("S54 open bug: GET /records?role_code=writer left RoleCode = %q, want %q — "+
			"a field with no explicit `query` tag falls back to strings.ToLower(f.Name) ('rolecode'), "+
			"not the snake_case every multi-word field uses on the wire elsewhere in this stack "+
			"(the field's own json tag is %q), so a caller spelling the parameter the way every "+
			"other query/body field on this engine is spelled is silently ignored", m.seen.RoleCode, "writer", `role_code,omitempty`)
	}
}
