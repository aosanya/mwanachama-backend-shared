package identity_test

import (
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/identity"
)

func TestParseRefusesByteOrderMarkAndSoftHyphen(t *testing.T) {
	for _, raw := range []string{"agency/inst/\ufeffR-3", "agency/inst/R\u00ad-3", "agency/inst/R-3\u2029"} {
		if _, err := identity.Parse(raw); !errors.Is(err, identity.ErrInvalidRaw) {
			t.Errorf("Parse(%q) err = %v, want ErrInvalidRaw", raw, err)
		}
	}
}

func TestWrittenCheckRefusesInvisibleAndLineBreakingIds(t *testing.T) {
	for _, id := range []string{"R-3\u202E", "R-3\u2028x", "\u200bR-3", "R-3\u2066"} {
		w := identity.Written{Partition: "agy1", Owner: "agency", Objects: []identity.WrittenObject{{Object: "role", ID: id}}}
		if err := w.Check(); err == nil {
			t.Errorf("Check accepted id %q", id)
		}
	}
}

func TestParseKeepsOrdinaryUnicodeIdentities(t *testing.T) {
	for _, raw := range []string{"agency/inst/M\u00fcller-3", "agency/inst/\u6771\u4eac/\u652f\u5e97", "agency/inst/a b"} {
		if _, err := identity.Parse(raw); err != nil {
			t.Errorf("Parse(%q) err = %v", raw, err)
		}
	}
}
