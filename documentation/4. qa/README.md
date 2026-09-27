# QA

Test coverage and results for `mwanachama-backend-shared`. The tests live
beside the code they cover; there is no separate test corpus.

```sh
go test ./...                 # in-memory sqlite/memory doubles, no database needed
POSTGRES_URL=... go test ./... # the same suite, plus the postgres-only cases that check for it
```

There is no `integration` build tag in this repo (unlike `catalog`/`agency`,
which gate a `postgres_integration_test.go` behind one) — the Postgres-only
cases here (`postgres/backend_test.go`, `postgres/percollection_test.go`,
`orgpolicy/postgres_integration_test.go`,
`orgsettings/postgres_integration_test.go`) each check `POSTGRES_URL`
themselves and skip when it is unset, so `go test ./...` and
`go test -tags=integration ./...` currently run identically. As of
2026-09-27, commit `e8afba8`, `go test ./... -v -count=1`: 159 `--- PASS`
lines, zero `FAIL`, across every package — `dispatch` (55, the largest
suite: route resolution, the deny-by-default gate, MCP tool derivation, the
S21/S22 ignored-segment and declared-tool-name refusals), `spec` (26: name
collisions, the 63-byte truncation budget, `NamePattern`/`DocPathPattern`,
`TestTwoDomainsCoexist`), `entitygraph` (23), `gormutil` (13), `orgpolicy` +
`orgpolicy/models` (12), `httpwire` (8), `specstore` (8), `orgsettings` +
`orgsettings/models` (10), `vocab` (2), `gormtest`/`memory` (1 each),
`postgres` (0 offline — every case there is Postgres-only and skips without
`POSTGRES_URL`, see below). No test files: `entitygraphtest` (a conformance
suite consumed by `memory`/`postgres`, not itself under test),
`events`, `mcpui`, `schema` (pure type-system structs).

## What is guarded, and how

- **`spec`/`specstore` (the declared-domain engine)** — the engine's own
  fixtures live in `spec/testdata/` (a two-role `record` blueprint filled by
  a clinic and a garage domain, S18's note on why: so a bug here is never
  proven only against a consumer's shipped spec). `specstore`'s 8 tests
  round-trip an inline `ticket` spec through a real SQLite table, proving
  the by-name column join and the declared DDL agree about a bool coming
  back as `int64` and text coming back as `[]byte`.
- **`dispatch` (the runtime route/tool engine)** — `TestAnUndeclaredMethodIsNotRouted`
  pins deny-by-default; four tests cover `Anonymous`/`Unmatched`'s
  allowlist-of-what's-public shape; `TestTheAddressOutranksAContradictingBody`
  pins that a path `into` overwrite always wins over a body claiming a
  different value for the same field. S21's four ignored-segment refusals
  (may not also be whole/repeated/an overwrite; must name a declared
  wildcard) and S22's duplicate-declared-tool-name refusal are each their
  own test in `dispatch/tools_test.go`/`dispatch_test.go`.
- **`entitygraph`+`postgres`/`memory`** — one conformance suite
  (`entitygraphtest.Run`) exercises schema lifecycle, entity CRUD,
  upsert-by-unique-key, relationships and traversal against **both**
  backends, so the two cannot drift into answering the same call
  differently. `SchemaManager.Activate`'s single-transaction fix (the
  "never actually atomic" gap in the original ArangoDB source, per
  `CLAUDE.md`) is covered by `postgres`'s own backend tests when
  `POSTGRES_URL` is set.
- **`gormutil`/`httpwire`/`gormtest`/`vocab`** — each is a straight
  extraction from a GORM-era repo's own hand-rolled helper (S10's audit
  names the origin per package); their tests were written or ported
  alongside the extraction, not before it, so "does this behave like the
  five copies it replaces" is the actual thing under test — e.g.
  `gormutil`'s dialect-aware constraint classification covers both the
  Postgres SQLSTATE and the sqlite text-message path.

## What only a real Postgres covers

`postgres/backend_test.go`, `postgres/percollection_test.go`,
`orgpolicy/postgres_integration_test.go` and
`orgsettings/postgres_integration_test.go` all skip without `POSTGRES_URL`
— this repo's own suite is not run against a live database as part of a
routine sweep; standing up Postgres containers is against the family's
`[[feedback_use_memory_backend_for_tests]]` convention (invoked by this
repo's own S9 row in `todo_done.md`). This is an accepted,
already-documented gap (S8's own `todo_done.md` row flagged it before
`v0.1.0` was tagged) rather than an oversight.

## Consumers depend on this engine's guarantees holding

`mwanachama-backend-catalog` and `mwanachama-backend-agency` both build their
own storage on `spec`/`specstore`, and `mwanachama-wakala-api` mounts the
route tables `dispatch` builds. A regression in either package is invisible
to those repos' own suites (their fixtures exercise the *shape* they
declare, not the engine's refusal paths) — this repo's own `spec`/`dispatch`
tests are the only net for the shared engine's edge cases (collision
detection, the 63-byte truncation budget, the deny-by-default gate).
