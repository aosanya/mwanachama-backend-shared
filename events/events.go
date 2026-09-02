// Package events defines the trivial publish contract mwanachama-git and
// mwanachama-taskmanager use to emit domain events, replacing
// CodeValdSharedLib/eventbus.Publisher (which layers a CodeValdCross
// registrar/heartbeat this project has no equivalent of).
package events

import "context"

// Publisher publishes a single event under topic, scoped to whatever
// identity payload's fields already carry (there is no separate agency/
// registrar envelope here). Implementations must be safe for concurrent use.
type Publisher interface {
	Publish(ctx context.Context, topic string, payload any) error
}

// PublisherFunc adapts a plain function to the [Publisher] interface —
// useful for tests that record events or for ad-hoc inline implementations.
type PublisherFunc func(ctx context.Context, topic string, payload any) error

// Publish invokes f.
func (f PublisherFunc) Publish(ctx context.Context, topic string, payload any) error {
	return f(ctx, topic, payload)
}
