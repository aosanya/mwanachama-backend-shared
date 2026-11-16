package dispatch_test

import (
	"net/http"
	"strings"
	"testing"
)

const requiredQuerySpec = `{"operations":{
  "repo_by_name":{"method":"GET","path":"/repos/find","call":"RepoByName","action":"s.repo.by_name",
    "args":[{"from":"query","as":"name","required":true}],"returns":[{"body":true}]}}}`

func TestAMissingRequiredQueryArgumentAnswers400AndNeverReachesTheManager(t *testing.T) {
	m := &shelf{}
	mux, _ := selectedMux(t, requiredQuerySpec, m, nil)
	rec := do(t, mux, http.MethodGet, "/repos/find", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d, body %s; want 400", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "name") {
		t.Errorf("body %s does not name the missing parameter", rec.Body)
	}
	if m.called != "" {
		t.Errorf("manager reached through %s", m.called)
	}
}

func TestARequiredQueryArgumentGivenEmptyIsStillGiven(t *testing.T) {
	m := &shelf{}
	mux, _ := selectedMux(t, requiredQuerySpec, m, nil)
	rec := do(t, mux, http.MethodGet, "/repos/find?name=", "")
	if rec.Code != http.StatusOK || m.called != "RepoByName" {
		t.Errorf("status %d, called %q; want 200 and RepoByName", rec.Code, m.called)
	}
}
