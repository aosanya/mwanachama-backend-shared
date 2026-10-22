package spec_test

import (
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

func withSettings(settings string) string {
	return strings.Replace(oneObject, `"module": "record",`, `"module": "record", "settings": [`+settings+`],`, 1)
}

const recordSettings = `
  {"name": "approval", "type": "bool", "description": "Whether an entry needs approval.", "default": false},
  {"name": "code_length", "type": "int", "description": "Digits in a code.", "default": 6},
  {"name": "language", "type": "enum", "description": "Language entries are written in.", "values": ["en", "sw"], "default": "en"}`

func TestASpecCarriesTheSettingsItsBlueprintDeclares(t *testing.T) {
	b := mustBlueprint(t, withSettings(recordSettings))
	s, err := b.Parse([]byte(domainSpec(`{"name":"entry","role":"entry","description":"One entry."}`)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(s.Settings) != 3 {
		t.Fatalf("settings = %d, want 3", len(s.Settings))
	}
	if st, ok := s.Setting("code_length"); !ok || st.Type != spec.TypeInt {
		t.Fatalf("code_length = %+v, %v", st, ok)
	}
}

func TestABlueprintWithoutSettingsLoadsUnchanged(t *testing.T) {
	b := mustBlueprint(t, oneObject)
	s, err := b.Parse([]byte(domainSpec(`{"name":"entry","role":"entry","description":"One entry."}`)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(s.Settings) != 0 {
		t.Fatalf("settings = %v, want none", s.Settings)
	}
}

func TestADomainSpecCannotDeclareSettings(t *testing.T) {
	b := mustBlueprint(t, withSettings(recordSettings))
	raw := `{"module":"record","domain":"clinic","instance":"clinic","settings":[],"objects":[{"name":"entry","role":"entry","description":"One entry."}]}`
	if _, err := b.Parse([]byte(raw)); err == nil {
		t.Fatal("a domain spec declaring settings was accepted")
	}
}

func TestABadSettingIsRefusedAtLoad(t *testing.T) {
	cases := map[string]string{
		"unknown type":      `{"name": "x", "type": "bytes", "description": "d", "default": "a"}`,
		"no description":    `{"name": "x", "type": "bool", "description": "", "default": true}`,
		"enum no values":    `{"name": "x", "type": "enum", "description": "d", "default": "a"}`,
		"values on string":  `{"name": "x", "type": "string", "description": "d", "values": ["a"], "default": "a"}`,
		"no default":        `{"name": "x", "type": "bool", "description": "d"}`,
		"default off shape": `{"name": "x", "type": "int", "description": "d", "default": 1.5}`,
		"default off enum":  `{"name": "x", "type": "enum", "description": "d", "values": ["a"], "default": "b"}`,
		"bad name":          `{"name": "x y", "type": "bool", "description": "d", "default": true}`,
		"twice":             `{"name": "x", "type": "bool", "description": "d", "default": true}, {"name": "x", "type": "bool", "description": "d", "default": true}`,
	}
	for name, setting := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := spec.ParseBlueprint([]byte(withSettings(setting))); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

func TestCheckSettingsHoldsAValueToItsShape(t *testing.T) {
	s := mustBlueprint(t, withSettings(recordSettings))
	if p := spec.CheckSettings(s.Settings, map[string]any{"approval": true, "code_length": float64(8), "language": "sw"}); len(p) != 0 {
		t.Fatalf("good values refused: %v", p)
	}
	p := spec.CheckSettings(s.Settings, map[string]any{
		"approval": "yes", "code_length": 4.5, "language": "fr", "colour": "red",
	})
	if len(p) != 4 {
		t.Fatalf("problems = %v, want 4", p)
	}
}

func TestResolveNamesTheLayerThatSetTheValue(t *testing.T) {
	settings := mustBlueprint(t, withSettings(recordSettings)).Settings
	definition := map[string]any{"code_length": float64(8), "language": "sw"}
	overrides := map[string]spec.Override{"language": {Value: "en", By: "owner-1", At: "2026-10-22T09:00:00Z"}}

	module, _ := spec.Resolve(settings, definition, overrides, "approval")
	if module.Layer != spec.LayerModule || module.Value != false {
		t.Fatalf("approval = %+v", module)
	}
	def, _ := spec.Resolve(settings, definition, overrides, "code_length")
	if def.Layer != spec.LayerDefinition || def.Value != float64(8) {
		t.Fatalf("code_length = %+v", def)
	}
	over, _ := spec.Resolve(settings, definition, overrides, "language")
	if over.Layer != spec.LayerOverride || over.Value != "en" || over.By != "owner-1" || over.At == "" {
		t.Fatalf("language = %+v", over)
	}
	if _, err := spec.Resolve(settings, definition, overrides, "colour"); err == nil {
		t.Fatal("an undeclared setting resolved")
	}
	if all := spec.ResolveAll(settings, definition, overrides); len(all) != 3 {
		t.Fatalf("all = %v", all)
	}
}

func TestResolveSkipsALayerWhoseValueNoLongerFits(t *testing.T) {
	settings := mustBlueprint(t, withSettings(recordSettings)).Settings
	overrides := map[string]spec.Override{"language": {Value: "fr", By: "owner-1", At: "2026-10-22T09:00:00Z"}}
	got, _ := spec.Resolve(settings, map[string]any{"language": "sw"}, overrides, "language")
	if got.Layer != spec.LayerDefinition || got.Value != "sw" {
		t.Fatalf("language = %+v", got)
	}
}
