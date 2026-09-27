# Requirements

`mwanachama-backend-shared` has no product of its own — it exists because
three unrelated things kept getting reinvented per repo, and centralizing
each one removed a whole class of duplication rather than adding a feature:

## Role 1 — the Postgres entity-graph engine (S1–S9, 2026-09-02)

`CodeValdGit` and `CodeValdWork` are ArangoDB-backed Go libraries built for
CodeValdCortex agencies, leaning on a shared `entitygraph` package for
almost all of their storage. `mwanachama-frontend-kazi` needed the same
git-like versioned content and task/workflow management, but
`mwanachama-backend-api-gateway` standardizes on Postgres and runs as one
service, not sub-services. Porting the shared engine once, here, let
[mwanachama-backend-git](../../../mwanachama-backend-git) and
[mwanachama-backend-taskmanager](../../../mwanachama-backend-taskmanager)
port their own business logic (`git_impl_*.go`, `task*.go`) with minimal
churn instead of each reinventing entity/edge storage and graph traversal.
The seam is `entitygraph.DataManager`/`SchemaManager` (`entitygraph/`),
backed by either `postgres/` (real storage) or `memory/` (an in-process
double both `postgres` and consumers can test against without a database).
See `CLAUDE.md`'s "Key invariants" for what a Postgres port of this engine
must keep true (one `entities` table, not one per type; `Activate` as a
real transaction; schema-enforced vertex uniqueness).

## Role 2 — GORM-era shared infrastructure (S10–S16, 2026-09-06 onward)

Five sibling repos (`actor`, `assetmanager`, `git`, `comm`, `forms`)
independently migrated off `entitygraph` onto GORM and each rebuilt the
same JSON wire helpers, nullable-string converters, sqlite/Postgres test
harnesses and `TableNames`/`Migrate` wrapper. S10's audit
([2. design/gorm-shared-infrastructure.md](../2.%20design/gorm-shared-infrastructure.md))
named what to extract; `httpwire/`, `gormtest/`, `gormutil/` and `vocab/`
are the result. Adoption by each consumer repo is opportunistic, not
mandatory — `mwanachama-backend-assetmanager`'s S16 pass is the one
first-adopter conversion done so far.

`mcpui/` (2026-09-11) is a smaller instance of the same pattern for a
newer surface: every backend repo's hand-rolled MCP Apps (SEP-1865)
dashboard View needs the same CSS, so it lives here once rather than
drifting per repo. See `CLAUDE.md`'s "mcpui" section.

## Role 3 — the declared-domain engine (S18–S22, 2026-09-24 onward)

Moved here from `mwanachama-backend-catalog` (S18) when
`mwanachama-backend-agency`'s AGD-007 needed the same "objects are declared,
not written" shape for a second module: a module ships a blueprint naming
its roles and fields, a domain ships a spec naming which object fills each
role, and `spec.Migrate` emits the DDL — no row structs, no `AutoMigrate`.
`specstore/` (S19) is the matching generic store, and `dispatch/` (from
`mwanachama-backend-catalog`'s own W14, extended here by S21/S22) is the
same idea for the route table: a declared operation becomes an
`httpwire.Route` or an MCP tool, never a hand-written handler.
`mwanachama-backend-catalog` is the first consumer, `mwanachama-backend-agency`
the second. See [2. design/declared-domains.md](../2.%20design/declared-domains.md)
and [2. design/dispatcher.md](../2.%20design/dispatcher.md) for the format
itself; this engine has no domain vocabulary of its own to specify further
requirements against.

## What this repo deliberately has no requirements for

No auth model, no HTTP server, no CLI, no per-tenant provisioning — each of
those is a decision for whatever mounts a blueprint or a spec
(`mwanachama-wakala-api`, the gateway), never for the engine itself.
