package events

import (
	"context"
	"log"
)

// logPublisher is the trivial default [Publisher] — it writes each event to
// the standard `log` package rather than delivering it anywhere. Callers
// that need real fan-out (webhooks, a queue, in-process subscribers) provide
// their own [Publisher] implementation.
type logPublisher struct {
	serviceName string
}

// NewLogPublisher returns a [Publisher] that logs every Publish call,
// prefixed with serviceName. Suitable as a default until a real delivery
// mechanism is wired in.
func NewLogPublisher(serviceName string) Publisher {
	return &logPublisher{serviceName: serviceName}
}

// Publish implements [Publisher].
func (p *logPublisher) Publish(_ context.Context, topic string, payload any) error {
	log.Printf("events[%s]: topic=%q payload=%T", p.serviceName, topic, payload)
	return nil
}
