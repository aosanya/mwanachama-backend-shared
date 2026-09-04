package memory_test

import (
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/entitygraphtest"
	"github.com/aosanya/mwanachama-backend-shared/memory"
)

func TestBackend_Conformance(t *testing.T) {
	b := memory.NewBackend()
	entitygraphtest.Run(t, b, b)
}
