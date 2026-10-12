package identity

import (
	"context"
	"errors"
	"fmt"
	"regexp"
)

var ErrInvalidWritten = errors.New("identity: invalid write report")

var ObjectPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type Written struct {
	Partition string          `json:"partition"`
	Owner     string          `json:"owner"`
	Objects   []WrittenObject `json:"objects"`
}

type WrittenObject struct {
	Object string `json:"object"`
	ID     string `json:"id"`
}

type Reactor interface {
	Wrote(ctx context.Context, w Written) error
}

func (w Written) Check() error {
	if !SegmentPattern.MatchString(w.Partition) {
		return fmt.Errorf("%w: partition %q must match %s", ErrInvalidWritten, w.Partition, SegmentPattern)
	}
	if !SegmentPattern.MatchString(w.Owner) {
		return fmt.Errorf("%w: owner %q must match %s", ErrInvalidWritten, w.Owner, SegmentPattern)
	}
	if len(w.Objects) == 0 {
		return fmt.Errorf("%w: a report names at least one object; there is no whole-owner form", ErrInvalidWritten)
	}
	for i, o := range w.Objects {
		if !ObjectPattern.MatchString(o.Object) {
			return fmt.Errorf("%w: object %d: %q must match %s", ErrInvalidWritten, i, o.Object, ObjectPattern)
		}
		if err := checkIdentity(o.ID); err != nil {
			return fmt.Errorf("%w: object %d: the id %q: %v", ErrInvalidWritten, i, o.ID, err)
		}
	}
	return nil
}

func (o WrittenObject) Refers(owner string) string {
	return owner + "." + o.Object
}
