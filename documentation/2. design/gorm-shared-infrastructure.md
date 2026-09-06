# GORM-era shared infrastructure — audit and extraction plan

Companion to the entity-graph design in this same folder. That design covers
what this repo already ships (the Postgres entity-graph engine, still
consumed by [mwanachama-backend-git](../../../mwanachama-backend-git) history
and [mwanachama-backend-taskmanager](../../../mwanachama-backend-taskmanager),
[mwanachama-backend-accounting](../../../mwanachama-backend-accounting), and
[mwanachama-backend-digitaltwin](../../../mwanachama-backend-digitaltwin)).
This document covers the *newer* thing: five backend repos independently
migrated off entitygraph onto GORM over 2026-09-04–09-06, and in doing so
each rebuilt the same handful of pieces of infrastructure from scratch —
`mwanachama-backend-actor` first, then every repo after it copied actor
near-verbatim (several say so in their own comments). None of that shared
shape lives in this repo today; this document is the audit of what it should
absorb, and what should wait.

Audited: all nine `mwanachama-backend-*` domain repos plus
`mwanachama-backend-api-gateway` (the mounting process, and — for the JSON
wire helpers — the actual origin repo). Not audited: the frontend/website
repos, which are out of scope for a Go shared library.

## Current state, per repo

| Repo | Storage | HTTP layer | Migrated to GORM |
|------|---------|------------|-------------------|
| [actor](../../../mwanachama-backend-actor) | GORM | `routes/` | 2026-09-04 (first — the reference shape) |
| [assetmanager](../../../mwanachama-backend-assetmanager) | GORM | `routes/` | 2026-09-04 (same day as actor) |
| [git](../../../mwanachama-backend-git) | GORM | `routes/` | 2026-09-04 |
| [comm](../../../mwanachama-backend-comm) | GORM | `routes/` | 2026-09-04 |
| [forms](../../../mwanachama-backend-forms) | GORM | `routes/` | 2026-09-04 |
| [taskmanager](../../../mwanachama-backend-taskmanager) | entitygraph (this repo) | none — library only | not started |
| [accounting](../../../mwanachama-backend-accounting) | entitygraph (this repo) | none — library only | not started |
| [digitaltwin](../../../mwanachama-backend-digitaltwin) | entitygraph (registry) + raw SQL (telemetry sidecar) | none — blocked on a capability-model decision | not started, one deliberate exception (telemetry bypasses entitygraph on purpose — see its CLAUDE.md) |
| [api-gateway](../../../mwanachama-backend-api-gateway) | raw `database/sql`/pgx (own store) + entitygraph (mid-retirement) + imports actor's and forms' GORM packages as libraries | `internal/api/http` — the origin of the wire-helper shape every GORM repo copied | n/a — the mounting process, not a domain repo |

Two things worth naming up front:

- **This repo's one existing extraction already works.** `postgres.Open`/
  `postgres.Config` (`postgres/conn.go`) — ported from the gateway's own
  `internal/store/postgres/conn.go` — is already imported by every GORM
  repo's `postgres_integration_test.go` to open the underlying `*sql.DB`
  before wrapping it in GORM's Postgres dialector. That's the precedent:
  extraction here doesn't require a repo to adopt GORM through this
  package, only to share the *connection* plumbing underneath it.
- **The duplication is not hypothetical or stylistic — it's the same code,
  independently retyped.** `forms/routes/wire.go` and `comm/routes/wire.go`
  both say outright, in their own comments, that they were copied verbatim
  from actor's. `git/routes/wire.go` is byte-for-byte identical to actor's
  aside from comments. Every one of the five GORM repos has a `tables.go`
  that git's audit describes as "near line-for-line the same file with
  names substituted." This has already produced a real inconsistency (see
  "Nullable-string helpers" below) — proof that copy-and-rename drifts
  faster than it looks like it should.

## Confirmed duplication — safe to extract now

These appear, independently built, in most or all of the five GORM repos.
Extracting them changes nothing about any repo's behavior; it only deletes
copies.

### 1. JSON wire helpers (`writeJSON` / `writeErr` / `readJSON`)

Origin: `mwanachama-backend-api-gateway/internal/api/http/wire.go`. Copied
into `actor/routes/wire.go`, then into `assetmanager`, `comm`, `forms`, and
`git`'s `routes/wire.go` — all five are the same three functions:

