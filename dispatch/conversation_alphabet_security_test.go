package dispatch_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

var hostileConversations = []string{
	"chat-1\n", "chat-1\r", "chat-1\r\n", "chat-1\x00", "chat\u00a01", "chat\u200b1", "chat-1\u2028",
	"chat-1\u202e", "\ufeffchat-1", "chat\u00ad1", "\uff43hat-1", "ch\u0430t-1", "chat-1, chat-2", "chat-1;x",
	".leading", ":leading", "_leading", "a" + strings.Repeat("b", 128),
}

func TestAConversationHeaderWithInvisibleLineBreakingOrLookalikeCharactersNeverReachesTheHandler(t *testing.T) {
	h, m := buildConversation(t, nil)
	for _, id := range hostileConversations {
		req := httptest.NewRequest(http.MethodPost, "/notes", strings.NewReader(`{"text":"x"}`))
		req.Header[dispatch.ConversationHeader] = []string{id}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("header %q: status %d, want 400: %s", id, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), id) {
			t.Errorf("header %q: the refusal repeats the value: %s", id, rec.Body.String())
		}
	}
	if len(m.seen) != 0 {
		t.Fatalf("the handler ran on a refused conversation: %q", m.seen)
	}
}

func TestAConversationInMetaWithInvisibleLineBreakingOrLookalikeCharactersIsRefused(t *testing.T) {
	tool, m := noteTool(t, nil)
	hosted := dispatch.WithConversation(context.Background(), "host-set")
	for _, id := range hostileConversations {
		_, err := tool.Handle(hosted, dispatch.Request{
			Arguments: []byte(`{"text":"x"}`),
			Meta:      map[string]any{dispatch.ConversationMetaKey: id},
		})
		if !errors.Is(err, dispatch.ErrInvalidConversation) {
			t.Errorf("_meta %q: err = %v, want ErrInvalidConversation", id, err)
		}
	}
	for _, meta := range []map[string]any{
		{dispatch.ConversationMetaKey: ""}, {dispatch.ConversationMetaKey: nil}, {"Conversation_Id": "forged"}, {"CONVERSATION_ID": "forged"},
	} {
		out, err := tool.Handle(hosted, dispatch.Request{Arguments: []byte(`{"text":"x"}`), Meta: meta})
		if err != nil || out != "host-set" {
			t.Errorf("_meta %v: out = %v, err = %v, want the host's conversation to stand", meta, out, err)
		}
	}
	if len(m.seen) != 4 {
		t.Fatalf("the handler ran %d time(s), want 4", len(m.seen))
	}
}

func TestTwoSpellingsOfTheConversationHeaderOnTheWireAreRefusedAsTwoHeaders(t *testing.T) {
	h, m := buildConversation(t, nil)
	srv := httptest.NewServer(h)
	defer srv.Close()

	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	body := `{"text":"x"}`
	fmt.Fprintf(conn, "POST /notes HTTP/1.1\r\nHost: x\r\nContent-Type: application/json\r\nx-conversation-id: chat-1\r\nX-CONVERSATION-ID: chat-2\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("two spellings of one header: status %d, want 400", resp.StatusCode)
	}
	if len(m.seen) != 0 {
		t.Fatalf("the handler ran on a doubled conversation: %q", m.seen)
	}
}
