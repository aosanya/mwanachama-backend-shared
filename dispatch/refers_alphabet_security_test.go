package dispatch_test

import (
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
)

func TestARefersMarkCarryingAnInvisibleLineBreakingOrLookalikeCharacterIsRefused(t *testing.T) {
	hostile := map[string]string{
		"trailing newline":       "agency.role\\n",
		"trailing carriage":      "agency.role\\r",
		"inner line separator":   "agency.ro\\u2028le",
		"trailing nul":           "agency.role\\u0000",
		"zero-width space":       "agency.role\\u200b",
		"right-to-left override": "agency.\\u202eelor",
		"byte order mark":        "\\ufeffagency.role",
		"soft hyphen":            "agen\\u00adcy.role",
		"cyrillic a":             "\\u0430gency.role",
		"accented o":             "agency.r\\u00f3le",
		"leading space":          " agency.role",
		"trailing space":         "agency.role ",
		"doubled dot":            "agency..role",
	}
	for what, v := range hostile {
		_, err := dispatch.Parse([]byte(specWith(`{"from":"path","as":"slug","refers":"` + v + `"},{"from":"query","as":"k"}`)))
		if err == nil || !strings.Contains(err.Error(), "is not <module>.<object>") {
			t.Errorf("refers with a %s: a declaration naming another module's object must be refused at load, got %v", what, err)
		}
	}
}

func TestARefersMarkSpelledUnderACaseVariantKeyIsHeldToTheSameAlphabet(t *testing.T) {
	for _, key := range []string{"REFERS", "Refers"} {
		_, err := dispatch.Parse([]byte(specWith(`{"from":"path","as":"slug","` + key + `":"Agency.role"},{"from":"query","as":"k"}`)))
		if err == nil || !strings.Contains(err.Error(), "is not <module>.<object>") {
			t.Errorf("key %q: a case-variant spelling must not slip a refers mark past the pattern, got %v", key, err)
		}
	}
}
