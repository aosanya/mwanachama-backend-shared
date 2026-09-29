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
	visit, _ := def.Object("visit")
	if got := def.TableFor(visit); got != "clinic_record_patient_page_views" {
		t.Fatalf("precondition: default visit table = %q", got)
	}

	second, err := b.Parse(clinicWithVisitTable(t, "patient_page_views", "patient"))
	if err != nil {
		return
	}
	pageView, _ := second.Object("page_view")
	t.Errorf("S28 (P2): mount %q loaded and its page_view lands in %q, the physical table the default mount's visit already owns; "+
		"a mount whose name plus one of the spec's tables spells another of its default-mount tables must be refused at load",
		second.MountName(), second.TableFor(pageView))
}
