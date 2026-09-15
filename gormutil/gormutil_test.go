package gormutil_test

import (
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/gormutil"
)

var (
	errReference = errors.New("invalid reference")
	errConflict  = errors.New("already exists")
)

func wrapReference(field string) error { return &wrappedFieldErr{errReference, field} }
func wrapConflict(field string) error  { return &wrappedFieldErr{errConflict, field} }

type wrappedFieldErr struct {
	err   error
	field string
}

func (e *wrappedFieldErr) Error() string { return e.err.Error() + ": " + e.field }
func (e *wrappedFieldErr) Unwrap() error { return e.err }

func TestClassifyError_Nil(t *testing.T) {
	if got := gormutil.ClassifyError(nil, wrapReference, wrapConflict); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestClassifyError_RecordNotFoundPassesThrough(t *testing.T) {
	got := gormutil.ClassifyError(gorm.ErrRecordNotFound, wrapReference, wrapConflict)
	if !errors.Is(got, gorm.ErrRecordNotFound) {
		t.Errorf("got %v, want gorm.ErrRecordNotFound", got)
	}
}

func TestClassifyError_UnrelatedErrorPassesThrough(t *testing.T) {
	unrelated := errors.New("connection refused")
	got := gormutil.ClassifyError(unrelated, wrapReference, wrapConflict)
	if !errors.Is(got, unrelated) {
		t.Errorf("got %v, want %v unchanged", got, unrelated)
	}
}

func TestClassifyError_PostgresUniqueViolation(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "23505", ConstraintName: "actors_email_uniq"}
	got := gormutil.ClassifyError(pgErr, wrapReference, wrapConflict)
	if !errors.Is(got, errConflict) {
		t.Errorf("got %v, want errConflict", got)
	}
	if got.Error() != "already exists: actors_email_uniq" {
		t.Errorf("got %q, want field carried through", got.Error())
	}
}

func TestClassifyError_PostgresForeignKeyAndNotNullAndCheckViolations(t *testing.T) {
	for _, code := range []string{"23503", "23502", "23514"} {
		pgErr := &pgconn.PgError{Code: code, ColumnName: "group_id"}
		got := gormutil.ClassifyError(pgErr, wrapReference, wrapConflict)
		if !errors.Is(got, errReference) {
			t.Errorf("code %s: got %v, want errReference", code, got)
		}
	}
}

func TestClassifyError_PostgresUnmappedCodePassesThrough(t *testing.T) {
	pgErr := &pgconn.PgError{Code: "40001"} // serialization_failure, not a constraint violation
	got := gormutil.ClassifyError(pgErr, wrapReference, wrapConflict)
	if !errors.Is(got, pgErr) {
		t.Errorf("got %v, want the pgErr unchanged", got)
	}
}

func TestClassifyError_PostgresConstraintNameFallsBackToColumnThenTable(t *testing.T) {
	got := gormutil.ClassifyError(&pgconn.PgError{Code: "23505", TableName: "actors"}, wrapReference, wrapConflict)
	if got.Error() != "already exists: actors" {
		t.Errorf("got %q, want table name fallback", got.Error())
	}
}

func TestClassifyError_SqliteUniqueViolation(t *testing.T) {
	sqliteErr := errors.New("UNIQUE constraint failed: actors.email")
	got := gormutil.ClassifyError(sqliteErr, wrapReference, wrapConflict)
	if !errors.Is(got, errConflict) {
		t.Errorf("got %v, want errConflict", got)
	}
	if got.Error() != "already exists: actors.email" {
		t.Errorf("got %q, want field pulled from the sqlite message", got.Error())
	}
}

func TestClassifyError_SqliteForeignKeyNotNullAndCheckViolations(t *testing.T) {
	msgs := []string{
		"FOREIGN KEY constraint failed: groups.parent_id",
		"NOT NULL constraint failed: actors.name",
		"CHECK constraint failed: actors",
	}
	for _, msg := range msgs {
		got := gormutil.ClassifyError(errors.New(msg), wrapReference, wrapConflict)
		if !errors.Is(got, errReference) {
			t.Errorf("msg %q: got %v, want errReference", msg, got)
		}
	}
}

func TestClassifyError_SqliteUnmappedMessagePassesThrough(t *testing.T) {
	unrelated := errors.New("database is locked")
	got := gormutil.ClassifyError(unrelated, wrapReference, wrapConflict)
	if !errors.Is(got, unrelated) {
		t.Errorf("got %v, want unrelated unchanged", got)
	}
}

type widgetRow struct {
	ID      string `gorm:"primaryKey"`
	Tag     string
	Deleted bool
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	if err := db.AutoMigrate(&widgetRow{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return db
}

func TestSyncPartialUniqueIndex_EnforcesUniquenessOnlyUnderWhere(t *testing.T) {
	db := openTestDB(t)
	if err := gormutil.SyncPartialUniqueIndex(db, "widget_rows", "tag", "(tag)", "deleted = false AND tag <> ''"); err != nil {
		t.Fatalf("SyncPartialUniqueIndex: %v", err)
	}

	if err := db.Create(&widgetRow{ID: "1", Tag: "a"}).Error; err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if err := db.Create(&widgetRow{ID: "2", Tag: "a"}).Error; err == nil {
		t.Fatal("expected a duplicate active tag to be rejected")
	}
	// A deleted row's tag doesn't collide — the WHERE clause excludes it.
	if err := db.Create(&widgetRow{ID: "3", Tag: "a", Deleted: true}).Error; err != nil {
		t.Fatalf("deleted-row insert: %v", err)
	}
	// An empty tag doesn't collide either.
	if err := db.Create(&widgetRow{ID: "4", Tag: ""}).Error; err != nil {
		t.Fatalf("first empty-tag insert: %v", err)
	}
	if err := db.Create(&widgetRow{ID: "5", Tag: ""}).Error; err != nil {
		t.Fatalf("second empty-tag insert: %v", err)
	}
}

func TestSyncPartialUniqueIndex_IdempotentAcrossRepeatedMigrate(t *testing.T) {
	db := openTestDB(t)
	for i := 0; i < 2; i++ {
		if err := gormutil.SyncPartialUniqueIndex(db, "widget_rows", "tag", "(tag)", "deleted = false AND tag <> ''"); err != nil {
			t.Fatalf("SyncPartialUniqueIndex run %d: %v", i, err)
		}
	}
}

func TestSetUUIDIfEmpty_MintsOnlyWhenEmpty(t *testing.T) {
	var id string
	gormutil.SetUUIDIfEmpty(&id)
	if id == "" {
		t.Fatal("expected a minted UUID, got empty string")
	}
	minted := id
	gormutil.SetUUIDIfEmpty(&id)
	if id != minted {
		t.Errorf("SetUUIDIfEmpty overwrote an already-set id: got %q, want %q", id, minted)
	}
}
