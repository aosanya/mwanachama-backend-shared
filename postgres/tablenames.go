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

	// SchemaDrafts is the one-row-per-agency mutable draft schema table.
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
