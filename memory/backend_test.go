package memory_test

import (
	"testing"

	"github.com/aosanya/mwanachama-go-shared/entitygraphtest"
	"github.com/aosanya/mwanachama-go-shared/memory"
)

func TestBackend_Conformance(t *testing.T) {
	b := memory.NewBackend()
	entitygraphtest.Run(t, b, b, "agency-1")
}
