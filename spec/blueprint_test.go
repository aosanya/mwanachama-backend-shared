package spec_test

import (
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

const oneObject = `{
  "module": "record",
  "objects": [
    {
      "role": "entry",
      "description": "One thing this module keeps a record of.",
      "fields": [
        {"name": "id", "type": "string", "description": "Storage key.", "primary": true},
        {"name": "slug", "type": "string", "description": "URL key.", "required": true},
        {"name": "state", "type": "enum", "description": "Where review has got to.",
         "default": "pending", "values": ["pending", "open", "closed"]}
      ],
      "indexes": [
        {"name": "state", "description": "The reviewer's queue.", "fields": ["state"]}
      ]
    }
  ]
}`

func domainSpec(objects string) string {
	return `{"module":"record","domain":"clinic","instance":"clinic","objects":[` + objects + `]}`
}

func mustBlueprint(t *testing.T, raw string) *spec.Blueprint {
	t.Helper()
	b, err := spec.ParseBlueprint([]byte(raw))
	if err != nil {
		t.Fatalf("parse blueprint: %v", err)
	}
	return b
}

func TestBlueprintFillsTheFieldsADomainDoesNotRestate(t *testing.T) {
	b := mustBlueprint(t, oneObject)
	s, err := b.Parse([]byte(domainSpec(
		`{"role":"entry","name":"patient","table":"patients","description":"One person the clinic treats."}`)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	o, ok := s.ByRole("entry")
	if !ok {
		t.Fatal("no object fills the entry role")
	}
	if len(o.Fields) != 3 {
		t.Fatalf("fields = %d, want the 3 the module declares", len(o.Fields))
	}
	if o.Name != "patient" || o.Table != "patients" {
		t.Errorf("name/table = %q/%q, want the domain's own", o.Name, o.Table)
	}
	if s.TableFor(o) != "clinic_record_patients" {
		t.Errorf("TableFor = %q", s.TableFor(o))
	}
	if len(o.Indexes) != 1 || o.Indexes[0].Name != "state" {
		t.Errorf("indexes = %v, want the module's own carried over", o.Indexes)
	}
}

func TestBlueprintDomainSetsADefaultAndNothingElse(t *testing.T) {
	b := mustBlueprint(t, oneObject)

	t.Run("a default is taken", func(t *testing.T) {
		s, err := b.Parse([]byte(domainSpec(
			`{"role":"entry","name":"patient","table":"patients","description":"One person the clinic treats.",
			  "fields":[{"name":"state","default":"open"}]}`)))
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		o, _ := s.ByRole("entry")
		for _, f := range o.Fields {
			if f.Name == "state" {
				if f.Default != "open" {
					t.Errorf("default = %q, want the domain's", f.Default)
				}
				if len(f.Values) != 3 {
					t.Errorf("values = %v, want the module's kept", f.Values)
				}
			}
		}
	})

	for _, tc := range []struct{ name, field, want string }{
		{"a type", `{"name":"state","type":"string"}`, "may set a default and nothing else"},
		{"a description", `{"name":"slug","description":"mine now"}`, "may set a default and nothing else"},
		{"more values", `{"name":"state","values":["pending","embargoed"]}`, "may set a default and nothing else"},
		{"a field the module does not declare", `{"name":"sector","type":"string","description":"x"}`, "declares no field"},
	} {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			_, err := b.Parse([]byte(domainSpec(
				`{"role":"entry","name":"patient","table":"patients","description":"One person the clinic treats.",
				  "fields":[` + tc.field + `]}`)))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestBlueprintRefusesARoleTheModuleDoesNotDeclare(t *testing.T) {
	b := mustBlueprint(t, oneObject)
	_, err := b.Parse([]byte(domainSpec(
		`{"role":"loan","name":"loan","table":"loans","description":"A loan."}`)))
	if err == nil || !strings.Contains(err.Error(), "which the module does not declare") {
		t.Fatalf("err = %v, want a refusal naming the unknown role", err)
	}
}

func TestBlueprintLeavesADomainsOwnObjectAlone(t *testing.T) {
	b := mustBlueprint(t, oneObject)
	s, err := b.Parse([]byte(domainSpec(
		`{"role":"entry","name":"patient","table":"patients","description":"One person the clinic treats."},
		 {"name":"visit","table":"visits","description":"One appointment the clinic keeps.",
		  "fields":[{"name":"id","type":"string","description":"Storage key.","primary":true},
		            {"name":"seen_at","type":"timestamp","description":"When it happened.","required":true}]}`)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	o, ok := s.Object("visit")
	if !ok {
		t.Fatal("the domain's own object is gone")
	}
	if len(o.Fields) != 2 {
		t.Errorf("fields = %d, want the domain's own 2", len(o.Fields))
	}
}

func TestBlueprintRefusesWhatIsTheDomainsToChoose(t *testing.T) {
	for _, tc := range []struct{ name, object, want string }{
		{"a name", `{"role":"entry","name":"patient","description":"x","fields":[{"name":"id","type":"string","description":"k","primary":true}]}`,
			`declares the name "patient", which is the domain's to choose`},
		{"a table", `{"role":"entry","table":"patients","description":"x","fields":[{"name":"id","type":"string","description":"k","primary":true}]}`,
			`declares the table "patients", which is the domain's to choose`},
		{"no role at all", `{"description":"x","fields":[{"name":"id","type":"string","description":"k","primary":true}]}`,
			"declares no role"},
	} {
		t.Run("refuses "+tc.name, func(t *testing.T) {
			_, err := spec.ParseBlueprint([]byte(`{"module":"record","objects":[` + tc.object + `]}`))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestBlueprintRefusesASpecForAnotherModule(t *testing.T) {
	b := mustBlueprint(t, oneObject)
	_, err := b.Parse([]byte(`{"module":"forms","domain":"clinic","instance":"clinic","objects":[
	  {"role":"entry","name":"patient","table":"patients","description":"One person the clinic treats."}]}`))
	if err == nil || !strings.Contains(err.Error(), "this is the blueprint for") {
		t.Fatalf("err = %v, want a refusal naming the module", err)
	}
}

func TestADefaultMustBeOneOfTheDeclaredValues(t *testing.T) {
	b := mustBlueprint(t, oneObject)
	_, err := b.Parse([]byte(domainSpec(
		`{"role":"entry","name":"patient","table":"patients","description":"One person the clinic treats.",
		  "fields":[{"name":"state","default":"embargoed"}]}`)))
	if err == nil || !strings.Contains(err.Error(), "which is not one of") {
		t.Fatalf("err = %v, want the default measured against the declared values", err)
	}
}

func TestARequiredFieldMayNotBeGivenADefault(t *testing.T) {
	b := mustBlueprint(t, oneObject)
	_, err := b.Parse([]byte(domainSpec(
		`{"role":"entry","name":"patient","table":"patients","description":"One person the clinic treats.",
		  "fields":[{"name":"slug","default":"untitled"}]}`)))
	if err == nil || !strings.Contains(err.Error(), "pass unnoticed") {
		t.Fatalf("err = %v, want a required field's default refused", err)
	}
}

func TestASpecThatNeedsTheBlueprintSaysSoWhenLoadedWithout(t *testing.T) {
	_, err := spec.Parse([]byte(domainSpec(
		`{"role":"entry","name":"patient","table":"patients","description":"One person the clinic treats."}`)))
	if err == nil || !strings.Contains(err.Error(), "through the module's blueprint") {
		t.Fatalf("err = %v, want the error to name the blueprint", err)
	}
}

func TestTheShippedFixturesLoadThroughTheirBlueprint(t *testing.T) {
	b := blueprint(t)
	for _, path := range fixtures {
		if _, err := b.Load(path); err != nil {
			t.Errorf("load %s: %v", path, err)
		}
	}
}
