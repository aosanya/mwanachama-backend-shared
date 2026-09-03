// Package memory is an in-process implementation of
// github.com/aosanya/mwanachama-backend-shared/entitygraph's DataManager and
// SchemaManager, sibling to postgres/. Same role as
// mwanachama-backend-api-gateway's internal/store/memory: lets tests and local dev
// run against the real interface without a database, while postgres/ is
// what actually ships.
package memory

import (
	"sync"

	"github.com/aosanya/mwanachama-backend-shared/entitygraph"
	"github.com/aosanya/mwanachama-backend-shared/schema"
)

// Backend holds every entity, relationship, and schema document in plain
// Go maps behind one mutex. Not tuned for concurrency — this is a test and
// local-dev double, not a production store.
type Backend struct {
	mu sync.Mutex

	entities       map[string]entitygraph.Entity       // by ID
	relationships  map[string]entitygraph.Relationship // by ID
	schemaDrafts   map[string]schema.Schema            // by AgencyID
	schemaVersions map[string][]schema.Schema          // by AgencyID, ascending version
}

// NewBackend constructs an empty Backend.
func NewBackend() *Backend {
	return &Backend{
		entities:       map[string]entitygraph.Entity{},
		relationships:  map[string]entitygraph.Relationship{},
		schemaDrafts:   map[string]schema.Schema{},
		schemaVersions: map[string][]schema.Schema{},
	}
}
