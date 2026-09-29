package spec_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func clinicWithMount(t *testing.T, mount string) []byte {
	t.Helper()
	raw, err := os.ReadFile(clinicSpec)
	if err != nil {
		t.Fatalf("read %s: %v", clinicSpec, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode %s: %v", clinicSpec, err)
	}
	doc["mount"] = mount
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return out
}

func TestMountSegmentRefusesAnythingThatCouldBlurOrOverflowAPhysicalName(t *testing.T) {
	b := blueprint(t)

	honest, err := b.Parse(clinicWithMount(t, "second"))
	if err != nil {
		t.Fatalf("precondition: a plain mount segment should load, got %v", err)
	}
	patients, _ := honest.Object("patient")
	if got := honest.TableFor(patients); got != "clinic_record_second_patients" {
		t.Fatalf("precondition: TableFor = %q, want the mount as its own segment", got)
	}

	for _, mount := range []string{"a_b", "Second", "x-y", "main ", "a;drop", "a.b"} {
		if _, err := b.Parse(clinicWithMount(t, mount)); err == nil || !strings.Contains(err.Error(), "mount") {
			t.Errorf("mount %q loaded (err %v); a segment carrying anything outside [a-z0-9] must be refused by name", mount, err)
		}
	}

	if _, err := b.Parse(clinicWithMount(t, strings.Repeat("m", 40))); err == nil {
		t.Error("a mount long enough to push a physical name past 63 bytes loaded; Postgres would truncate it silently")
	}
}
