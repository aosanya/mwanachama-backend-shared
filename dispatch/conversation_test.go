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

type conversationManager struct {
	seen []string
}

func (m *conversationManager) Note(ctx context.Context, text string) (string, error) {
	id := dispatch.ConversationOf(ctx)
	m.seen = append(m.seen, id)
	return id, nil
}

const conversationSpec = `{
  "operations": {
    "note": {"method":"POST","path":"/notes","call":"Note","action":"t.note.write",
      "title":"Write a note",
      "description":"Writes one note.",
      "args":[{"from":"body","as":"text","description":"The note."}],
      "returns":[{"body":true}]}
  }
}`

func conversationDeps(m *conversationManager, authorize dispatch.Authorizer) (*dispatch.Spec, dispatch.Deps) {
	s, err := dispatch.Parse([]byte(conversationSpec))
	if err != nil {
		panic(err)
	}
	return s, dispatch.Deps{Manager: m, Authorize: authorize}
}

func buildConversation(t *testing.T, authorize dispatch.Authorizer) (http.Handler, *conversationManager) {
	t.Helper()
	m := &conversationManager{}
	s, d := conversationDeps(m, authorize)
	routes, err := dispatch.Dispatch(s, d)
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	mux := http.NewServeMux()
	for _, r := range routes {
		mux.Handle(r.Method+" "+r.Path, r.Handler)
	}
	return mux, m
}

func noteTool(t *testing.T, authorize dispatch.Authorizer) (dispatch.Tool, *conversationManager) {
	t.Helper()
	m := &conversationManager{}
	s, d := conversationDeps(m, authorize)
	tools, err := dispatch.Tools(s, d)
	if err != nil {
		t.Fatalf("tools: %v", err)
	}
	return tools[0], m
}

