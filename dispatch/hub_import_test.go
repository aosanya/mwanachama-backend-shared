package dispatch_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

type importManager struct {
	failWith error
	gotID    string
	gotBody  importBody
}

type importBody struct {
	Holder string `json:"holder"`
}

func (m *importManager) Take(ctx context.Context, id string) (string, error) {
	m.gotID = id
	return id, m.failWith
}

func (m *importManager) Claim(ctx context.Context, id string, body importBody) (string, error) {
	m.gotID = id
	m.gotBody = body
	return id, m.failWith
}

const importSpec = `{"operations":{
  "take":{"method":"GET","path":"/issues/{taskID}","call":"Take","action":"t.issue.take","description":"Take an issue.",
    "args":[{"from":"path","as":"taskID","required":true}],"returns":[{"body":true}]},
  "claim":{"method":"POST","path":"/issues/{taskID}/claim","call":"Claim","action":"t.issue.claim","description":"Claim an issue.",
    "args":[{"from":"path","as":"taskID","required":true},{"from":"body","whole":true,"optional":true}],"returns":[{"body":true}]}}}`

func importFixture(t *testing.T, m *importManager) (http.Handler, map[string]dispatch.Tool) {
	t.Helper()
	s, err := dispatch.Parse([]byte(importSpec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	routes, err := dispatch.Dispatch(s, dispatch.Deps{Manager: m})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.Handle(r.Method+" "+r.Path, r.Handler)
	}
	tools, err := dispatch.Tools(s, dispatch.Deps{Manager: m})
	if err != nil {
		t.Fatalf("tools: %v", err)
	}
	byName := map[string]dispatch.Tool{}
	for _, tool := range tools {
		byName[tool.Name] = tool
	}
	return mux, byName
}

func TestABusyDatabaseAnswers409NotAnInternalError(t *testing.T) {
	for _, message := range []string{"database is locked", "SQLITE_BUSY", "could not serialize access due to concurrent update", "deadlock detected"} {
		mux, tools := importFixture(t, &importManager{failWith: errors.New(message)})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/issues/one", nil))
		if rec.Code != http.StatusConflict {
			t.Errorf("%q: status %d, body %s; want 409", message, rec.Code, rec.Body)
		}
		_, err := call(t, tools["t_issue_take"], `{"taskID":"one"}`)
		if err == nil || !strings.Contains(err.Error(), "try again") {
			t.Errorf("%q: tool err = %v; want the try-again refusal", message, err)
		}
	}
}

func TestAnOrdinaryFailureIsNotTakenForABusyDatabase(t *testing.T) {
	mux, _ := importFixture(t, &importManager{failWith: errors.New("connection refused")})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/issues/one", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status %d; want 500", rec.Code)
	}
}

func TestAToolTakingTaskIDAcceptsIDForIt(t *testing.T) {
	m := &importManager{}
	_, tools := importFixture(t, m)
	if _, err := call(t, tools["t_issue_take"], `{"id":"abc"}`); err != nil {
		t.Fatalf("err = %v", err)
	}
	if m.gotID != "abc" {
		t.Errorf("id = %q; want abc", m.gotID)
	}
}

func TestIDAndTaskIDGivenTogetherMustAgree(t *testing.T) {
	_, tools := importFixture(t, &importManager{})
	if _, err := call(t, tools["t_issue_take"], `{"id":"a","taskID":"a"}`); err != nil {
		t.Errorf("same value: err = %v", err)
	}
	_, err := call(t, tools["t_issue_take"], `{"id":"a","taskID":"b"}`)
	if err == nil || !strings.Contains(err.Error(), "different values") {
		t.Errorf("err = %v; want the different-values refusal", err)
	}
}

func TestAnUnknownArgumentNamesTheAliasWhenToolTakesTaskID(t *testing.T) {
	_, tools := importFixture(t, &importManager{})
	_, err := call(t, tools["t_issue_take"], `{"taskID":"a","sector":"x"}`)
	if err == nil || !strings.Contains(err.Error(), `"id" is accepted`) {
		t.Errorf("err = %v; want the alias named", err)
	}
}

func TestAnEmptyBodyIsAllowedOnAnOptionalWholeBody(t *testing.T) {
	m := &importManager{}
	mux, _ := importFixture(t, m)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/issues/one/claim", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s; want 200", rec.Code, rec.Body)
	}
	if m.gotBody.Holder != "" {
		t.Errorf("holder = %q; want empty", m.gotBody.Holder)
	}
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/issues/one/claim", strings.NewReader(`{"holder":"w1"}`)))
	if rec.Code != http.StatusOK || m.gotBody.Holder != "w1" {
		t.Errorf("status %d, holder %q; want 200 and w1", rec.Code, m.gotBody.Holder)
	}
}
