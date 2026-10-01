package spec_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
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

func patientTableUnderMount(t *testing.T, b *spec.Blueprint, mount string) string {
	t.Helper()
	s, err := b.Parse(clinicWithMount(t, mount))
	if err != nil {
		t.Fatalf("precondition: mount %q should load, got %v", mount, err)
	}
	patients, ok := s.Object("patient")
	if !ok {
		t.Fatalf("precondition: mount %q declares no patient object", mount)
	}
	return s.TableFor(patients)
}

func TestMountSegmentRefusesAnythingThatCouldBlurOrOverflowAPhysicalName(t *testing.T) {
	b := blueprint(t)

	second := patientTableUnderMount(t, b, "second")
	third := patientTableUnderMount(t, b, "third")
	if second == third {
		t.Fatalf("precondition: mounts \"second\" and \"third\" both put patient in %q, so the mount is not its own segment", second)
	}

	for _, mount := range []string{"a_b", "Second", "x-y", "main ", "a;drop", "a.b"} {
		if _, err := b.Parse(clinicWithMount(t, mount)); err == nil || !strings.Contains(err.Error(), "mount") {
			t.Errorf("mount %q loaded (err %v); a segment carrying anything outside [a-z0-9] must be refused by name", mount, err)
		}
	}

	long := patientTableUnderMount(t, b, strings.Repeat("m", 40))
	if len(long) > spec.MaxIdentifier {
		t.Errorf("a 40-character mount put patient in %q, %d bytes against the %d-byte limit; Postgres would truncate it silently",
			long, len(long), spec.MaxIdentifier)
	}
}
