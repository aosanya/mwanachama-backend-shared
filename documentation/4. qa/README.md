# QA

Test coverage for `mwanachama-backend-shared`. Tests live beside the code
they cover, split across the repo's three unrelated roles (see
[1. requirements/README.md](../1.%20requirements/README.md)); there is no
separate test corpus.

```sh
go test ./...                  # in-memory: memory/, sqlite via glebarez/sqlite for spec/specstore
go test -tags=integration ./... # same suites plus every //go:build integration file
make test-pg                    # -tags=integration, additionally exercising postgres/ against a real database when POSTGRES_URL is set
```

As of 2026-09-27 (commit `9272c53`), both `go test ./...` and
`go test -tags=integration ./...` are green across all 14 packages that
carry test files:

| Package | Test functions | Role |
|---|---:|---|
| `dispatch` | 62 | the declared-operations engine (routes + MCP tools) |
| `spec` | 26 | the declared-domain loader/validator/migrator |
| `entitygraph` | 23 | the Postgres entity-graph contract |
| `orgpolicy` | 14 | entity-graph role/RACI policy |
| `gormutil` | 13 | shared GORM helpers |
| `orgsettings` | 11 | entity-graph settings |
| `specstore` | 8 | the declared-domain store (column-by-field-name, write-every-column) |
| `httpwire` | 8 | the `Route`/error-to-status contract `dispatch` builds on |
| `orgpolicy/models` | 6 | — |
| `orgsettings/models` | 6 | — |
| `postgres` | 3 | live-Postgres conformance for the entity-graph engine, gated on `POSTGRES_URL` |
| `gormtest` | 2 | shared GORM test harness |
| `vocab` | 2 | — |
| `memory` | 1 | the entity-graph in-memory test double |

`schema`, `events`, `mcpui`, `orgpolicy/gormstore` and `orgsettings/gormstore`
build but carry no test files of their own — `schema` and `events` are pure
type declarations, `mcpui` is a `//go:embed`'d stylesheet, and the two
`gormstore` packages are exercised through their owning package's tests.

## What is guarded, and how

- **`entitygraphtest`** is one `Run(t, dm, sm, agencyID)` conformance suite
  covering schema lifecycle, entity CRUD, upsert-by-unique-key, relationships
  and traversal, run by both `memory` and `postgres` against their own
  `DataManager`/`SchemaManager` implementation — so the two backends are held
  to one behavioural contract rather than two hand-written suites that could
  drift apart.
- **`spec`'s refusals** — a bad name, an underscore in an instance/module
  segment, a duplicate role, two identifiers the database would merge past
  63 bytes, a required field carrying a default, an index naming both fields
  and a path — are each their own test in `spec_test.go`/`blueprint_test.go`.
- **`spec/testdata/clinic.record.json` and `garage.record.json`** are domains
  the engine's own authors did not write for any consumer, used specifically
  so a spec fixture in this repo's own suite is not just the shapes
  `mwanachama-backend-catalog`/`-agency`/`-permissions` already ship.
- **`dispatch`'s validation** — an operation naming no action, two operations
  claiming one action or one tool name, an unbound path wildcard, an
  overwrite from a source other than the path or an authenticated caller,
  a signature mismatch between an operation's declared args/returns and the
  manager method it calls (`checkSignature`) — each has its own test in
  `dispatch_test.go`/`overwrite_test.go`/`gate_test.go`/`tools_test.go`.
- **`postgres`, `orgpolicy`, `orgsettings` and `gormtest`** each gate their
  live-database cases on `POSTGRES_URL` being set (skipping otherwise,
  mirroring `mwanachama-backend-api-gateway`'s `make test` vs `make pg &&
  make test-pg` split); those cases are the one thing in this repo this
  sweep does not execute, matching every other repo in the declared-domain
  stack, which run in-memory rather than against a live Postgres container
  in their own scheduled test runs.

## Red by design

- `dispatch/overwrite_conflict_test.go`'s
  `TestS24_OpenBug_DuplicateOverwriteTargetIsRefusedAtLoad` (S24, filed
  2026-09-27) — `dispatch.Parse` currently accepts an operation where a
  `caller`-sourced overwrite and a `path`-sourced overwrite both target the
  same field, with the one declared last silently winning at runtime. See
  [3. implementation/todo.md](../3.%20implementation/todo.md)'s S24 row for
  the full evidence and fix location.
