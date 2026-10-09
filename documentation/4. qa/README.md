# QA

Test coverage and results for `mwanachama-backend-shared`.

## How to run it

`go test ./...` runs every package offline, on SQLite (`glebarez/sqlite`) or
in memory. `orgpolicy/postgres_integration_test.go` and
`orgsettings/postgres_integration_test.go` skip unless `POSTGRES_URL` is set,
so a real Postgres is never reached by default.

## What the suite covers

Test functions per package (`go test ./<pkg>/ -list '.*'`), counted
2026-10-03 and re-counted for `dispatch` and `spec` on 2026-10-09 — S54's
`querykey_fallback_test.go` and S55's `default_literal_security_test.go`:

| Package | Tests | What it holds |
|---------|-------|---------------|
| `dispatch` | 77 | the operations engine — binding, routes, MCP tools and the authorizer gate |
| `spec` | 56 | loading, validation and blueprint merge of a declared domain; the migrator |
| `entitygraph` | 23 | the entity-graph contract |
| `specstore` | 31 | the codec and query helpers every declared module stores through |
| `gormutil` | 13 | shared GORM helpers |
| `httpwire` | 8 | the `Route` type and JSON error writing |
| `orgpolicy`, `orgpolicy/models` | 8, 6 | org policy store and types |
| `orgsettings`, `orgsettings/models` | 5, 6 | org settings store and types |
| `postgres` | 3 | the Postgres entity-graph backend |
| `gormtest`, `vocab`, `memory` | 2, 2, 1 | test helpers, vocabulary, in-memory backend |

## The authorizer gate

`dispatch/authorize_test.go` pins the gate every declared module mounts
behind: a refused route answers `403` and never reaches the manager, a refused
tool is refused the same way, the refusal does not name the action, and the
authorizer is asked before any argument is read. The same gate is exercised
end to end through a real module's route table by
`mwanachama-backend-catalog/routes/deny_all_security_test.go` (security sweep,
2026-09-29), which was mutation-checked against `handlerFor` and `invokerFor`
here: skipping either gate for one action turns it red.

## Red by design

- `specstore/store_listcap_test.go`'s `TestS24_OpenBug_ListHasNoDefaultCap`
  (S24) — the integration sweep's, left failing on purpose until `List`
  gains a default cap.
- `spec/migrate_evolve_test.go`'s
  `TestS52_OpenBug_SecondMigrateDoesNotAddANewColumn` (S52) — the integration
  sweep's, left failing on purpose until a second `Migrate` against an
  already-provisioned table alters it to add a field the blueprint gained.
- `spec/migrate_drop_field_test.go`'s
  `TestS53_OpenBug_DroppedRequiredFieldOrphansANotNullColumn` (S53) — the
  integration sweep's, left failing on purpose until dropping a `required`
  field's column (or relaxing its `not null`) is part of the same fix S52
  needs.
- `dispatch/querykey_fallback_test.go`'s
  `TestS54_OpenBug_UntaggedQueryFieldBindsOnlyAllLowercaseNotSnakeCase` (S54)
  — the integration sweep's, left failing on purpose until an untagged query
  field binds on its snake_case name.
- `spec/default_literal_security_test.go`'s
  `TestS55_OpenHole_NumericDefaultReachesTheDDLUnquoted` (S55) — the security
  sweep's, left failing on purpose until `spec.Parse` refuses an `int`,
  `float` or `bool` default that is not a literal of its type; today one
  carrying a stacked statement is executed by `spec.Migrate`.

`spec/mount_collision_security_test.go`'s
`TestS28_OpenHole_MountNameSpellsTheDefaultMountsTable` (S28) is no longer
red: S29's hashed physical name (f96ad8b) gives the mount its own segment, so
a mount named `patient` can no longer spell the default mount's
`patient_page_views`. It passes on 51d191d (security sweep, 2026-10-01) and
still carries its `_OpenHole_` name until it is renamed as a regression guard.

The mount segment (S26) is pinned green by
`spec/mount_segment_security_test.go` (security sweep, 2026-09-29): an
underscore, capital, hyphen, space, `;` or `.` in `mount` is refused by name,
and a mount that pushes a name past 63 bytes is refused (mutation-checked by
dropping the `SegmentPattern` check on the mount).
