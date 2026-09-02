package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/aosanya/mwanachama-go-shared/entitygraph"
)

// Postgres SQLSTATE codes this package maps to entitygraph sentinel errors.
// See https://www.postgresql.org/docs/current/errcodes-appendix.html.
const (
	sqlstateUniqueViolation     = "23505"
	sqlstateForeignKeyViolation = "23503"
)

// classify maps a raw driver error to the entitygraph sentinel error a
// caller expects. "Not found" is call-site specific (entity vs relationship
// vs schema), so callers check sql.ErrNoRows themselves before falling back
// to classify for everything else.
func classify(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case sqlstateUniqueViolation:
			return entitygraph.ErrEntityAlreadyExists
		case sqlstateForeignKeyViolation:
			return entitygraph.ErrEntityNotFound
		}
	}
	return err
}
