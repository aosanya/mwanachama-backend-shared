package dispatch_test

import (
	"context"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
	"github.com/aosanya/mwanachama-backend-shared/specstore"
)

// Loophole #4/#8, widened: spec.Validate only ever checks one spec's own
// object list for a physical-name collision (its seenPhysical map is local
// to one Validate call). Two independently-loaded, independently-valid
// specs that happen to agree on instance+module+table are never compared to
// each other by anything in this package, so spec.Migrate silently shares
// one physical table between what are, to their own callers, two unrelated
// domains.
func TestS24_TwoUnrelatedSpecsSharingATableAreNeverCompared(t *testing.T) {
	db := openWidgetDB(t)

	widgetBP, err := spec.ParseBlueprint([]byte(widgetBlueprintJSON))
	if err != nil {
		t.Fatalf("ParseBlueprint widget: %v", err)
	}
	widgetSpec, err := widgetBP.Parse([]byte(widgetDomainSpec("acme")))
	if err != nil {
		t.Fatalf("widget bp.Parse: %v", err)
	}

	const gadgetBlueprintJSON = `{
  "module": "widgetworks",
  "objects": [{"role": "gadget", "description": "an unrelated second thing",
    "fields": [
      {"name": "id", "type": "string", "primary": true, "description": "the gadget's key"},
      {"name": "code", "type": "string", "required": true, "description": "a caller-facing code"}
    ]}]
}`
	const gadgetDomainSpecJSON = `{
  "module": "widgetworks", "domain": "gadgetworks", "instance": "acme",
  "objects": [{"name": "gadget", "role": "gadget", "table": "widgets", "description": "reuses the widget table by accident", "fields": [], "indexes": []}]
}`
	gadgetBP, err := spec.ParseBlueprint([]byte(gadgetBlueprintJSON))
	if err != nil {
		t.Fatalf("ParseBlueprint gadget: %v", err)
	}
	gadgetSpec, err := gadgetBP.Parse([]byte(gadgetDomainSpecJSON))
	if err != nil {
		t.Fatalf("gadget bp.Parse: %v", err)
	}

	if widgetSpec.TableFor(widgetSpec.Objects[0]) != gadgetSpec.TableFor(gadgetSpec.Objects[0]) {
		t.Fatalf("fixture setup is wrong: the two specs must land on the same physical table")
	}

	if err := spec.Migrate(db, widgetSpec); err != nil {
		t.Fatalf("migrate widget: %v", err)
	}
	// Nothing here refuses this, even though gadgetworks and widgetworks are
	// two unrelated modules that never agreed to share storage.
	if err := spec.Migrate(db, gadgetSpec); err != nil {
		t.Fatalf("migrate gadget: %v", err)
	}

	wm := newWidgetManager(t, db, widgetSpec)
	if _, err := wm.Create(context.Background(), widgetRow{Code: "W-1"}); err != nil {
		t.Fatalf("create widget: %v", err)
	}

	type gadgetRow struct {
		ID   string
		Code string
	}
	gst, err := specstore.New(db, gadgetSpec, map[string]any{"gadget": gadgetRow{}})
	if err != nil {
		t.Fatalf("specstore.New gadget: %v", err)
	}
	rows, err := specstore.List[gadgetRow](gst, gst.Query(context.Background(), "gadget"), "gadget")
	if err != nil {
		t.Fatalf("list gadget: %v", err)
	}
	if len(rows) != 1 || rows[0].Code != "W-1" {
		t.Fatalf("gadgetworks's own List saw %+v; want to see widgetworks's row, proving the shared table leaks across the two unrelated domains", rows)
	}
}
