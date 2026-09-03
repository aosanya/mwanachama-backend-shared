package postgres

import "database/sql"

// Backend is the Postgres implementation of both entitygraph.DataManager and
// entitygraph.SchemaManager, reading and writing the tables named by
// TableNames (see DDL). One Backend serves one domain (e.g. mwanachama-backend-git's
// git_entities/git_relationships/…) — construct a second Backend with a
// different TableNames to serve another domain over the same *sql.DB.
type Backend struct {
	db     *sql.DB
	tables TableNames
}

// NewBackend constructs a Backend over db, reading and writing the tables
// named by t. Does not verify the tables exist — run a migration built from
// DDL(t) first.
func NewBackend(db *sql.DB, t TableNames) *Backend {
	return &Backend{db: db, tables: t}
}
