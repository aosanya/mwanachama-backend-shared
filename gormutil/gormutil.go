package gormutil

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"strings"
)

const (
	sqlstateNotNullViolation    = "23502"
	sqlstateForeignKeyViolation = "23503"
	sqlstateUniqueViolation     = "23505"
	sqlstateCheckViolation      = "23514"
)

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
	case strings.Contains(msg, "FOREIGN KEY constraint failed"), strings.Contains(msg, "NOT NULL constraint failed"), strings.Contains(msg, "CHECK constraint failed"):
		return wrapReference(sqliteConstraintOf(msg))
	default:
		return err
	}
}
func IsUniqueViolation(err error) bool {
	conflicted := false
	ClassifyError(err, func(string) error {
		return err
	}, func(string) error {
		conflicted = true
		return err
	})
	return conflicted
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
func sqliteConstraintOf(msg string) string {
	if i := strings.LastIndex(msg, "failed: "); i >= 0 {
		return msg[i+len("failed: "):]
	}
	return msg
}
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
func SetUUIDIfEmpty(id *string) {
	if *id == "" {
		*id = uuid.NewString()
	}
}
