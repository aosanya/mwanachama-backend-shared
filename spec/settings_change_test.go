package spec_test

import (
	"errors"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

func TestSetOverrideRecordsWhoChangedWhatAndLeavesTheOldMapAlone(t *testing.T) {
	settings := mustBlueprint(t, withSettings(recordSettings)).Settings
	before := map[string]spec.Override{"language": {Value: "sw", By: "owner-1", At: "2026-10-22T09:00:00Z"}}

	after, change, err := spec.SetOverride(settings, before, "language", "en", "owner-2", "2026-10-22T10:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if change.From != "sw" || change.To != "en" || change.By != "owner-2" || change.At == "" || change.Cleared {
		t.Fatalf("change = %+v", change)
	}
	if before["language"].Value != "sw" {
		t.Fatalf("the caller's overrides were edited: %+v", before)
	}
	got, _ := spec.Resolve(settings, nil, after, "language")
	if got.Layer != spec.LayerOverride || got.Value != "en" || got.By != "owner-2" {
		t.Fatalf("resolved = %+v", got)
	}
}

func TestSetOverrideRefusesWhatResolveWouldPassOver(t *testing.T) {
	settings := mustBlueprint(t, withSettings(recordSettings)).Settings
	if _, _, err := spec.SetOverride(settings, nil, "language", "fr", "owner-1", "2026-10-22T10:00:00Z"); err == nil {
		t.Fatal("a value off its shape was accepted")
	}
	if _, _, err := spec.SetOverride(settings, nil, "colour", "red", "owner-1", "2026-10-22T10:00:00Z"); err == nil {
		t.Fatal("an undeclared setting was accepted")
	}
	if _, _, err := spec.SetOverride(settings, nil, "language", "en", "", "2026-10-22T10:00:00Z"); !errors.Is(err, spec.ErrSettingChangeUnattributed) {
		t.Fatalf("an unattributed change = %v", err)
	}
}

func TestClearOverrideFallsBackToTheDefinitionAndLogsTheReset(t *testing.T) {
	settings := mustBlueprint(t, withSettings(recordSettings)).Settings
	before := map[string]spec.Override{"language": {Value: "en", By: "owner-1", At: "2026-10-22T09:00:00Z"}}

	after, change, cleared, err := spec.ClearOverride(settings, before, "language", "owner-2", "2026-10-22T10:00:00Z")
	if err != nil || !cleared {
		t.Fatalf("cleared = %v, err = %v", cleared, err)
	}
	if !change.Cleared || change.From != "en" || change.By != "owner-2" {
		t.Fatalf("change = %+v", change)
	}
	if _, ok := before["language"]; !ok {
		t.Fatal("the caller's overrides were edited")
	}
	got, _ := spec.Resolve(settings, map[string]any{"language": "sw"}, after, "language")
	if got.Layer != spec.LayerDefinition || got.Value != "sw" {
		t.Fatalf("resolved = %+v", got)
	}
	if _, _, cleared, err := spec.ClearOverride(settings, after, "language", "owner-2", "2026-10-22T10:01:00Z"); err != nil || cleared {
		t.Fatalf("clearing nothing: cleared = %v, err = %v", cleared, err)
	}
}
