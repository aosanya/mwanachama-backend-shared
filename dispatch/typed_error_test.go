package dispatch_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

type mergeConflict struct {
	TaskID string
	Files  []string
}

func (e *mergeConflict) Error() string {
	return fmt.Sprintf("merge conflict on %q: %v", e.TaskID, e.Files)
}

type otherTypedError struct{}

func (otherTypedError) Error() string { return "other typed" }

type merger struct{ fail error }

func (m *merger) Merge(ctx context.Context, branch string) (thing, error) {
	if m.fail != nil {
		return thing{}, m.fail
	}
	return thing{Slug: branch}, nil
}

const mergeSpec = `{
  "operations": {
    "merge": {"method":"POST","path":"/branches/{branch}/merge","call":"Merge","action":"t.branch.merge",
      "title":"Merge a branch","description":"Merges a task branch.",
      "args":[{"from":"path","as":"branch"}],
      "returns":[{"body":true}]}
  },
  "errors": {"ErrMergeConflict": 409}
}`

func conflict() *mergeConflict {
	return &mergeConflict{TaskID: "task-7", Files: []string{"a.go", "b.go"}}
}

func filesOf(e *mergeConflict) map[string]any {
	return map[string]any{"task_id": e.TaskID, "conflicting_files": e.Files}
}

func mergeHandler(t *testing.T, fail error, sentinel error) http.Handler {
	t.Helper()
	s, err := dispatch.Parse([]byte(mergeSpec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	routes, err := dispatch.Dispatch(s, dispatch.Deps{
		Manager: &merger{fail: fail},
		Errors:  map[string]error{"ErrMergeConflict": sentinel},
	})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.Handle(r.Method+" "+r.Path, r.Handler)
	}
	return mux
}

func decodeBody(t *testing.T, body string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("body is not an object: %v: %s", err, body)
	}
	return out
}

func TestATypedErrorInTheTableIsNotRedactedToA500(t *testing.T) {
	h := mergeHandler(t, conflict(), dispatch.ByType(&mergeConflict{}, nil))

	rec := do(t, h, http.MethodPost, "/branches/task-7/merge", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body %s", rec.Code, rec.Body.String())
	}
	if got := decodeBody(t, rec.Body.String())["error"]; got != conflict().Error() {
		t.Fatalf("error = %v, want the error's own text", got)
	}
}

func TestATypedErrorIsFoundBehindAWrap(t *testing.T) {
	h := mergeHandler(t, fmt.Errorf("merging: %w", conflict()), dispatch.ByType(&mergeConflict{}, nil))

	rec := do(t, h, http.MethodPost, "/branches/task-7/merge", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 through a wrapped error; body %s", rec.Code, rec.Body.String())
	}
}

func TestATypedErrorContributesItsDetailToTheBody(t *testing.T) {
	h := mergeHandler(t, conflict(), dispatch.ByType(&mergeConflict{}, filesOf))

	rec := do(t, h, http.MethodPost, "/branches/task-7/merge", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body %s", rec.Code, rec.Body.String())
	}
	body := decodeBody(t, rec.Body.String())
	if body["task_id"] != "task-7" {
		t.Errorf("task_id = %v, want task-7; body %s", body["task_id"], rec.Body.String())
	}
	files, _ := body["conflicting_files"].([]any)
	if len(files) != 2 || files[0] != "a.go" || files[1] != "b.go" {
		t.Errorf("conflicting_files = %v, want [a.go b.go]", body["conflicting_files"])
	}
	if body["error"] != conflict().Error() {
		t.Errorf("error = %v, want the message beside the detail", body["error"])
	}
}

func TestDetailNeverReplacesTheErrorMessage(t *testing.T) {
	h := mergeHandler(t, conflict(), dispatch.ByType(&mergeConflict{}, func(e *mergeConflict) map[string]any {
		return map[string]any{"error": "forged", "task_id": e.TaskID}
	}))

	rec := do(t, h, http.MethodPost, "/branches/task-7/merge", "")
	body := decodeBody(t, rec.Body.String())
	if body["error"] != conflict().Error() {
		t.Fatalf("error = %v, a detail key replaced the message", body["error"])
	}
	if body["task_id"] != "task-7" {
		t.Fatalf("task_id = %v, the rest of the detail was lost", body["task_id"])
	}
}

func TestATypedErrorThatIsNotMappedStaysOpaqueAndLeaksNoDetail(t *testing.T) {
	h := mergeHandler(t, otherTypedError{}, dispatch.ByType(&mergeConflict{}, filesOf))

	rec := do(t, h, http.MethodPost, "/branches/task-7/merge", "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if body := decodeBody(t, rec.Body.String()); body["error"] != "internal error" || len(body) != 1 {
		t.Fatalf("body = %s, want only the opaque message", rec.Body.String())
	}
}

func TestPlainSentinelsStillMatchByIdentityNextToATypedOne(t *testing.T) {
	other := errors.New("some other failure")
	h := mergeHandler(t, other, dispatch.ByType(&mergeConflict{}, filesOf))
	if rec := do(t, h, http.MethodPost, "/branches/task-7/merge", ""); rec.Code != http.StatusInternalServerError {
		t.Fatalf("a different error was matched by the typed key: status %d", rec.Code)
	}

	h = mergeHandler(t, fmt.Errorf("wrapped: %w", other), other)
	if rec := do(t, h, http.MethodPost, "/branches/task-7/merge", ""); rec.Code != http.StatusConflict {
		t.Fatalf("a plain sentinel stopped matching: status %d", rec.Code)
	}
}

func TestAToolReportsATypedErrorInsteadOfRedactingIt(t *testing.T) {
	s, err := dispatch.Parse([]byte(mergeSpec))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tools, err := dispatch.Tools(s, dispatch.Deps{
		Manager: &merger{fail: conflict()},
		Errors:  map[string]error{"ErrMergeConflict": dispatch.ByType(&mergeConflict{}, filesOf)},
	})
	if err != nil {
		t.Fatalf("tools: %v", err)
	}
	_, err = tools[0].Invoke(context.Background(), json.RawMessage(`{"branch":"task-7"}`))
	if err == nil || !strings.Contains(err.Error(), "a.go") {
		t.Fatalf("err = %v, want the conflict's own text naming the files", err)
	}
}
