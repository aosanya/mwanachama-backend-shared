package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
)

const ConversationHeader = "X-Conversation-Id"

const ConversationMetaKey = "conversation_id"

var ConversationPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

var ErrInvalidConversation = errors.New("is not a conversation id")

type Request struct {
	Arguments json.RawMessage
	Meta      map[string]any
}

type conversationKey struct{}

func WithConversation(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, conversationKey{}, id)
}

func ConversationOf(ctx context.Context) string {
	id, _ := ctx.Value(conversationKey{}).(string)
	return id
}

func conversationFromHeader(r *http.Request) (context.Context, error) {
	values := r.Header.Values(ConversationHeader)
	if len(values) > 1 {
		return r.Context(), fmt.Errorf("%s is sent more than once", ConversationHeader)
	}
	if len(values) == 0 {
		return r.Context(), nil
	}
	return carry(r.Context(), values[0], ConversationHeader)
}

func conversationFromMeta(ctx context.Context, meta map[string]any) (context.Context, error) {
	v, ok := meta[ConversationMetaKey]
	if !ok || v == nil {
		return ctx, nil
	}
	id, ok := v.(string)
	if !ok {
		return ctx, fmt.Errorf("_meta.%s %w", ConversationMetaKey, ErrInvalidConversation)
	}
	return carry(ctx, id, "_meta."+ConversationMetaKey)
}

func carry(ctx context.Context, id, where string) (context.Context, error) {
	if id == "" {
		return ctx, nil
	}
	if !ConversationPattern.MatchString(id) {
		return ctx, fmt.Errorf("%s %w", where, ErrInvalidConversation)
	}
	return WithConversation(ctx, id), nil
}
