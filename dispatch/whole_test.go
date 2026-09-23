package dispatch_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

type filter struct {
	Visibility string   `query:"visibility"`
	Search     string   `query:"search"`
	Tags       []string `query:"tag"`
	Limit      int      `query:"limit"`
	Internal   string   `query:"-"`
}

type record struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type wholeManager struct {
	filter filter
	record record
}

func (m *wholeManager) Search(ctx context.Context, f filter) ([]record, error) {
	m.filter = f
	return []record{{Slug: "one"}}, nil
}

func (m *wholeManager) Put(ctx context.Context, rec record) (record, error) {
	m.record = rec
	return rec, nil
}

const wholeSpec = `{
  "operations": {
    "search": {"method":"GET","path":"/records","call":"Search","action":"t.record.search",
               "args":[{"from":"query","whole":true}],"returns":[{"body":true}]},
    "put":    {"method":"PUT","path":"/records","call":"Put","action":"t.record.put",
               "args":[{"from":"body","whole":true}],"returns":[{"body":true}]}
  }
}`

func buildWhole(t *testing.T) (http.Handler, *wholeManager) {
	t.Helper()
	s, err := dispatch.Parse([]byte(wholeSpec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	m := &wholeManager{}
	routes, err := dispatch.Dispatch(s, dispatch.Deps{Manager: m})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.Handle(r.Method+" "+r.Path, r.Handler)
	}
	return mux, m
}

func TestWholeQueryAssemblesAStruct(t *testing.T) {
	h, m := buildWhole(t)

	rec := do(t, h, http.MethodGet, "/records?visibility=public&search=hay&tag=a&tag=b&limit=5", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if m.filter.Visibility != "public" || m.filter.Search != "hay" || m.filter.Limit != 5 {
		t.Fatalf("filter = %+v, want visibility=public search=hay limit=5", m.filter)
	}
	if len(m.filter.Tags) != 2 || m.filter.Tags[0] != "a" || m.filter.Tags[1] != "b" {
		t.Fatalf("tags = %v, want [a b]", m.filter.Tags)
	}
}

func TestWholeQueryLeavesAbsentParametersZero(t *testing.T) {
	h, m := buildWhole(t)

	if rec := do(t, h, http.MethodGet, "/records", ""); rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if m.filter.Visibility != "" || m.filter.Limit != 0 || m.filter.Tags != nil {
		t.Fatalf("filter = %+v, want the zero value", m.filter)
	}
}

func TestWholeQuerySkipsAFieldTaggedOut(t *testing.T) {
	h, m := buildWhole(t)

	if rec := do(t, h, http.MethodGet, "/records?-=x&Internal=y", ""); rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if m.filter.Internal != "" {
		t.Fatalf("Internal = %q, want empty; a query:\"-\" field is not bindable", m.filter.Internal)
	}
}

func TestWholeBodyDecodesIntoTheParameter(t *testing.T) {
	h, m := buildWhole(t)

	rec := do(t, h, http.MethodPut, "/records", `{"slug":"alpha","name":"Alpha"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if m.record.Slug != "alpha" || m.record.Name != "Alpha" {
		t.Fatalf("record = %+v, want alpha/Alpha", m.record)
	}
}

func TestWholeBodyRefusesAnUnknownField(t *testing.T) {
	h, _ := buildWhole(t)

	rec := do(t, h, http.MethodPut, "/records", `{"slug":"alpha","nme":"typo"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; a misspelled field must not be silently dropped", rec.Code)
	}
}

func TestWholeRefusals(t *testing.T) {
	cases := map[string]string{
		"a whole argument that also names a field": `{"operations":{"a":{"method":"PUT","path":"/a","call":"Put",
			"action":"t.a.put","args":[{"from":"body","whole":true,"as":"slug"}],"returns":[{"body":true}]}}}`,
		"two whole arguments": `{"operations":{"a":{"method":"PUT","path":"/a","call":"Put",
			"action":"t.a.put","args":[{"from":"body","whole":true},{"from":"query","whole":true}],"returns":[{"body":true}]}}}`,
		"a whole path": `{"operations":{"a":{"method":"PUT","path":"/a","call":"Put",
			"action":"t.a.put","args":[{"from":"path","whole":true}],"returns":[{"body":true}]}}}`,
		"a whole repeated argument": `{"operations":{"a":{"method":"GET","path":"/a","call":"Put",
			"action":"t.a.put","args":[{"from":"query","whole":true,"repeated":true}],"returns":[{"body":true}]}}}`,
	}
	for what, raw := range cases {
		t.Run(what, func(t *testing.T) {
			if _, err := dispatch.Parse([]byte(raw)); err == nil {
				t.Errorf("accepted %s", what)
			}
		})
	}
}
