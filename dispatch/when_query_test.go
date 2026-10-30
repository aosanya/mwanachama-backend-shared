package dispatch_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

type shelf struct {
	called string
	arg    string
}

func (s *shelf) ListRepos(ctx context.Context) ([]string, error) {
	s.called, s.arg = "ListRepos", ""
	return []string{"all"}, nil
}

func (s *shelf) RepoByName(ctx context.Context, name string) (string, error) {
	s.called, s.arg = "RepoByName", name
	return name, nil
}

func (s *shelf) ReposByRun(ctx context.Context, run string) ([]string, error) {
	s.called, s.arg = "ReposByRun", run
	return []string{run}, nil
}

const selectedSpec = `{"operations":{
  "list_repos":{"method":"GET","path":"/repos","call":"ListRepos","action":"s.repo.list",
    "returns":[{"body":true}]},
  "repo_by_name":{"method":"GET","path":"/repos","whenQuery":"name","call":"RepoByName","action":"s.repo.by_name",
    "args":[{"from":"query","as":"name","required":true}],"returns":[{"body":true}]},
  "repos_by_run":{"method":"GET","path":"/repos","whenQuery":"workflow_run_id","call":"ReposByRun","action":"s.repo.by_run",
    "args":[{"from":"query","as":"workflow_run_id","required":true}],"returns":[{"body":true}]}}}`

func selectedMux(t *testing.T, raw string, m *shelf, authorize dispatch.Authorizer) (*http.ServeMux, []dispatch.Route) {
	t.Helper()
	s, err := dispatch.Parse([]byte(raw))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	routes, err := dispatch.Dispatch(s, dispatch.Deps{Manager: m, Authorize: authorize})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.Handle(r.Method+" "+r.Path, r.Handler)
	}
	return mux, routes
}

func TestAnAddressAnswersWithTheCallItsQueryParameterSelects(t *testing.T) {
	cases := []struct {
		url, called, arg string
	}{
		{"/repos", "ListRepos", ""},
		{"/repos?name=alpha", "RepoByName", "alpha"},
		{"/repos?workflow_run_id=r9", "ReposByRun", "r9"},
		{"/repos?name=", "ListRepos", ""},
		{"/repos?workflow_run_id=", "ListRepos", ""},
		{"/repos?name=alpha&workflow_run_id=r9", "RepoByName", "alpha"},
		{"/repos?other=1", "ListRepos", ""},
	}
	for _, c := range cases {
		m := &shelf{}
		mux, _ := selectedMux(t, selectedSpec, m, nil)
		rec := do(t, mux, http.MethodGet, c.url, "")
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d, body %s", c.url, rec.Code, rec.Body)
			continue
		}
		if m.called != c.called || m.arg != c.arg {
			t.Errorf("%s: called %s(%q), want %s(%q)", c.url, m.called, m.arg, c.called, c.arg)
		}
	}
}

func TestOneAddressIsOneRouteWhateverItsAlternatives(t *testing.T) {
	_, routes := selectedMux(t, selectedSpec, &shelf{}, nil)
	if len(routes) != 1 {
		t.Fatalf("built %d routes for one address, want 1", len(routes))
	}
	if routes[0].Action != "s.repo.list" {
		t.Errorf("route action = %q, want the default operation's s.repo.list", routes[0].Action)
	}
}

func TestEachAlternativeAsksTheAuthorizerForItsOwnAction(t *testing.T) {
	asked := []string{}
	m := &shelf{}
	mux, _ := selectedMux(t, selectedSpec, m, func(ctx context.Context, action string) error {
		asked = append(asked, action)
		return nil
	})
	do(t, mux, http.MethodGet, "/repos", "")
	do(t, mux, http.MethodGet, "/repos?name=a", "")
	do(t, mux, http.MethodGet, "/repos?workflow_run_id=r", "")
	if got := strings.Join(asked, ","); got != "s.repo.list,s.repo.by_name,s.repo.by_run" {
		t.Errorf("authorizer asked for %s", got)
	}
}

func TestADeniedAlternativeNeverReachesTheManager(t *testing.T) {
	m := &shelf{}
	mux, _ := selectedMux(t, selectedSpec, m, func(ctx context.Context, action string) error {
		if action == "s.repo.by_name" {
			return dispatch.ErrForbidden
		}
		return nil
	})
	rec := do(t, mux, http.MethodGet, "/repos?name=a", "")
	if rec.Code != http.StatusForbidden || m.called != "" {
		t.Errorf("status %d, called %q; want 403 and no call", rec.Code, m.called)
	}
}

