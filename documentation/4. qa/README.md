# QA

Test coverage and results for `mwanachama-backend-shared`.

## How to run it

`go test ./...` runs every package offline, on SQLite (`glebarez/sqlite`) or
in memory. `orgpolicy/postgres_integration_test.go` and
`orgsettings/postgres_integration_test.go` skip unless `POSTGRES_URL` is set,
so a real Postgres is never reached by default.

## What the suite covers

Test functions per package, counted 2026-09-29 at `a95fbd7`:

| Package | Tests | What it holds |
|---------|-------|---------------|
| `dispatch` | 61 | the operations engine — binding, routes, MCP tools and the authorizer gate |
| `spec` | 26 | loading, validation and blueprint merge of a declared domain; the migrator |
| `entitygraph` | 23 | the entity-graph contract |
| `specstore` | 14 | the codec and query helpers every declared module stores through |
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

Not by design, and not a security hole: since 7ebe719 (S26's mount segment)
`go test ./...` is also red for `spec`'s `TestBlueprintFillsTheFieldsADomainDoesNotRestate`
and `TestTwoDomainsCoexist` and `specstore`'s `TestTableComesFromTheSpec`, each
still expecting a three-segment name (`clinic_record_patients`) where
`TableFor` now emits `clinic_record_main_patients` — measured by the security
sweep, 2026-09-29T20:30Z. They are S27's to update. The mount segment's
validation is pinned green by `spec/mount_segment_security_test.go`: an
underscore, capital, hyphen, space, `;` or `.` in `mount` is refused by name,
and a mount that pushes a name past 63 bytes is refused (mutation-checked by
dropping the `SegmentPattern` check on the mount).
