package spec_test

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
	"github.com/aosanya/mwanachama-backend-shared/spec"
)

func attachedSpec(attach string) []byte {
	return []byte(`{"module":"record","domain":"clinic","instance":"clinic","objects":[` +
		`{"role":"entry","name":"patient","table":"patients","description":"One person the clinic treats."}` +
		`],"attach":[` + attach + `]}`)
}

func TestADomainAttachesAnotherModuleToOneOfItsRoles(t *testing.T) {
	b := mustBlueprint(t, oneObject)
	s, err := b.Parse(attachedSpec(`{"module":"ratings","items":"record.entry"}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []spec.Attachment{{Module: "ratings", Items: "record.entry"}}
	if !reflect.DeepEqual(s.Attach, want) {
		t.Fatalf("attach = %+v, want %+v", s.Attach, want)
	}
	if got := s.Attached("ratings"); len(got) != 1 || got[0].ItemsRole() != "entry" {
		t.Fatalf("Attached(ratings) = %+v", got)
	}
	if got := s.Attached("forms"); len(got) != 0 {
		t.Fatalf("nothing attached forms, got %+v", got)
	}
	o, ok := s.ByRole(s.Attach[0].ItemsRole())
	if !ok || o.Name != "patient" {
		t.Fatalf("the attached role resolves to this spec's own object, got %+v %v", o, ok)
	}
}

func TestAnAttachmentIsRefusedByName(t *testing.T) {
	b := mustBlueprint(t, oneObject)
	cases := map[string]struct{ attach, says string }{
		"module off alphabet":   {`{"module":"Ratings","items":"record.entry"}`, "not a usable name segment"},
		"module empty":          {`{"module":"","items":"record.entry"}`, "not a usable name segment"},
		"attaches itself":       {`{"module":"record","items":"record.entry"}`, "cannot attach to itself"},
		"items not module.role": {`{"module":"ratings","items":"entry"}`, "not <module>.<role>"},
		"items another module":  {`{"module":"ratings","items":"agency.entry"}`, "an attachment points at this spec's own objects"},
		"items undeclared role": {`{"module":"ratings","items":"record.page_view"}`, "no object in this spec claims"},
		"items by object name":  {`{"module":"ratings","items":"record.patient"}`, "no object in this spec claims"},
		"twice": {`{"module":"ratings","items":"record.entry"},{"module":"ratings","items":"record.entry"}`,
			"attached to \"record.entry\" twice"},
		"unknown field": {`{"module":"ratings","items":"record.entry","mount":"x"}`, "unknown field"},
	}
	for name, c := range cases {
		_, err := b.Parse(attachedSpec(c.attach))
		if err == nil || !strings.Contains(err.Error(), c.says) {
			t.Errorf("%s: %v, want an error saying %q", name, err, c.says)
		}
	}
}

func TestASpecWithNoAttachmentIsUnchanged(t *testing.T) {
	for _, path := range fixtures {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		b, err := spec.LoadBlueprint("testdata/record.blueprint.json")
		if err != nil {
			t.Fatalf("blueprint: %v", err)
		}
		s, err := b.Parse(raw)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if s.Attach != nil {
			t.Fatalf("%s: attach = %+v, want none", path, s.Attach)
		}
		out, err := json.Marshal(s)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if strings.Contains(string(out), `"attach"`) {
			t.Fatalf("%s: a spec with no attachment must not grow an attach key: %s", path, out)
		}
	}
}

func TestAnAttachmentTargetIsSpelledAsARefersMark(t *testing.T) {
	for _, target := range []string{"catalog.entry", "agency.work_item", "a.b", "catalog", "Catalog.entry", "catalog.entry.x", "catalog.", ".entry", ""} {
		if got, want := spec.AttachTargetPattern.MatchString(target), dispatch.RefersPattern.MatchString(target); got != want {
			t.Errorf("%q: AttachTargetPattern says %v, dispatch.RefersPattern says %v", target, got, want)
		}
	}
}
