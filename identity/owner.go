package identity

import (
	"context"
	"errors"
)

var ErrNotFound = errors.New("identity: not found")

var ErrNotOwned = errors.New("identity: not owned by this module")

type Owner interface {
	StableIdentity(ctx context.Context, object, id string) (string, error)
	Resolve(ctx context.Context, raw string) (object, id string, err error)
}
