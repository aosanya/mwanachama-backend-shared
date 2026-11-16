package consent

import (
	"context"
	"errors"
)

var ErrRequired = errors.New("consent: no live consent on record for this actor")
var ErrWithdrawn = errors.New("consent: consent was withdrawn")

type Gate interface {
	Check(ctx context.Context, actorID string) error
}

func Require(ctx context.Context, g Gate, actorID string) error {
	if g == nil {
		return nil
	}
	return g.Check(ctx, actorID)
}
