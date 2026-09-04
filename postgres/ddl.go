package postgres

import "fmt"

// DDL returns the CREATE TABLE / CREATE INDEX statements for one domain's
// entity-graph tables, named per t. Each consumer (mwanachama-backend-git,
// mwanachama-backend-taskmanager, …) embeds this into its own golang-migrate
// up-migration — e.g.
//
//	postgres.DDL(postgres.DefaultTableNames("git_"))
//
// — so every consumer owns and versions its own physical tables, even
// though the SQL shape (and the DataManager/SchemaManager code that reads
// and writes it) is defined once, here.
func DDL(t TableNames) string {
	return fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s (
    id          TEXT PRIMARY KEY,
    type_id     TEXT NOT NULL,
    properties  JSONB NOT NULL DEFAULT '{}'::jsonb,
    -- unique_key is set only by UpsertEntity, from TypeDefinition.UniqueKey —
    -- a JSON array of the entity's key property values, e.g. ["a1b2c3"].
    -- NULL for entities created via plain CreateEntity.
    unique_key  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted     BOOLEAN NOT NULL DEFAULT false,
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS %[1]s_type_idx
    ON %[1]s (type_id) WHERE NOT deleted;

CREATE INDEX IF NOT EXISTS %[1]s_properties_gin_idx
    ON %[1]s USING GIN (properties);

-- Backs UpsertEntity's ON CONFLICT target: at most one non-deleted entity
-- per (type_id, unique_key).
CREATE UNIQUE INDEX IF NOT EXISTS %[1]s_unique_key_idx
    ON %[1]s (type_id, unique_key)
    WHERE unique_key IS NOT NULL AND NOT deleted;

CREATE TABLE IF NOT EXISTS %[2]s (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    from_id     TEXT NOT NULL REFERENCES %[1]s (id),
    to_id       TEXT NOT NULL REFERENCES %[1]s (id),
    properties  JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS %[2]s_from_idx
    ON %[2]s (from_id, name);

CREATE INDEX IF NOT EXISTS %[2]s_to_idx
    ON %[2]s (to_id, name);

-- A single mutable draft document — one deployment, one schema. singleton
-- is always true; the CHECK plus the PK is the standard Postgres idiom for
-- guaranteeing at most one row in a table.
CREATE TABLE IF NOT EXISTS %[3]s (
    singleton   BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
    document    JSONB NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Immutable, append-only published snapshots.
CREATE TABLE IF NOT EXISTS %[4]s (
    version     INTEGER PRIMARY KEY,
    document    JSONB NOT NULL,
    active      BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Enforces "exactly one active version, full stop" at the DB level —
-- Activate flips the old and new rows inside one transaction; this index is
-- the backstop if it ever doesn't. A unique index on a constant expression
-- is the standard Postgres idiom for "at most one row with this flag set."
CREATE UNIQUE INDEX IF NOT EXISTS %[4]s_one_active_idx
    ON %[4]s ((true)) WHERE active;
`, t.Entities, t.Relationships, t.SchemaDrafts, t.SchemaVersions)
}

// DropDDL returns the DROP TABLE statements undoing [DDL], in dependency
// order. For a migration's down file.
func DropDDL(t TableNames) string {
	return fmt.Sprintf(`
DROP TABLE IF EXISTS %[4]s;
DROP TABLE IF EXISTS %[3]s;
DROP TABLE IF EXISTS %[2]s;
DROP TABLE IF EXISTS %[1]s;
`, t.Entities, t.Relationships, t.SchemaDrafts, t.SchemaVersions)
}
