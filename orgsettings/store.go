package orgchrome

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// sqlstateCheckViolation is Postgres's SQLSTATE for a violated CHECK
// constraint — org_chrome_dialling_region_shape is the only one this table
// carries.
const sqlstateCheckViolation = "23514"

// ErrInvalidDiallingRegion is what Put returns when default_dialling_region
// fails its CHECK — the caller's mistake, not a server fault. Kept local to
// this package rather than routed through the gateway's cross-domain
// domerr.ErrInvalidReference sentinel, since domerr hasn't moved to this repo
// (a separate, not-yet-done pass); callers that want a domerr-shaped error
// can wrap this themselves.
var ErrInvalidDiallingRegion = errors.New(
	"orgchrome: default_dialling_region must be an ISO-3166-1 alpha-2 code or empty")

// schema creates the org_chrome table if it doesn't already exist — the same
// self-healing idiom every mwanachamaX.Migrate(db, ...) call in the
// gateway's stores.go already follows, ported from
// mwanachama-backend-api-gateway's own migrations_archive/000001_orgchrome.up.sql
// and migrations_archive/000023_org_dialling_region.up.sql (DEV-1258).
//
// default_dialling_region carries a real CHECK, not just a Go-level guard:
// ISO-3166-1 alpha-2 uppercase or empty. Per G366/DSN-1479 (2026-08-22), a
// wrong region silently mis-hashes every nationally-formatted phone number
// entered under it with nothing to re-hash from — the wrongness is
// unrecoverable, which is exactly the class of value a Go guard (one call
// site away from being bypassed) is the wrong tool for. Lowercase is refused
// rather than folded because the region parser downstream is case-sensitive
// on region codes.
const schema = `
CREATE TABLE IF NOT EXISTS org_chrome (
    slug                     TEXT PRIMARY KEY,
    display_name             TEXT NOT NULL DEFAULT '',
    primary_color            TEXT NOT NULL DEFAULT '',
    accent_color             TEXT NOT NULL DEFAULT '',
    logo_url                 TEXT NOT NULL DEFAULT '',
    support_email            TEXT NOT NULL DEFAULT '',
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    default_dialling_region  TEXT NOT NULL DEFAULT ''
        CONSTRAINT org_chrome_dialling_region_shape
        CHECK (default_dialling_region = '' OR default_dialling_region ~ '^[A-Z]{2}$')
)`

// Migrate creates the org_chrome table if it doesn't already exist. Callers
// run this once at startup (or in test setup) before constructing a Store
// with the same db.
func Migrate(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, schema)
	return err
}

// Store is the Postgres implementation of Repository.
type Store struct {
	db *sql.DB
}

// NewStore constructs a store over the given pool.
func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// Get returns the chrome record for slug, or ErrNotFound when there is no
// row.
func (s *Store) Get(ctx context.Context, slug string) (Chrome, error) {
	const q = `SELECT slug, display_name, primary_color, accent_color, logo_url, support_email,
	                  default_dialling_region
	           FROM org_chrome WHERE slug = $1`
	var c Chrome
	err := s.db.QueryRowContext(ctx, q, slug).Scan(
		&c.Slug, &c.DisplayName, &c.PrimaryColor, &c.AccentColor, &c.LogoURL, &c.SupportEmail,
		&c.DefaultDiallingRegion,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Chrome{}, ErrNotFound
	}
	if err != nil {
		return Chrome{}, err
	}
	return c, nil
}

// Put upserts a chrome record by slug and returns the persisted value.
func (s *Store) Put(ctx context.Context, c Chrome) (Chrome, error) {
	const q = `
		INSERT INTO org_chrome (slug, display_name, primary_color, accent_color, logo_url, support_email,
		                        default_dialling_region, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT (slug) DO UPDATE SET
		    display_name            = EXCLUDED.display_name,
		    primary_color           = EXCLUDED.primary_color,
		    accent_color            = EXCLUDED.accent_color,
		    logo_url                = EXCLUDED.logo_url,
		    support_email           = EXCLUDED.support_email,
		    default_dialling_region = EXCLUDED.default_dialling_region,
		    updated_at              = NOW()
		RETURNING slug, display_name, primary_color, accent_color, logo_url, support_email,
		          default_dialling_region`
	var out Chrome
	err := s.db.QueryRowContext(ctx, q,
		c.Slug, c.DisplayName, c.PrimaryColor, c.AccentColor, c.LogoURL, c.SupportEmail,
		c.DefaultDiallingRegion,
	).Scan(
		&out.Slug, &out.DisplayName, &out.PrimaryColor, &out.AccentColor, &out.LogoURL, &out.SupportEmail,
		&out.DefaultDiallingRegion,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == sqlstateCheckViolation {
			return Chrome{}, ErrInvalidDiallingRegion
		}
		return Chrome{}, err
	}
	return out, nil
}

// Compile-time interface check.
var _ Repository = (*Store)(nil)
