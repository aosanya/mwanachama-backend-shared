package spec_test

import (
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

func TestMigrateRecordsEveryPhysicalNameAgainstItsRawOne(t *testing.T) {
	db := open(t)
	clinic := load(t, clinicSpec)
	if err := spec.Migrate(db, clinic); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	for _, o := range clinic.Objects {
		rec, found, err := spec.LookupName(db, clinic.TableFor(o))
		if err != nil {
			t.Fatalf("lookup %s: %v", o.Name, err)
		}
		if !found {
			t.Fatalf("%q is in the database but not in the registry", clinic.TableFor(o))
		}
		if rec.Raw != clinic.RawNameFor(o) {
			t.Errorf("raw = %q, want %q", rec.Raw, clinic.RawNameFor(o))
		}
		if rec.Instance != clinic.Instance || rec.Module != clinic.Module || rec.Mount != spec.DefaultMount {
			t.Errorf("record = %+v, want it split out for querying", rec)
		}
	}
}

func TestTheRegistryIsTheOnlyWayBackFromAHashedName(t *testing.T) {
	db := open(t)
	clinic := load(t, clinicSpec)
	if err := spec.Migrate(db, clinic); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	o, _ := clinic.ByRole("entry")
	physical := clinic.TableFor(o)

	if _, _, err := spec.LookupName(db, physical); err != nil {
		t.Fatalf("lookup: %v", err)
	}
	for _, part := range []string{clinic.Module, "patients", spec.DefaultMount} {
		if contains(physical, part) {
			t.Errorf("%q still spells %q — nothing should be readable but the instance", physical, part)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) &&
		func() bool {
			for i := 0; i+len(needle) <= len(haystack); i++ {
				if haystack[i:i+len(needle)] == needle {
					return true
				}
			}
			return false
		}()
}

func TestMigratingTwiceLeavesOneRegistryRowPerTable(t *testing.T) {
	db := open(t)
	clinic := load(t, clinicSpec)
	for i := 0; i < 3; i++ {
		if err := spec.Migrate(db, clinic); err != nil {
			t.Fatalf("migrate %d: %v", i, err)
		}
	}
	var n int64
	if err := db.Table(clinic.NameRegistryTable()).Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if want := int64(len(clinic.Objects)); n != want {
		t.Errorf("registry rows = %d, want %d", n, want)
	}
}

func TestTwoMountsGetTwoRegistryRowsAndTwoTables(t *testing.T) {
	db := open(t)
	supplier := load(t, clinicSpec)
	supplier.Mount = "supplier"
	product := load(t, clinicSpec)
	product.Mount = "product"

	for _, s := range []*spec.Spec{supplier, product} {
		if err := spec.Migrate(db, s); err != nil {
			t.Fatalf("migrate %q: %v", s.Mount, err)
		}
	}

	o, _ := supplier.ByRole("entry")
	po, _ := product.ByRole("entry")
	if supplier.TableFor(o) == product.TableFor(po) {
		t.Fatal("two mounts hashed to one table")
	}

	var n int64
	if err := db.Table(supplier.NameRegistryTable()).Where("mount = ?", "supplier").Count(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if want := int64(len(supplier.Objects)); n != want {
		t.Errorf("supplier rows = %d, want %d", n, want)
	}
}

func TestOneMountsTablesSortTogether(t *testing.T) {
	supplier := load(t, clinicSpec)
	supplier.Mount = "supplier"
	product := load(t, clinicSpec)
	product.Mount = "product"

	for _, s := range []*spec.Spec{supplier, product} {
		keys := map[string]bool{}
		for _, o := range s.Objects {
			name := s.TableFor(o)
			keys[name[:len(s.Instance)+1+spec.MountHashLength]] = true
			t.Logf("mount=%-9s %-40s  %s", s.Mount, name, s.RawNameFor(o))
		}
		if len(keys) != 1 {
			t.Errorf("mount %q spans %d prefixes, so its tables do not sort together", s.Mount, len(keys))
		}
	}

	so, _ := supplier.ByRole("entry")
	po, _ := product.ByRole("entry")
	if supplier.TableFor(so) == product.TableFor(po) {
		t.Fatal("two mounts landed on one table")
	}
	if supplier.MountKey() == product.MountKey() {
		t.Error("two mounts share a group prefix")
	}
}

func TestTwoModulesUnderOneMountStayApart(t *testing.T) {
	a := &spec.Spec{Module: "agency", Instance: "agy1f2e3d4c"}
	tm := &spec.Spec{Module: "taskmanager", Instance: "agy1f2e3d4c"}
	o := spec.Object{Name: "work_item", Table: "work_items"}

	if a.MountKey() != tm.MountKey() {
		t.Error("the default mount should group both modules")
	}
	if a.TableFor(o) == tm.TableFor(o) {
		t.Fatalf("both modules' work_items became %q", a.TableFor(o))
	}
	t.Logf("agency      %s", a.TableFor(o))
	t.Logf("taskmanager %s", tm.TableFor(o))
	if n := len(a.TableFor(o)); n != len(a.Instance)+1+spec.MountHashLength+1+spec.HashLength {
		t.Errorf("table is %d bytes, not instance + mount key + object hash", n)
	}
}