```go
func writeJSON(w http.ResponseWriter, code int, v any) { ... }
func writeErr(w http.ResponseWriter, code int, msg string) { ... }
func readJSON(r *http.Request, v any) error { ... }
```

The gateway's own copy carries two extras none of the library repos need
yet: `readJSONFields` (decode-twice, for partial-update present-vs-null
semantics) and `bearer(r)` (Authorization header extraction) — the
extraction should offer these but not force every repo to use them.

### 2. `routes.Route` / `Routes` aggregator shape

Every GORM repo's `routes/routes.go` defines the same `Route{Method, Path,
Handler}` with a `.Pattern(prefix)` method and a `Routes` aggregator; two
of them (`actor`, `forms`) additionally carry a `ResourceNames` override
struct so a mounting process can relabel a noun that's org-configurable
(actor's `Group` → the gateway's "chapter"). `assetmanager` and `git`
deliberately skip `ResourceNames` — their nouns aren't relabeled by anything
that mounts them. The shared version should make `ResourceNames` optional,
not assumed.

### 3. Root `tables.go` wrapper

```go
type TableNames = gormstore.TableNames
func DefaultTableNames(instance string) TableNames { return gormstore.DefaultTableNames(instance) }
func Migrate(db *gorm.DB, t TableNames) error { return gormstore.Migrate(db, t) }
```

Identical shape in all five repos, differing only in the `gormstore` type
each delegates to. Not literally extractable as one function (each repo's
`TableNames` struct has different fields), but generalizable with Go
generics: `func Migrate[T any](db *gorm.DB, rows map[string]T) error` plus a
thin per-repo `gormstore.Migrate` that supplies the row map — worth
prototyping against `assetmanager` (4 tables, simplest case) first.

### 4. `testdb_test.go` (in-memory sqlite harness)

All five repos open `glebarez/sqlite` in-memory, migrate via the repo's own
`Migrate`, and construct the manager — actor's `newTestManager` is the
template comm and forms both cite by name. `git`'s copy differs in two
deliberate ways worth preserving as options rather than losing in the
extraction: it stays in the internal (non-`_test`) package because some of
its tests need unexported fields, and it pins `SetMaxOpenConns(1)` because
git's tests exercise genuine concurrency the other repos' single-goroutine
suites don't.

### 5. `postgres_integration_test.go` (opt-in Postgres harness)

Same shape in all five: skip unless `POSTGRES_URL` is set, open it via this
repo's own `postgres.Open`, wrap in `gorm.io/driver/postgres`, migrate a
unique-per-test table prefix (`"useri"`, `"assetit"`, `"commi"`, `"formsi"`,
git's own), drop tables in `t.Cleanup`. Since every copy already depends on
this repo's `postgres.Open`, this is the most natural next extraction —
it only needs a thin GORM-dialector wrapper on top of what already exists
here.

### 6. UUID generation via `BeforeCreate`

`actor`, `forms`, `git`, and `comm` all mint row IDs the same way — a GORM
`BeforeCreate` hook calling `uuid.NewString()` when the ID field is empty.
Trivial, but worth one shared helper (`gormutil.SetUUIDIfEmpty(id *string)`
or a generic `BeforeCreate` mixin) so the next repo doesn't retype it a
sixth time.

## Confirmed once, but on track to be reinvented — generalize pre-emptively

These exist in only one repo today, but the shape of the problem is
generic enough, and the org's copy-actor's-homework pattern is strong
enough, that documenting and extracting now is cheaper than waiting for a
second bespoke copy.

### 7. Driver-error classification (`comm/errors.go`'s `classify`)

`comm` is the only repo that translates raw GORM/driver errors into
sentinel errors by SQLSTATE: Postgres `pgconn.PgError` codes 23502
(not-null), 23503 (FK), 23505 (unique), 23514 (check), with a text-matching
fallback for sqlite (which doesn't surface structured SQLSTATEs the same
way). Every other GORM repo either returns raw GORM errors from a failed
`Create`/`Update` or hand-rolls a narrower pre-check (actor's
`checkUniqueAttributes`, assetmanager's serial-tag pre-check) instead of
classifying the DB's own answer. This is the single highest-value
un-shared piece surveyed: it's dialect-aware, storage-agnostic, and every
GORM repo needs exactly this, not a domain-specific variant of it.

### 8. Postgres-only raw-SQL constraint sync, layered on `AutoMigrate`

Three independent versions of the same idiom — GORM's `AutoMigrate` can't
express a partial unique index or a multi-column `CHECK`, so each repo
drops to a raw `db.Exec(...)` behind a dialect switch, run once from
`Migrate`:

- `actor/gormstore/tables.go`'s `syncUniqueAttributeIndexes` — one partial
  unique index per `Unique` `models.Property`, dialect-branching between
  Postgres `attributes ->> 'x'` and sqlite `json_extract(attributes,
  '$.x')`.
- `assetmanager/gormstore/tables.go`'s `syncSerialTagUniqueIndex` — a
  fixed partial unique index (`WHERE deleted = false AND serial_tag <>
  ''`).
- `forms`'s `syncConstraints` — postgres-only (explicitly skipped on
  sqlite, same as actor's), applying a trigger, a multi-column `CHECK`,
  and a partial unique index.

The dialect-detection scaffolding (`switch db.Dialector.Name() { case
"postgres": ...; case "sqlite": ...; default: continue }`, `CREATE ... IF
NOT EXISTS`, naming the constraint `<table>_<thing>_uniq`) is identical
across all three. The JSON-path-vs-plain-column expression building is the
only part that's genuinely different per caller, and that's a small enough
seam to parameterize.

### 9. Error-to-HTTP-status mapping

Every repo with a `routes/` package hand-rolls a sentinel-error → HTTP
status mapper: actor has one `statusFor` per resource file, forms
collapsed that into a single package-wide `formStatusFor` mapping ~20
sentinels (its own comment explains why: too many shared handlers to
duplicate per-resource), git has `gitStatusFor`, and the gateway itself has
`storeErr` plus a `notFoundErrors` list. A tiny shared helper —
`httpwire.StatusFor(err error, table map[error]int, fallback int) int` —
would let each repo keep its own error→status table (those are genuinely
domain-specific) while sharing the lookup-and-fallback mechanics.

### 10. Closed-vocabulary + state-machine idiom

`taskmanager` (`TaskStatus`, `WorkflowRunStatus`), `accounting`
(`AccountKind`/`AccountType`/`DocumentKind` in `vocabulary.go`), and
`digitaltwin` (`AssetType`/`StationType`/`AssetStatus`, explicitly modeled
on taskmanager's per its own CLAUDE.md) all independently built the same
pattern: a `map[T]bool` membership set plus, where the value has lifecycle,
a `CanTransitionTo` method. This one is notable because it's **storage-
agnostic** — it doesn't care whether the repo is on entitygraph or GORM —
so it's the one item on this list that benefits every repo audited, not
just the five that migrated. Pure Go, no dependencies; a generic
`vocab.Set[T]`/small transition-table helper is a same-day extraction.

### 11. Conformance-test-against-multiple-backends pattern

This repo already has exactly this pattern for its own entity-graph engine
— [`entitygraphtest/`](../../../entitygraphtest)'s `Run(t, dm, sm,
agencyID)`, exercised against both `memory` and `postgres` so the two
backends can't silently drift apart (see `todo_done.md`'s S8). `accounting`
independently reinvented the same idea one level up, as
`RunLedgerConformance(t, factory)` run against its `MemoryRepository` and
`PostgresRepository`. Not a candidate for literal code-sharing (each
domain's interface differs), but worth writing up as a named, documented
convention — "if your package ships more than one backend, give it one
`Run(t, impl)` conformance suite, not two hand-written test files" — since
this repo is already proof it holds up.

## What should stay put, or wait

- **`StringToNullable`/`nullableToString`** — present in `actor`,
  `assetmanager`, `comm`, `git`, but **not consistent**: actor and
  assetmanager keep both unexported; comm and git export
  `NullableToString`. Worse, `forms` deliberately has *neither* — its own
  CLAUDE.md states every optional field is a plain `string` with `""` as
  the unset sentinel, specifically to avoid the nullable-pointer problem
  rather than solve it. Extracting this today means picking a winner
  between two real, already-shipping conventions (nullable `*string` vs.
  empty-string sentinel) org-wide. That's a design decision for whoever
  owns this repo's roadmap, not something to resolve silently inside an
  extraction — call it out as an open question (see Sequencing below)
  before writing a `gormutil.StringToNullable`.
- **`models/property.go`'s `Property`/`ValidateAttributes`/
  `checkUniqueAttributes` system** (actor only) — a real, working
  Required/Range/Unique/Options validator over a `map[string]any`
  attribute bag, paired with a partial-unique-index sync (item 8, above).
  `forms` explicitly opted out ("no free-form attribute bag — every field
  is fixed-shape"), and `assetmanager` deliberately deferred promoting its
  own `AttributesJSON` to this shape. This is the org's most complete
  typed-attribute-bag pattern and a strong future candidate, but with only
  one adopter it's premature to extract — watch for a second repo that
  needs it (also flagged in [[consider-jsonschema-go]] as a possible fit
  for a schema-validation library instead of hand-rolling this again).
- **`comm/routes/identity.go`'s `Identity` interface** (`CallerID`,
  `CallerDeviceID`) — comm-only, needed because its DM handlers act on the
  caller's own session rather than a URL path segment. No second
  consumer yet; wait.
- **Capability/auth middleware** (`api-gateway/internal/api/http/
  capability.go`) — deliberately gateway-only, and every GORM repo's
  `routes/` package explicitly does not gate auth itself (the mounting
  process wraps the returned `http.HandlerFunc`). This is a boundary to
  keep, not code to extract — worth stating explicitly in whatever package
  doc introduces a shared `routes` helper, so nobody "fixes" a library
  repo by adding auth to it.
- **`digitaltwin`'s raw-SQL telemetry sidecar** and **`git`'s
  BFS-over-SQL-edges `NeighborhoodEdges`** — both are deliberate,
  domain-specific escapes from their repo's main storage pattern (documented
  as such in each repo's own CLAUDE.md). Neither generalizes; not
  candidates.

## Proposed package layout

All new, under this repo, none breaking existing consumers of `entitygraph`/
`postgres`/`memory`/`events`/`schema`:

| Package | Contents | Extract from |
|---|---|---|
| `httpwire/` | `WriteJSON`/`WriteErr`/`ReadJSON`(/`ReadJSONFields`), `Route`/`Routes`/`Pattern`, `StatusFor` | api-gateway (origin), actor (routes shape) |
| `gormutil/` | Nullable-string helpers (pending the open question above), `SetUUIDIfEmpty`, `ClassifyError` (ported from comm's `classify`), the dialect-aware partial-unique-index/constraint-sync helper | comm (`classify`), actor/assetmanager/forms (constraint sync) |
| `gormtest/` | `OpenSQLiteDB(t) *gorm.DB`, `OpenPostgresDB(t, prefix string) (*gorm.DB, cleanup func())` wrapping this repo's existing `postgres.Open` | actor/assetmanager/comm/forms/git (testdb_test.go, postgres_integration_test.go) |
| `vocab/` | Generic closed-membership set + transition-table helper | taskmanager/accounting/digitaltwin |

`postgres.Open`/`postgres.Config` stay where they are — already correctly
shared, nothing to move.

## Sequencing

1. **Resolve the nullable-value question first** (blocks `gormutil`'s
   string helpers) — pick nullable-pointer or empty-string-sentinel as the
   org convention, or explicitly bless both and give the helper a name
   that doesn't imply "the" way.
2. Extract `gormtest/` first regardless — it has zero design decisions
   left to make (five repos already agree on the shape) and immediately
   deletes the most repeated file in the family.
3. Extract `httpwire/` next — same reasoning, and it's the one three repos'
   own comments already call "byte-for-byte" or "copied verbatim," so
   there's no behavior to reconcile, only files to delete.
4. `gormutil/`'s `ClassifyError` and the constraint-sync helper next —
   generalize from comm's and actor's existing code respectively; both are
   additive (no repo is forced to adopt them until it wants to).
5. `vocab/` can happen independently of the above, any time — it's the one
   piece that also helps the three entitygraph repos, not just the GORM
   five.
6. Existing repos adopt the shared packages opportunistically, one at a
   time, swapping a local copy for the import — not a coordinated
   flag-day migration. `assetmanager` (smallest GORM repo, 4 tables) is
   the cheapest first adopter to validate each package against before
   `git` (largest, most divergent) adopts.

Tracked as open tasks on [todo.md](../3.%20implementation/todo.md), S11
onward.
