// Package postgres is the Postgres implementation of
// github.com/aosanya/mwanachama-backend-shared/entitygraph's DataManager and
// SchemaManager interfaces.
//
// Stays behind database/sql (via the pgx stdlib driver) rather than exposing
// a pgxpool.Pool, matching mwanachama-backend-api-gateway's own store convention —
// callers depend on the standard interface, not on pgx types directly.
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Config carries the connection parameters. A DSN wins over the discrete
// fields when set; the discrete fields are a DSN-less convenience path.
type Config struct {
	DSN string // e.g. postgres://user:pass@host:5432/dbname?sslmode=disable

	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string // "disable", "require", ...

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxIdleTime time.Duration
}

// ErrMissingConfig is returned when neither a DSN nor the discrete fields
// give us enough to reach a database.
var ErrMissingConfig = errors.New("postgres: no DSN and no host/database provided")

// Open dials the database, pings it to fail fast, and returns the pool.
// Callers own closing the returned *sql.DB.
func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	dsn := cfg.DSN
	if dsn == "" {
		if cfg.Host == "" || cfg.Database == "" {
			return nil, ErrMissingConfig
		}
		dsn = buildDSN(cfg)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// buildDSN assembles a libpq-style DSN from discrete fields; pgx's driver
// accepts this form.
func buildDSN(cfg Config) string {
	sslmode := cfg.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	port := cfg.Port
	if port == 0 {
		port = 5432
	}
	return "host=" + cfg.Host +
		" port=" + itoa(port) +
		" user=" + cfg.User +
		" password=" + cfg.Password +
		" dbname=" + cfg.Database +
		" sslmode=" + sslmode
}

// itoa avoids strconv just to keep the DSN builder dependency-free.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	n := len(buf)
	for i > 0 {
		n--
		buf[n] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		n--
		buf[n] = '-'
	}
	return string(buf[n:])
}
