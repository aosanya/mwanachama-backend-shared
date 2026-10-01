package spec_test

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

func columnTypeDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ddl := `create table shapes (
		a text,
		b timestamp,
		c integer,
		d blob,
		e double precision,
		f character varying(20)
	)`
	if err := db.Exec(ddl).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	return db
}

func TestColumnIsTextualReadsTheStoredTypeNotTheDeclaredOne(t *testing.T) {
	db := columnTypeDB(t)

	for _, tc := range []struct {
		column string
		want   bool
	}{
		{"a", true},
		{"f", true},
		{"b", false},
		{"c", false},
		{"d", false},
		{"e", false},
	} {
		got, err := spec.ColumnIsTextual(db, "shapes", tc.column)
		if err != nil {
			t.Fatalf("ColumnIsTextual(shapes.%s): %v", tc.column, err)
		}
		if got != tc.want {
			t.Errorf("ColumnIsTextual(shapes.%s) = %v, want %v", tc.column, got, tc.want)
		}
	}
}

func TestColumnIsTextualRefusesAColumnThatIsNotThere(t *testing.T) {
	db := columnTypeDB(t)
	if _, err := spec.ColumnIsTextual(db, "shapes", "nope"); err == nil {
		t.Fatal("ColumnIsTextual accepted a column the table does not have")
	}
}

func TestUnsetSQLDropsTheEmptyStringTestForANonTextColumn(t *testing.T) {
	if got, want := spec.UnsetSQL("retired_at", true), "(retired_at IS NULL OR retired_at = '')"; got != want {
		t.Errorf("textual = %q, want %q", got, want)
	}
	if got, want := spec.UnsetSQL("retired_at", false), "(retired_at IS NULL)"; got != want {
		t.Errorf("non-text = %q, want %q", got, want)
	}
}

func TestDeclaresTextualColumnFollowsTheEmittedColumnType(t *testing.T) {
	o := spec.Object{
		Name: "salt",
		Fields: []spec.Field{
			{Name: "set_by", Type: spec.TypeString},
			{Name: "retired_at", Type: spec.TypeTimestamp},
			{Name: "id", Type: spec.TypeInt},
			{Name: "secret", Type: spec.TypeBytes},
			{Name: "doc", Type: spec.TypeJSON},
		},
	}
	for _, tc := range []struct {
		column string
		want   bool
	}{
		{"set_by", true},
		{"retired_at", true},
		{"id", false},
		{"secret", false},
		{"doc", false},
	} {
		if got := spec.DeclaresTextualColumn(o, tc.column, "postgres"); got != tc.want {
			t.Errorf("DeclaresTextualColumn(%s) = %v, want %v", tc.column, got, tc.want)
		}
	}
}

func TestUnsetClauseFallsBackToTheDeclarationWhenTheTableIsNotThere(t *testing.T) {
	db := columnTypeDB(t)
	o := spec.Object{
		Name:   "salt",
		Fields: []spec.Field{{Name: "retired_at", Type: spec.TypeTimestamp}},
	}
	got := spec.UnsetClause(db, o, "absent_table", "retired_at")
	if want := "(retired_at IS NULL OR retired_at = '')"; got != want {
		t.Errorf("UnsetClause = %q, want the declared form %q", got, want)
	}
}