func post(h http.Handler, path, body string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Add(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestTheConversationComesFromOneHeaderAndReachesTheHandler(t *testing.T) {
	h, m := buildConversation(t, nil)

	rec := post(h, "/notes", `{"text":"hello"}`, dispatch.ConversationHeader, "chat-7f3a")
	if rec.Code != http.StatusOK || rec.Body.String() != `"chat-7f3a"`+"\n" {
		t.Fatalf("status %d, body %q", rec.Code, rec.Body.String())
	}
	rec = post(h, "/notes", `{"text":"hello"}`)
	if rec.Code != http.StatusOK || rec.Body.String() != `""`+"\n" {
		t.Fatalf("a request with no header carries none: status %d, body %q", rec.Code, rec.Body.String())
	}
	if len(m.seen) != 2 || m.seen[0] != "chat-7f3a" || m.seen[1] != "" {
		t.Fatalf("seen = %q", m.seen)
	}
}

func TestAConversationIsNeverReadFromTheQueryOrTheBody(t *testing.T) {
	h, m := buildConversation(t, nil)

	rec := post(h, "/notes?conversation_id=forged&X-Conversation-Id=forged",
		`{"text":"hello","conversation_id":"forged","session_id":"forged"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	if len(m.seen) != 1 || m.seen[0] != "" {
		t.Fatalf("seen = %q, want one empty conversation", m.seen)
	}
}

func TestAConversationOffItsShapeIsRefusedBeforeTheCall(t *testing.T) {
	h, m := buildConversation(t, nil)

	for _, id := range []string{
		"with space", "x‮", "-leading-dash", strings.Repeat("a", 129), "a/b", "a\tb",
	} {
		rec := post(h, "/notes", `{"text":"hello"}`, dispatch.ConversationHeader, id)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%q: status %d, want 400: %s", id, rec.Code, rec.Body.String())
		}
	}
	rec := post(h, "/notes", `{"text":"hello"}`,
		dispatch.ConversationHeader, "chat-1", dispatch.ConversationHeader, "chat-2")
	if rec.Code != http.StatusBadRequest {
		t.Errorf("two headers: status %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if len(m.seen) != 0 {
		t.Fatalf("the handler ran %d time(s) on a refused conversation", len(m.seen))
	}
	for _, id := range []string{"chat-1", "7f3a0c2e-1b4d-4e5f-9a8b-0c1d2e3f4a5b", "A.b_c:d-e", "1"} {
		if !dispatch.ConversationPattern.MatchString(id) {
			t.Errorf("%q is a conversation id", id)
		}
	}
}

func TestTheGateRunsBeforeTheConversationIsRead(t *testing.T) {
	refuse := func(context.Context, string) error { return errors.New("no") }
	h, m := buildConversation(t, refuse)

	rec := post(h, "/notes", `{"text":"hello"}`, dispatch.ConversationHeader, "with space")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d, want 403 before the header is judged: %s", rec.Code, rec.Body.String())
	}
	if len(m.seen) != 0 {
		t.Fatal("the handler ran for a refused caller")
	}

	tool, _ := noteTool(t, refuse)
	_, err := tool.Handle(context.Background(), dispatch.Request{
		Arguments: []byte(`{"text":"hello"}`),
		Meta:      map[string]any{dispatch.ConversationMetaKey: "with space"},
	})
	if !errors.Is(err, dispatch.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden before _meta is judged", err)
	}
}

func TestAToolReadsTheConversationFromMetaAndNeverFromAnArgument(t *testing.T) {
	tool, m := noteTool(t, nil)
	ctx := context.Background()

	out, err := tool.Handle(ctx, dispatch.Request{
		Arguments: []byte(`{"text":"hello"}`),
		Meta:      map[string]any{"progressToken": 3, dispatch.ConversationMetaKey: "chat-7f3a"},
	})
	if err != nil || out != "chat-7f3a" {
		t.Fatalf("out = %v, err = %v", out, err)
	}
	out, err = tool.Handle(ctx, dispatch.Request{Arguments: []byte(`{"text":"hello"}`), Meta: map[string]any{"progressToken": 3}})
	if err != nil || out != "" {
		t.Fatalf("_meta without the key carries none: out = %v, err = %v", out, err)
	}
	out, err = tool.Invoke(ctx, []byte(`{"text":"hello"}`))
	if err != nil || out != "" {
		t.Fatalf("Invoke carries none: out = %v, err = %v", out, err)
	}

	for _, args := range []string{
		`{"text":"hello","conversation_id":"forged"}`,
		`{"text":"hello","session_id":"forged"}`,
		`{"text":"hello","_meta":{"conversation_id":"forged"}}`,
	} {
		if _, err := tool.Invoke(ctx, []byte(args)); err == nil || !strings.Contains(err.Error(), "takes no argument") {
			t.Errorf("%s: err = %v, want the argument refused", args, err)
		}
	}
	for _, meta := range []map[string]any{
		{dispatch.ConversationMetaKey: 42},
		{dispatch.ConversationMetaKey: "with space"},
		{dispatch.ConversationMetaKey: map[string]any{"id": "chat-1"}},
	} {
		_, err := tool.Handle(ctx, dispatch.Request{Arguments: []byte(`{"text":"hello"}`), Meta: meta})
		if !errors.Is(err, dispatch.ErrInvalidConversation) {
			t.Errorf("%v: err = %v, want ErrInvalidConversation", meta, err)
		}
	}
	if len(m.seen) != 3 {
		t.Fatalf("the handler ran %d time(s), want 3", len(m.seen))
	}
}

func TestTheCallsOwnConversationOutranksTheHosts(t *testing.T) {
	tool, _ := noteTool(t, nil)
	hosted := dispatch.WithConversation(context.Background(), "host-set")

	out, err := tool.Invoke(hosted, []byte(`{"text":"hello"}`))
	if err != nil || out != "host-set" {
		t.Fatalf("a host's conversation survives a call without _meta: out = %v, err = %v", out, err)
	}
	out, err = tool.Handle(hosted, dispatch.Request{
		Arguments: []byte(`{"text":"hello"}`),
		Meta:      map[string]any{dispatch.ConversationMetaKey: "chat-7f3a"},
	})
	if err != nil || out != "chat-7f3a" {
		t.Fatalf("_meta outranks the host: out = %v, err = %v", out, err)
	}
	if dispatch.ConversationOf(context.Background()) != "" || dispatch.WithConversation(context.Background(), "") != context.Background() {
		t.Fatal("an empty conversation is none")
	}
}
