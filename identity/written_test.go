package identity_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
	"github.com/aosanya/mwanachama-backend-shared/identity"
)

func publishReport() identity.Written {
	return identity.Written{
		Partition: "agy1f2e3d4c",
		Owner:     "agency",
		Objects: []identity.WrittenObject{
			{Object: "draft", ID: "0b9e6f1c-6a3e-4c55-9d1e-3f0f6f2b8a10"},
			{Object: "role_assignment", ID: "c2d4e6f8-1a3b-4c5d-8e9f-0a1b2c3d4e5f"},
		},
	}
}

func TestAWriteReportCarriesThePartitionTheOwnerAndTheObjects(t *testing.T) {
	if err := publishReport().Check(); err != nil {
		t.Fatalf("a well-formed report: %v", err)
	}
	git := identity.Written{
		Partition: "agy1f2e3d4c",
		Owner:     "git",
		Objects:   []identity.WrittenObject{{Object: "blob", ID: "internal/store.go"}},
	}
	if err := git.Check(); err != nil {
		t.Fatalf("an id may carry slashes, as git's path does: %v", err)
	}
}

func TestAWriteReportIsRefusedByName(t *testing.T) {
	cases := map[string]func(*identity.Written){
		"empty partition":        func(w *identity.Written) { w.Partition = "" },
		"partition off alphabet": func(w *identity.Written) { w.Partition = "Agy_1" },
		"empty owner":            func(w *identity.Written) { w.Owner = "" },
		"owner off alphabet":     func(w *identity.Written) { w.Owner = "agency.role" },
		"no objects":             func(w *identity.Written) { w.Objects = nil },
		"empty object":           func(w *identity.Written) { w.Objects[1].Object = "" },
		"object off alphabet":    func(w *identity.Written) { w.Objects[1].Object = "Role" },
		"empty id":               func(w *identity.Written) { w.Objects[0].ID = "" },
		"id edge whitespace":     func(w *identity.Written) { w.Objects[0].ID = " R-3" },
		"id control character":   func(w *identity.Written) { w.Objects[0].ID = "R-3\n" },
	}
	for name, spoil := range cases {
		w := publishReport()
		w.Objects = append([]identity.WrittenObject(nil), w.Objects...)
		spoil(&w)
		if err := w.Check(); !errors.Is(err, identity.ErrInvalidWritten) {
			t.Errorf("%s: Check() = %v, want ErrInvalidWritten", name, err)
		}
	}
}

func TestAReportedObjectNamesWhatARefersMarkNames(t *testing.T) {
	for _, o := range publishReport().Objects {
		refers := o.Refers("agency")
		if !dispatch.RefersPattern.MatchString(refers) {
			t.Errorf("%q must be a refers mark dispatch accepts", refers)
		}
	}
	for _, object := range []string{"role", "work_item", "a1", "Role", "1role", "work-item", "role.x", ""} {
		got := identity.ObjectPattern.MatchString(object)
		want := dispatch.RefersPattern.MatchString("agency." + object)
		if got != want {
			t.Errorf("object %q: ObjectPattern says %v, the object half of RefersPattern says %v", object, got, want)
		}
	}
}

func TestAWriteReportTravelsAsJSONUnderItsOwnNames(t *testing.T) {
	raw, err := json.Marshal(publishReport())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, k := range []string{"partition", "owner", "objects"} {
		if _, ok := fields[k]; !ok {
			t.Errorf("the wire form lacks %q: %s", k, raw)
		}
	}
	var back identity.Written
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if !reflect.DeepEqual(back, publishReport()) {
		t.Fatalf("round trip changed the report: %+v", back)
	}
}

type recordingReactor struct{ got []identity.Written }

func (r *recordingReactor) Wrote(ctx context.Context, w identity.Written) error {
	if err := w.Check(); err != nil {
		return err
	}
	r.got = append(r.got, w)
	return nil
}

func TestAReactorIsHandedExactlyTheObjectsTheHostWrote(t *testing.T) {
	r := &recordingReactor{}
	var reactor identity.Reactor = r
	if err := reactor.Wrote(context.Background(), publishReport()); err != nil {
		t.Fatalf("wrote: %v", err)
	}
	if len(r.got) != 1 || !reflect.DeepEqual(r.got[0].Objects, publishReport().Objects) {
		t.Fatalf("the reactor saw %+v", r.got)
	}
	if err := reactor.Wrote(context.Background(), identity.Written{Partition: "agy1", Owner: "agency"}); !errors.Is(err, identity.ErrInvalidWritten) {
		t.Fatalf("an empty report must be refused, not read as the whole owner: %v", err)
	}
}
