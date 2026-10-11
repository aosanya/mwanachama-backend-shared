package identity_test

import (
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/identity"
)

func TestS60_OpenHole_ARawCarriesInvisibleAndLineBreakingCharacters(t *testing.T) {
	for _, raw := range []string{
		"agency/inst/R-3‮",
		"agency/inst/R-3⁦",
		"agency/inst/​R-3",
		"agency/inst/R -3",
	} {
		if _, err := identity.Parse(raw); !errors.Is(err, identity.ErrInvalidRaw) {
			t.Errorf("S60 (P3): Parse(%q) answered err=%v and a raw that renders as %q under a different key; want ErrInvalidRaw for a bidi, zero-width or line-separator character", raw, err, "agency/inst/R-3")
		}
	}
}
