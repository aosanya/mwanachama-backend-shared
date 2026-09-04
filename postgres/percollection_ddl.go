package postgres

import (
	"fmt"
	"strings"

	"github.com/aosanya/mwanachama-backend-shared/schema"
)

// idsTableDDL is the shared id-registry table: id -> type_id, one row per
// entity across every per-type table. It's what lets a split-by-type
// design still have a real Postgres foreign key (relationshipsDDL below
// targets it) and a single indexed lookup for GetEntity/UpdateEntity/
// DeleteEntity, which take no type argument — see percollection.go's
// resolveTable.
const idsTableDDL = `
CREATE TABLE IF NOT EXISTS %[1]s (
    id          TEXT PRIMARY KEY,
    type_id     TEXT NOT NULL
);
`

// entityTableDDL is one physical entity table's shape — identical to the
// entities table [DDL] emits. type_id stays even though a
// PerCollectionBackend table holds only one type, so scanEntity/entityCols
// (shared with Backend) keep working unchanged. id has no FK to the ids
// table: CreateEntity/UpsertEntity write both rows in one transaction, but
// UpsertEntity doesn't know its final id (new vs. conflicting-existing)
// until after its own INSERT, so the ids-table row is written second —
// an FK the other way round would reject that order.
const entityTableDDL = `
CREATE TABLE IF NOT EXISTS %[1]s (
    id          TEXT PRIMARY KEY,
    type_id     TEXT NOT NULL,
    properties  JSONB NOT NULL DEFAULT '{}'::jsonb,
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

CREATE UNIQUE INDEX IF NOT EXISTS %[1]s_unique_key_idx
    ON %[1]s (type_id, unique_key)
    WHERE unique_key IS NOT NULL AND NOT deleted;
`

// relTableDDL is one relationship-pair table's shape — same columns
// relationshipCols/scanRelationship already expect (reused unchanged), but
// from_id/to_id reference the two SPECIFIC content tables this table
// connects, not a generic registry: a real, narrow foreign key per pair,
// the same guarantee [DDL] gives with one shared entities table, given
// here without one.
const relTableDDL = `
CREATE TABLE IF NOT EXISTS %[1]s (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    from_id     TEXT NOT NULL REFERENCES %[2]s (id),
    to_id       TEXT NOT NULL REFERENCES %[3]s (id),
    properties  JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS %[1]s_from_idx
    ON %[1]s (from_id, name);

CREATE INDEX IF NOT EXISTS %[1]s_to_idx
    ON %[1]s (to_id, name);
`

// schemaTablesDDL is the schema_drafts/schema_published shape, unchanged
// from [DDL] — schema documents aren't entity content and stay shared
// regardless of how entities/relationships are stored.
const schemaTablesDDL = `
-- A single mutable draft document — see postgres/ddl.go's DDL for the
-- singleton-table idiom this shares.
CREATE TABLE IF NOT EXISTS %[1]s (
    singleton   BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
    document    JSONB NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Immutable, append-only published snapshots.
CREATE TABLE IF NOT EXISTS %[2]s (
    version     INTEGER PRIMARY KEY,
    document    JSONB NOT NULL,
    active      BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS %[2]s_one_active_idx
    ON %[2]s ((true)) WHERE active;
`

// distinctCollections returns sch's TypeDefinition.StorageCollection
// values, each once, in declaration order.
func distinctCollections(sch schema.Schema) []string {
	seen := make(map[string]bool, len(sch.Types))
	var out []string
	for _, td := range sch.Types {
		if td.StorageCollection == "" || seen[td.StorageCollection] {
			continue
		}
		seen[td.StorageCollection] = true
		out = append(out, td.StorageCollection)
	}
	return out
}

// relEdge is one physical relationship-pair table: the two types it
// connects (fromType/toType, TypeDefinition.Name values) and the table
// name, taken from the declaring RelationshipDefinition.StorageTable — the
// schema author names it explicitly (convention: the two connected types
// with an X between them, e.g. "memberXgroup"), this package never derives
// it.
type relEdge struct {
	relName          string
	table            string
	fromType, toType string
}

// relationshipEdges walks every TypeDefinition.Relationships in sch and
// returns one relEdge per declared RelationshipDefinition that sets
// StorageTable, deduplicated by table name. A RelationshipDefinition with
// no StorageTable is skipped (not a physical edge — e.g. an inverse
// declared purely for documentation with no writer of its own; see
// mwanachama-backend-actor's "has_member").
func relationshipEdges(sch schema.Schema) []relEdge {
	seen := make(map[string]bool)
	var out []relEdge
	for _, td := range sch.Types {
		for _, rel := range td.Relationships {
			if rel.StorageTable == "" || seen[rel.StorageTable] {
				continue
			}
			seen[rel.StorageTable] = true
			out = append(out, relEdge{relName: rel.Name, table: rel.StorageTable, fromType: td.Name, toType: rel.ToType})
		}
	}
	return out
}

// PerCollectionDDL returns the CREATE TABLE / CREATE INDEX statements for
// [NewPerCollectionBackend]: the shared id-registry table (tables.Entities
// — see DefaultPerCollectionTableNames), one physical table per distinct
// TypeDefinition.StorageCollection value in sch (every type must set one —
// see NewPerCollectionBackend), one physical table per relationship type
// pair (relationshipEdges — real foreign keys straight to the two content
// tables it connects, not the id registry), and tables.SchemaDrafts/
// SchemaVersions.
func PerCollectionDDL(sch schema.Schema, tables TableNames) string {
	tableFor := make(map[string]string, len(sch.Types))
	for _, td := range sch.Types {
		tableFor[td.Name] = td.StorageCollection
	}

	var b strings.Builder
	fmt.Fprintf(&b, idsTableDDL, tables.Entities)
	for _, name := range distinctCollections(sch) {
		fmt.Fprintf(&b, entityTableDDL, name)
	}
	for _, e := range relationshipEdges(sch) {
		fmt.Fprintf(&b, relTableDDL, e.table, tableFor[e.fromType], tableFor[e.toType])
	}
	fmt.Fprintf(&b, schemaTablesDDL, tables.SchemaDrafts, tables.SchemaVersions)
	return b.String()
}

// PerCollectionDropDDL returns the DROP TABLE statements undoing
// [PerCollectionDDL], in dependency order. For a migration's down file.
func PerCollectionDropDDL(sch schema.Schema, tables TableNames) string {
	var b strings.Builder
	fmt.Fprintf(&b, "DROP TABLE IF EXISTS %s;\n", tables.SchemaVersions)
	fmt.Fprintf(&b, "DROP TABLE IF EXISTS %s;\n", tables.SchemaDrafts)
	for _, e := range relationshipEdges(sch) {
		fmt.Fprintf(&b, "DROP TABLE IF EXISTS %s;\n", e.table)
	}
	for _, name := range distinctCollections(sch) {
		fmt.Fprintf(&b, "DROP TABLE IF EXISTS %s;\n", name)
	}
	fmt.Fprintf(&b, "DROP TABLE IF EXISTS %s;\n", tables.Entities)
	return b.String()
}
