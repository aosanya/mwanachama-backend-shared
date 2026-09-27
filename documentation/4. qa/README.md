# QA

Test coverage for `mwanachama-backend-shared`. Tests live beside the code
they cover; there is no separate test corpus. This page was itself stale —
it read "Empty at bootstrap — populate alongside S8" until the 2026-09-27
scheduled integration sweep found S8 long since landed and the suite far
from empty.

```sh
go test ./...                  # in-memory (glebarez/sqlite where a store
                                # backend is under test), no database needed
go test -tags=integration ./...  # same suites; a tagged file can add cases
                                  # that only compile under this tag
```

As of 2026-09-27: **164 test functions, all green**, across every package
this module ships — `dispatch` (55), `entitygraph` (23), `spec` (26),
`specstore` (8), `orgpolicy` (12) + `orgpolicy/models` (6), `orgsettings`
(10) + `orgsettings/models` (6), `gormutil` (13), `httpwire` (8), `vocab`
(2), `gormtest` (1), `memory` (1). `go test -tags=integration ./...` runs
the same suites and stays green (159 `--- PASS` lines — the count differs
from the untagged run's because `-tags=integration` recompiles some
table-driven tests into a different subtest shape, not because it skips
anything).

## What each package is actually tested as

- **`entitygraph`** — the Postgres-backed generic entity/relationship graph
  engine (`Entity`/`Relationship`/`DataManager`/`SchemaManager`) that
  `mwanachama-backend-git` and `mwanachama-backend-taskmanager` build their
  ported business logic against; see
  [documentation/2. design/](../2.%20design/) for the port's own record.
- **`spec`/`specstore`** — the declared-domain engine
  ([declared-domains.md](../2.%20design/declared-domains.md)):
  `spec.Load`/`spec.Migrate`, blueprint merge, and the validation this
  module's own `CLAUDE.md` describes (`NamePattern`/`DocPathPattern`
  refusing anything that would reach SQL unescaped, the 63-byte truncation
  budget, `TestRequiredFieldsHaveNoDefault`). `mwanachama-backend-catalog`
  and `mwanachama-backend-agency` (AGD-007) are its two real consumers —
  both repos' own `go test ./...` exercises this package's real code
  through their `replace` directive, not a mock of it.
- **`dispatch`/`httpwire`** — the operations half of the same shape
  ([dispatcher.md](../2.%20design/dispatcher.md),
  [httpwire.md](../2.%20design/httpwire.md)): resolving a declared
  operation by name against a loaded spec and wiring it onto real
  `net/http` routes, the largest single suite in the repo (55 functions).
- **`orgpolicy`/`orgsettings`** (+ their `models` subpackages) — policy and
  settings storage ported alongside the entity-graph engine, each with its
  own `gormstore` (no test files of its own — covered through the parent
  package's tests, matching `spec`/`specstore`'s own split).
- **`gormutil`** — shared GORM helpers (13 functions); **`gormtest`** — a
  test-only harness other packages' own tests import, so it carries exactly
  one test of itself; **`vocab`** — a small shared vocabulary package (2
  functions).
- **`postgres`** — `TestBackend_Conformance` and the two
  `TestPerCollectionBackend_*` cases are real conformance suites against a
  live Postgres, opt-in via `POSTGRES_URL` and skipped (not failed) without
  it — confirmed this run: `backend_test.go:21: POSTGRES_URL not set;
  skipping Postgres conformance test`. This is the one package where
  `go test ./...` passing does **not** mean the Postgres-specific behaviour
  was actually exercised.
- **`mcpui`** — no test files (a static CSS/theme package,
  `//go:embed`-only; see this repo's own `CLAUDE.md`). Nothing to test
  beyond "does it embed", which compiling already proves.

## What is not covered here

`entities`/`relationships`'s single-`entities`-table invariant, the
`SchemaManager.Activate` single-transaction guarantee, and
`TypeDefinition.UniqueKey` mapping to a real partial/composite unique index
are this module's own `CLAUDE.md` "Key invariants" — each has coverage
inside `entitygraph`'s 23 functions, not a separate suite; there is no
dedicated top-level invariants test file to point to by name.