func TestEveryAlternativeIsStillItsOwnTool(t *testing.T) {
	raw := strings.NewReplacer(
		`"returns":[{"body":true}]},`, `"description":"d","returns":[{"body":true}]},`,
		`"returns":[{"body":true}]}}}`, `"description":"d","returns":[{"body":true}]}}}`,
	).Replace(selectedSpec)
	s, err := dispatch.Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	tools, err := dispatch.Tools(s, dispatch.Deps{Manager: &shelf{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 3 {
		t.Errorf("published %d tools, want 3", len(tools))
	}
}

func TestADeclarationThatSharesAnAddressWrongIsRefused(t *testing.T) {
	cases := map[string]string{
		"no default": `{"operations":{
  "a":{"method":"GET","path":"/r","whenQuery":"x","call":"RepoByName","action":"s.r.a","args":[{"from":"query","as":"x"}],"returns":[{"body":true}]},
  "b":{"method":"GET","path":"/r","whenQuery":"y","call":"RepoByName","action":"s.r.b","args":[{"from":"query","as":"y"}],"returns":[{"body":true}]}}}`,
		"two defaults": `{"operations":{
  "a":{"method":"GET","path":"/r","call":"ListRepos","action":"s.r.a","returns":[{"body":true}]},
  "b":{"method":"GET","path":"/r","call":"ListRepos","action":"s.r.b","returns":[{"body":true}]}}}`,
		"same selector twice": `{"operations":{
  "a":{"method":"GET","path":"/r","call":"ListRepos","action":"s.r.a","returns":[{"body":true}]},
  "b":{"method":"GET","path":"/r","whenQuery":"x","call":"RepoByName","action":"s.r.b","args":[{"from":"query","as":"x"}],"returns":[{"body":true}]},
  "c":{"method":"GET","path":"/r","whenQuery":"x","call":"RepoByName","action":"s.r.c","args":[{"from":"query","as":"x"}],"returns":[{"body":true}]}}}`,
		"lone selector": `{"operations":{
  "a":{"method":"GET","path":"/r","whenQuery":"x","call":"RepoByName","action":"s.r.a","args":[{"from":"query","as":"x"}],"returns":[{"body":true}]}}}`,
		"unusable selector": `{"operations":{
  "a":{"method":"GET","path":"/r","call":"ListRepos","action":"s.r.a","returns":[{"body":true}]},
  "b":{"method":"GET","path":"/r","whenQuery":"X y","call":"RepoByName","action":"s.r.b","args":[{"from":"query","as":"x"}],"returns":[{"body":true}]}}}`,
		"handled selector": `{"operations":{
  "a":{"method":"GET","path":"/r","call":"ListRepos","action":"s.r.a","returns":[{"body":true}]},
  "b":{"method":"GET","path":"/r","whenQuery":"x","handled":true,"action":"s.r.b"}}}`,
		"handled default": `{"operations":{
  "a":{"method":"GET","path":"/r","handled":true,"action":"s.r.a"},
  "b":{"method":"GET","path":"/r","whenQuery":"x","call":"RepoByName","action":"s.r.b","args":[{"from":"query","as":"x"}],"returns":[{"body":true}]}}}`,
	}
	for name, raw := range cases {
		if _, err := dispatch.Parse([]byte(raw)); err == nil {
			t.Errorf("%s: parsed without complaint", name)
		}
	}
}

func TestTwoOperationsAtOneAddressWithNoSelectorStillCollide(t *testing.T) {
	_, err := dispatch.Parse([]byte(`{"operations":{
  "a":{"method":"GET","path":"/r","call":"ListRepos","action":"s.r.a","returns":[{"body":true}]},
  "b":{"method":"GET","path":"/r","call":"ListRepos","action":"s.r.b","returns":[{"body":true}]}}}`))
	if err == nil || !strings.Contains(err.Error(), "both answer") {
		t.Fatalf("err = %v, want the existing address collision", err)
	}
}

const selectedAnonymousSpec = selectedSpec

func TestASharedAddressIsAnonymousOnlyWhenEveryAlternativeIs(t *testing.T) {
	cases := []struct {
		name      string
		anonymous []string
		wantOpen  bool
	}{
		{"default only", []string{"s.repo.list"}, false},
		{"all three", []string{"s.repo.list", "s.repo.by_name", "s.repo.by_run"}, true},
		{"alternative only", []string{"s.repo.by_name"}, false},
	}
	for _, c := range cases {
		table := dispatch.NewTable([]byte(selectedAnonymousSpec), nil, c.anonymous...)
		split := table.Split(&shelf{}, dispatch.Mount{})
		if open := len(split.Anonymous) == 1; open != c.wantOpen {
			t.Errorf("%s: anonymous = %d routes, gated = %d, want open=%v",
				c.name, len(split.Anonymous), len(split.Gated), c.wantOpen)
		}
		if len(split.Anonymous)+len(split.Gated) != 1 {
			t.Errorf("%s: %d routes in all, want 1", c.name, len(split.Anonymous)+len(split.Gated))
		}
	}
}

func TestAGatedSharedAddressStillAsksTheAuthorizerForTheAlternative(t *testing.T) {
	asked := map[string]bool{}
	table := dispatch.NewTable([]byte(selectedAnonymousSpec), nil, "s.repo.list")
	split := table.Split(&shelf{}, dispatch.Mount{Authorize: func(ctx context.Context, action string) error {
		asked[action] = true
		return nil
	}})
	for _, r := range split.Gated {
		rec := httptest.NewRecorder()
		r.Handler(rec, httptest.NewRequest(http.MethodGet, "/repos?name=a", nil))
	}
	if !asked["s.repo.by_name"] {
		t.Error("an alternative of a half-anonymous address was served without asking the authorizer")
	}
}
