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

## mcpui

`mcpui/` (added 2026-09-11) has nothing to do with the entity-graph engine
above — it's this module's second role, as the dependency every backend
repo's own MCP Apps (SEP-1865) dashboard View pulls its CSS from, so those
hand-rolled HTML views share one visual design system
(`mwanachama-wakala-studio`'s look: IBM Plex Sans/Mono + Big Shoulders
Display, warm ochre accent, sharp corners, hairline borders) instead of
each repo hand-copying and drifting from its own snapshot of the palette.
`mwanachama-backend-agency/mcp/dashboard_ui.go` is the first, reference
consumer — see that repo's CLAUDE.md (AG17) for the full record. Depend on
this module the same way `mwanachama-backend-actor` already does: a
`replace github.com/aosanya/mwanachama-backend-shared =>
../mwanachama-backend-shared` line in the consumer's `go.mod`, since this
is unpublished and always resolved from the sibling checkout.

## spec

`spec/` is this module's third role, and has nothing to do with the
entity-graph engine above either: it is the loader for a **declared
domain** — the objects a module stores, their fields and their indexes, as
JSON rather than as Go row structs. A module ships a blueprint declaring
its roles and their fields; a domain ships a spec naming which object
fills each role, where it lands, and its own indexes. `spec.Migrate` emits
the DDL, which is the whole storage story — there is no `AutoMigrate` and
there are no row structs.

It knows nothing about any one module: a table is
`<instance>_<module>_<table>`, every identifier is validated against a strict
alphabet because it reaches SQL as text, and every emitted name is measured
against Postgres's 63-byte limit, which truncates silently. The `matches`
pattern registry stays per module — the spec names a pattern, the module says
what the name means.

`mwanachama-backend-agency` is a real consumer (AGD-007 there: no local
`spec/`or `gormstore/`, every `spec.Migrate`/`spec.Spec` reference in that
repo resolves to this package). **`mwanachama-backend-catalog` is not** —
despite row S18 (2026-09-24, `todo_done.md`) and an earlier version of this
section claiming catalog's own `spec/` was deleted and replaced with an
import of this package, catalog's checkout still carries its own complete,
untouched `spec/` (`blueprint.go`, `migrate.go`, `spec.go`, `validate.go`,
its own `examples/`) with its own `Migrate`, and imports this repo only for
`routes/dispatch`/`routes/httpwire`, never for `spec` — confirmed directly
against catalog's code and git history by the 2026-09-25 integration-test
sweep, which also filed the gap as catalog's own CAT10 after it broke
`mwanachama-wakala-api`'s build (a test there had been written on the faith
of this section's now-corrected claim). Whatever landed in this repo for
S18 did not reach catalog's `master` — treat that row's "and catalog" half
as not done until CAT10 closes it. The org-wide strategy this serves is
`developer/documentation/2. design/architecture-spec-driven-modules.md`;
the engine's own reference is
[documentation/2. design/](documentation/2.%20design/), beside `dispatcher.md`
and `httpwire.md`, which document the operations half of the same shape.

## Code comments

Write code with no comments. Not one-liners above a function, not section
banners, not doc comments on exported symbols, not "why" notes next to a
tricky line. A name, a type, or a smaller function carries it instead.

Anything that genuinely needs explaining goes in this repo's `documentation/`
folder, under the phase it belongs to (`1. requirements`, `2. design`,
`3. implementation`, `4. qa`) — never inline.

**Why:** inline prose drifts out of sync with the code, duplicates what
`documentation/` already owns, and buries the explanation where nobody
looking for it will search.

**How to apply:**

- New code ships without comments. If a line seems to need one, rename or
  split until it doesn't.
- Touching code that already has comments: strip the ones in the code you are
  changing. Do not sweep untouched files unless asked.
- If the reasoning matters, add or update the matching `documentation/` page
  in the same change and leave nothing behind in the source.
- Machine-read directives are not comments and stay: build tags, `//go:embed`,
  `//go:generate`, linter pragmas (`//nolint`, `// eslint-disable-next-line`,
  `// ignore:`), license headers, codegen "do not edit" banners, and generated
  files as a whole.
- Commit messages, PR descriptions, and test names carry the narration that
  used to go in comments.

This rule is repeated verbatim in every mwanachama repo's `CLAUDE.md` so that
it reaches sessions that do not load this machine's user-level config —
scheduled cloud routines, other machines, and other agent harnesses.
