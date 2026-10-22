package spec_test

import (
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

func TestAGeometryFieldRefusesWhatCannotApplyToAPlace(t *testing.T) {
	for name, field := range map[string]string{
		"default": `{"name": "shape", "type": "geometry", "description": "d", "default": "x"}`,
		"matches": `{"name": "shape", "type": "geometry", "description": "d", "matches": "slug"}`,
		"primary": `{"name": "shape", "type": "geometry", "description": "d", "primary": true}`,
	} {
		raw := `{"module": "atlas", "domain": "town", "instance": "twn1", "objects": [{"name": "place", "table": "places", "description": "d", "fields": [` +
			`{"name": "id", "type": "string", "description": "d", "primary": true}, ` + field + `]}]}`
		if name == "primary" {
			raw = strings.Replace(raw, `, "primary": true}, `, `}, `, 1)
		}
		_, err := spec.Parse([]byte(raw))
		if err == nil || !strings.Contains(err.Error(), "shape") {
			t.Errorf("%s: want a refusal naming the field, got %v", name, err)
		}
	}
}
