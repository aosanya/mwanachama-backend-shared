package spec_test

import (
	"encoding/json"
	"testing"
)

func clinicWithVisitTable(t *testing.T, table, mount string) []byte {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(clinicWithMount(t, mount), &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if mount == "" {
		delete(doc, "mount")
	}
	for _, o := range doc["objects"].([]any) {
		if obj := o.(map[string]any); obj["name"] == "visit" {
			obj["table"] = table
		}
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return out
}

func TestS28_OpenHole_MountNameSpellsTheDefaultMountsTable(t *testing.T) {
	b := blueprint(t)

	def, err := b.Parse(clinicWithVisitTable(t, "patient_page_views", ""))
	if err != nil {
		t.Fatalf("precondition: the default mount should load, got %v", err)
	}
	visit, ok := def.Object("visit")
	if !ok {
		t.Fatal("precondition: the default-mount spec declares no visit object")
	}

	second, err := b.Parse(clinicWithVisitTable(t, "patient_page_views", "patient"))
	if err != nil {
		return
	}
	pageView, ok := second.Object("page_view")
	if !ok {
		t.Fatal("precondition: the mounted spec declares no page_view object")
	}

	if got, owned := second.TableFor(pageView), def.TableFor(visit); got == owned {
		t.Errorf("S28 (P2): mount %q loaded and its page_view lands in %q, the physical table the default mount's visit already owns; "+
			"a mount whose name plus one of the spec's tables spells another of its default-mount tables must be refused at load, "+
			"or the mount must be its own segment so the two cannot spell the same name",
			second.MountName(), got)
	}
}
