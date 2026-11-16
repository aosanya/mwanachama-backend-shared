package plaintext

import (
	"errors"
	"fmt"
	"testing"
)

type stateOnlyError struct{ state string }

func (e stateOnlyError) Error() string    { return "refused" }
func (e stateOnlyError) SQLState() string { return e.state }

func TestUnstorable_RecognisesAnyErrorCarryingTheRefusalSQLState(t *testing.T) {
	for _, state := range []string{"22021", "22P05"} {
		if !errors.Is(Unstorable(fmt.Errorf("write: %w", stateOnlyError{state})), ErrUnstorableText) {
			t.Fatalf("SQLSTATE %s was not read as unstorable text", state)
		}
	}
	if Unstorable(stateOnlyError{"23505"}) != nil {
		t.Fatal("a unique violation was read as unstorable text")
	}
}
