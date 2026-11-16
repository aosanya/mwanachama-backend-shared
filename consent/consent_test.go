package consent_test

import (
	"context"
	"errors"
	"github.com/aosanya/mwanachama-backend-shared/consent"
	"testing"
)

type refuse struct{ err error }

func (r refuse) Check(context.Context, string) error {
	return r.err
}
func TestRequireWithNoGateAllows(t *testing.T) {
	if err := consent.Require(context.Background(), nil, "mem-1"); err != nil {
		t.Fatalf("Require = %v, want nil", err)
	}
}
func TestRequirePassesTheGateRefusalThrough(t *testing.T) {
	err := consent.Require(context.Background(), refuse{consent.ErrWithdrawn}, "mem-1")
	if !errors.Is(err, consent.ErrWithdrawn) {
		t.Fatalf("Require = %v, want ErrWithdrawn", err)
	}
}
