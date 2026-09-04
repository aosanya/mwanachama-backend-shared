package postgres

// TableNames configures which physical tables a Backend reads and writes.
// Each consumer of this package owns its own tables — mirroring how
// CodeValdGit and CodeValdWork each fixed their own ArangoDB collection
// names (git_entities/git_relationships vs work_entities/
// work_relationships) over the same shared entitygraph engine — rather than
// every consumer sharing one physical entities/relationships table.
type TableNames struct {
	// Entities is the table holding every entity for this domain,
	// regardless of TypeID.
	Entities string

	// Relationships is the table holding every directed edge for this
	// domain.
	Relationships string

	// SchemaDrafts is the singleton mutable draft schema table (at most one
	// row — see DDL's schema_drafts CHECK(singleton) idiom).
	SchemaDrafts string

	// SchemaVersions is the append-only published schema snapshot table.
	SchemaVersions string
}

// DefaultTableNames builds the conventional table set for a domain prefix,
// e.g. DefaultTableNames("git_") yields git_entities, git_relationships,
// git_schemas_draft, git_schemas_published — matching CodeValdGit's/
// CodeValdWork's own collection-naming convention (their storage/arangodb
// shims fixed exactly these four names per domain over the shared
// entitygraph backend).
func DefaultTableNames(prefix string) TableNames {
	return TableNames{
		Entities:       prefix + "entities",
		Relationships:  prefix + "relationships",
		SchemaDrafts:   prefix + "schemas_draft",
		SchemaVersions: prefix + "schemas_published",
	}
}

// DefaultPerCollectionTableNames builds the table set for
// [NewPerCollectionBackend], e.g. DefaultPerCollectionTableNames("member_")
// yields member_ids, member_relationships, member_schemas_draft,
// member_schemas_published. Entities here does NOT name a content table —
// PerCollectionBackend's entity content lives in one table per
// TypeDefinition.StorageCollection instead (e.g. member_members,
// member_groups) — it names the small shared id-registry table
// (id -> type_id) that makes that split work: a real foreign key target
// for Relationships.from_id/to_id, and a single indexed lookup instead of
// probing every per-type table to resolve an id with no type given.
func DefaultPerCollectionTableNames(prefix string) TableNames {
	return TableNames{
		Entities:       prefix + "ids",
		Relationships:  prefix + "relationships",
		SchemaDrafts:   prefix + "schemas_draft",
		SchemaVersions: prefix + "schemas_published",
	}
}
