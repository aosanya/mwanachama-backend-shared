// Package gormutil is the small set of GORM-adjacent helpers every
// GORM-backed repo in this family independently retyped: driver-error
// classification, a dialect-aware partial-unique-index sync (layered on
// top of AutoMigrate, which can't express one on its own), and the
// three-line UUID-minting BeforeCreate body. Each is additive — no
// consumer repo is forced to adopt any of them.
package gormutil

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// SQLSTATE class 23 is "integrity constraint violation".
const (
	sqlstateNotNullViolation    = "23502"
	sqlstateForeignKeyViolation = "23503"
	sqlstateUniqueViolation     = "23505"
	sqlstateCheckViolation      = "23514"
)

// ClassifyError maps a GORM/driver error from either Postgres or sqlite
// onto one of two categories the caller names: a violated foreign-key,
// not-null, or check constraint calls wrapReference(field); a violated
// unique constraint calls wrapConflict(field) — field is the
// constraint/column/table name the driver reported, best-effort. err is
// returned unchanged when it matches neither class, including
// gorm.ErrRecordNotFound, always passed through untouched. Ported from
// mwanachama-backend-comm's classify, generalized so each repo keeps its
// own sentinel errors (comm's ErrInvalidReference/ErrConflict are
// domain-specific, not shared) while sharing the SQLSTATE-vs-sqlite-text
// lookup mechanics — the same "share the mechanism, not the mapping"
// shape httpwire.StatusFor already uses for HTTP status codes.
func ClassifyError(err error, wrapReference, wrapConflict func(field string) error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case sqlstateForeignKeyViolation, sqlstateNotNullViolation, sqlstateCheckViolation:
			return wrapReference(pgConstraintOf(pgErr))
		case sqlstateUniqueViolation:
			return wrapConflict(pgConstraintOf(pgErr))
		default:
			return err
		}
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "UNIQUE constraint failed"):
		return wrapConflict(sqliteConstraintOf(msg))
	case strings.Contains(msg, "FOREIGN KEY constraint failed"),
		strings.Contains(msg, "NOT NULL constraint failed"),
		strings.Contains(msg, "CHECK constraint failed"):
		return wrapReference(sqliteConstraintOf(msg))
	default:
		return err
	}
}

func pgConstraintOf(pgErr *pgconn.PgError) string {
	if pgErr.ConstraintName != "" {
		return pgErr.ConstraintName
	}
	if pgErr.ColumnName != "" {
		return pgErr.ColumnName
	}
	return pgErr.TableName
}

// sqliteConstraintOf pulls the "table.column[, table.column...]" tail
// modernc.org/sqlite appends after "constraint failed: <KIND> constraint
// failed: " — the closest sqlite equivalent of pgconn.PgError's constraint
// name, good enough for a wrapReference/wrapConflict field label.
func sqliteConstraintOf(msg string) string {
	if i := strings.LastIndex(msg, "failed: "); i >= 0 {
		return msg[i+len("failed: "):]
	}
	return msg
}

// SyncPartialUniqueIndex creates a partial unique index named
// "<table>_<suffix>_uniq" (CREATE UNIQUE INDEX IF NOT EXISTS) if it
// doesn't already exist, skipping any dialect other than postgres/sqlite
// — the two this family's tests and deployments actually use — rather
// than guessing at unsupported SQL, mirroring every existing caller's own
// dialect gate. columns is the indexed column-list expression (e.g.
// "(hierarchy_id)", or a per-dialect JSON-path expression the caller
// builds itself — that expression-building is genuinely per-caller and
// stays there, not generalized here); where is the partial index's WHERE
// clause. This is the naming/templating/error-wrapping scaffolding every
// existing caller (actor's syncDefaultAnchorIndex/
// syncUniqueAttributeIndexes, assetmanager's syncSerialTagUniqueIndex)
// already independently retyped identically.
func SyncPartialUniqueIndex(db *gorm.DB, table, suffix, columns, where string) error {
	switch db.Dialector.Name() {
	case "postgres", "sqlite":
	default:
		return nil
	}
	idx := fmt.Sprintf("%s_%s_uniq", table, suffix)
	sql := fmt.Sprintf("CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s %s WHERE %s", idx, table, columns, where)
	if err := db.Exec(sql).Error; err != nil {
		return fmt.Errorf("SyncPartialUniqueIndex: %s: %w", idx, err)
	}
	return nil
}

// SetUUIDIfEmpty sets *id to a fresh random UUID if it is currently
// empty — the three-line GORM BeforeCreate body
// (actor/forms/git's ActorRow/ApprovalRow/BranchRow etc. all read
// identically: "if r.ID == "" { r.ID = uuid.NewString() }") copied
// across every repo that mints its own primary key this way. Comm mints
// human-readable, prefix-numbered ids instead (its own gormstore.mintID)
// and has no use for this.
func SetUUIDIfEmpty(id *string) {
	if *id == "" {
		*id = uuid.NewString()
	}
}
