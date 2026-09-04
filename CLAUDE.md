# CLAUDE.md

Guidance for Claude Code working in this repository.

## Project: mwanachama-backend-shared

The shared Postgres entity-graph engine underneath
[mwanachama-backend-git](../mwanachama-backend-git) and
[mwanachama-backend-taskmanager](../mwanachama-backend-taskmanager). Module path
`github.com/aosanya/mwanachama-backend-shared`.

Ported from `CodeValdSharedLib/entitygraph` (an ArangoDB-backed generic
entity/relationship graph store built for CodeValdCortex agencies), but:

- **Retargeted to Postgres.** Entities live in one `entities` table
  (`type_id` + `jsonb properties` + soft-delete columns) instead of one
  Arango collection per type; edges live in one `relationships` table.
  The original ArangoDB backend's AQL graph traversal was ported here as a
  server-side `TraverseGraph` (a recursive CTE) but has since been removed
  from `DataManager` as part of retiring per-agency scoping — consumers
  (e.g. `mwanachama-backend-git`'s `traverseNeighborhood`) now walk the
  graph client-side via `ListRelationships`/`GetEntity` instead.
- **No `CodeValdSharedLib` dependency.** This is meant to be a standalone
  public library — `CodeValdSharedLib` is private and unpublished — so the
  `Entity`/`Relationship`/`DataManager`/`SchemaManager`/`Schema` contract is
  redefined here, not imported.
- **No gRPC / sub-service shape.** `mwanachama-backend-api-gateway` runs as one
  service and imports libraries directly; there is no `proto/`, `cmd/`, or
  registrar/heartbeat layer here, only a trivial local `Publisher` interface
  (`Publish(ctx, topic string, payload any) error`) for callers to plug into.

## Key invariants (carry these into the Postgres port)

- **The `DataManager`/`SchemaManager` interfaces must stay storage-agnostic**
  Go — this is the seam `mwanachama-backend-git` and `mwanachama-backend-taskmanager` build
  their ported business logic against, mirroring how `CodeValdGit`'s
  `git_impl_*.go` and `CodeValdWork`'s `task*.go` only ever call
  `entitygraph.DataManager`, never the ArangoDB driver directly.
- **One `entities` table, not one table per type.** The old Arango backend
  had to scan every collection to find an entity by ID because Arango has no
  global secondary index across collections — a single table with a `type_id`
  column and a normal index removes that whole class of complexity. Don't
  reintroduce per-type tables.
- **`SchemaManager.Activate` must be a real transaction** (`UPDATE ... FOR
  UPDATE` / single `pgx.Tx`) enforcing exactly one active schema version,
  full stop (this is a single-tenant store — one deployment, one schema) —
  the Arango original did this as two sequential AQL statements scoped per
  agency, which was never actually atomic. Fix it here rather than port the
  bug.
- **Vertex uniqueness (`TypeDefinition.UniqueKey`) is schema-enforced**, not
  just app-checked — map it to a real Postgres partial/composite unique index
  per type, not a pre-check-then-insert race.

## Conventions

- Task status lives on
  [documentation/3. implementation/todo.md](documentation/3.%20implementation/todo.md).
- Four-phase `documentation/` layout — see
  [documentation/README.md](documentation/README.md).
