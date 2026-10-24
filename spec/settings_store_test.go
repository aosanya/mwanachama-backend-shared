package spec_test

import (
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

func settingsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	return db
}

func settingsSpec(t *testing.T, instance string, withDeclared bool) *spec.Spec {
	t.Helper()
	raw := oneObject
	if withDeclared {
		raw = withSettings(recordSettings)
	}
	s, err := mustBlueprint(t, raw).Parse([]byte(strings.Replace(
		domainSpec(`{"name":"entry","role":"entry","description":"One entry."}`),
		`"instance":"clinic"`, `"instance":"`+instance+`"`, 1)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return s
}

func migrated(t *testing.T, db *gorm.DB, s *spec.Spec) {
	t.Helper()
	for i := 0; i < 2; i++ {
		if err := spec.Migrate(db, s); err != nil {
			t.Fatalf("migrate %d: %v", i, err)
		}
	}
}

func TestMigrateCreatesTheSettingsTablesOnlyWhenSettingsAreDeclared(t *testing.T) {
	db := settingsDB(t)
	with := settingsSpec(t, "clinic", true)
	without := settingsSpec(t, "ward", false)
	migrated(t, db, with)
	migrated(t, db, without)

	tables := with.SettingsTables()
	if len(tables) != 3 {
		t.Fatalf("settings tables = %v, want 3", tables)
	}
	for _, table := range tables {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("%s was not created", table)
		}
		rec, ok, err := spec.LookupName(db, table)
		if err != nil || !ok {
			t.Fatalf("%s is not in the registry: %v %v", table, ok, err)
		}
		if rec.Module != "record" || !strings.HasPrefix(rec.Raw, "record_main_spec_settings_") {
			t.Fatalf("registry row = %+v", rec)
		}
	}
	if got := without.SettingsTables(); len(got) != 0 {
		t.Fatalf("a module with no settings has settings tables %v", got)
	}
	if n := len(without.NameRecords()); n != len(without.Objects) {
		t.Fatalf("name records = %d, want only the objects' %d", n, len(without.Objects))
	}
}

func TestStoredSettingsRoundTripThroughEveryLayer(t *testing.T) {
	db := settingsDB(t)
	s := settingsSpec(t, "clinic", true)
	migrated(t, db, s)

	if err := spec.PutDefinitionSettings(db, s, map[string]any{"code_length": 8, "language": "sw"}); err != nil {
		t.Fatalf("definition: %v", err)
	}
	e, err := spec.ResolveStored(db, s, "code_length")
	if err != nil || e.Layer != spec.LayerDefinition || e.Value != float64(8) {
		t.Fatalf("code_length = %+v, %v", e, err)
	}

	change, err := spec.StoreOverride(db, s, "code_length", 4, "owner-1", "2026-10-24T07:00:00Z")
	if err != nil {
		t.Fatalf("override: %v", err)
	}
	if change.To != 4 || change.From != nil {
		t.Fatalf("change = %+v", change)
	}
	if _, err := spec.StoreOverride(db, s, "code_length", 5, "owner-1", "2026-10-24T07:01:00Z"); err != nil {
		t.Fatalf("second override: %v", err)
	}
	e, err = spec.ResolveStored(db, s, "code_length")
	if err != nil || e.Layer != spec.LayerOverride || e.Value != float64(5) || e.By != "owner-1" || e.At != "2026-10-24T07:01:00Z" {
		t.Fatalf("code_length = %+v, %v", e, err)
	}

	cleared, ok, err := spec.StoreClearOverride(db, s, "code_length", "owner-2", "2026-10-24T07:02:00Z")
	if err != nil || !ok || !cleared.Cleared || cleared.From != float64(5) {
		t.Fatalf("clear = %+v, %v, %v", cleared, ok, err)
	}
	if _, ok, err := spec.StoreClearOverride(db, s, "code_length", "owner-2", "2026-10-24T07:03:00Z"); err != nil || ok {
		t.Fatalf("clearing nothing = %v, %v", ok, err)
	}
	e, _ = spec.ResolveStored(db, s, "code_length")
	if e.Layer != spec.LayerDefinition {
		t.Fatalf("after clear, layer = %s", e.Layer)
	}
	all, err := spec.ResolveAllStored(db, s)
	if err != nil || len(all) != 3 || all[0].Layer != spec.LayerModule {
		t.Fatalf("all = %+v, %v", all, err)
	}

	log, err := spec.SettingChanges(db, s)
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	if len(log) != 3 {
		t.Fatalf("log = %+v, want 3 entries", log)
	}
	if log[0].To != float64(4) || log[1].From != float64(4) || log[1].To != float64(5) || !log[2].Cleared || log[2].By != "owner-2" {
		t.Fatalf("log = %+v", log)
	}
}

func TestAStoredValueOffItsShapeIsRefused(t *testing.T) {
	db := settingsDB(t)
	s := settingsSpec(t, "clinic", true)
	migrated(t, db, s)

	if err := spec.PutDefinitionSettings(db, s, map[string]any{"code_length": 1.5}); err == nil {
		t.Fatal("a definition value off its shape was stored")
	}
	if err := spec.PutDefinitionSettings(db, s, map[string]any{"colour": "red"}); err == nil {
		t.Fatal("an undeclared setting was stored")
	}
	if _, err := spec.StoreOverride(db, s, "language", "fr", "owner-1", "2026-10-24T07:00:00Z"); err == nil {
		t.Fatal("an override off its enum was stored")
	}
	if _, err := spec.StoreOverride(db, s, "language", "sw", "", ""); err == nil {
		t.Fatal("an unattributed override was stored")
	}
	_, overrides, err := spec.LoadSettingLayers(db, s)
	if err != nil || len(overrides) != 0 {
		t.Fatalf("overrides = %v, %v", overrides, err)
	}
	if log, _ := spec.SettingChanges(db, s); len(log) != 0 {
		t.Fatalf("a refused change was logged: %+v", log)
	}
}

func TestTwoInstancesDoNotSeeEachOthersOverrides(t *testing.T) {
	db := settingsDB(t)
	a := settingsSpec(t, "clinic", true)
	b := settingsSpec(t, "ward", true)
	migrated(t, db, a)
	migrated(t, db, b)

	if _, err := spec.StoreOverride(db, a, "approval", true, "owner-a", "2026-10-24T07:00:00Z"); err != nil {
		t.Fatalf("override: %v", err)
	}
	e, err := spec.ResolveStored(db, b, "approval")
	if err != nil || e.Layer != spec.LayerModule || e.Value != false {
		t.Fatalf("ward approval = %+v, %v", e, err)
	}
	if log, _ := spec.SettingChanges(db, b); len(log) != 0 {
		t.Fatalf("ward sees clinic's log: %+v", log)
	}
}

func TestTheStoreRefusesAModuleWithNoSettings(t *testing.T) {
	db := settingsDB(t)
	s := settingsSpec(t, "ward", false)
	migrated(t, db, s)
	if _, _, err := spec.LoadSettingLayers(db, s); err == nil {
		t.Fatal("loaded settings for a module that declares none")
	}
}

func TestAModuleObjectCannotTakeASettingsTable(t *testing.T) {
	_, err := mustBlueprint(t, oneObject).Parse([]byte(domainSpec(
		`{"name":"entry","role":"entry","table":"spec_settings_override","description":"One entry."}`)))
	if err == nil {
		t.Fatal("an object took a table reserved for settings storage")
	}
}
