# Declared domains

`spec/` and `specstore/` are the storage half of the org-wide shape designed
in
[architecture-spec-driven-modules.md](../../../developer/documentation/2.%20design/architecture-spec-driven-modules.md):
a module's objects are **data**, not Go row structs, and its tables come from
that data. [dispatcher.md](dispatcher.md) is the other half — the same idea
applied to the route table and the MCP tools.

Both packages arrived here on 2026-09-24 (S18, S19), following the shape
`mwanachama-backend-catalog` proved first. `mwanachama-backend-agency` is the
confirmed consumer, converted under AGD-007 on its own board — it imports
this package directly and keeps no `spec/` package of its own.
`mwanachama-backend-catalog` itself has not converted: as of 2026-09-25 it
still carries its own separate, independent `spec/` package
(`github.com/aosanya/mwanachama-backend-catalog/spec`), not this one — a real
type mismatch (`catalog.ParseSpec`'s `*spec.Spec` is not this package's
`*spec.Spec`) surfaces wherever a caller mixes the two, as
`mwanachama-wakala-api`'s `internal/api/http/mcp_catalog_test.go` does. This
page is what a next repo — catalog included, when it converts — is converted
against.

## Two declarations, two owners

A **blueprint** is the module's. It names the roles the module's own rules
operate on and, for each, every field and the indexes every domain needs.

A **spec** is one domain's. It names which object fills each role, what the
domain calls it, which table it lands in, its own indexes, and — on a field
the module declared — a default, and nothing else.

| The module's blueprint | A domain's spec |
| --- | --- |
| the roles that exist | which object fills each role, and what it is called |
| every field: name, type, description, `required`/`unique`/`immutable`/`primary`, `values`, `matches` | `module`, `domain`, `instance`, and each object's `table` |
| the indexes every domain needs | its own indexes, including document-path ones over its own vocabulary |
| — | a **default**, and nothing else, on a field it fills a role with |
| — | objects of its own, which declare their own fields and take no role |

`Blueprint.Load`/`Blueprint.Parse` read a domain spec through the blueprint;
`Blueprint.Apply` is the merge, and it is deliberately unforgiving. A domain
that sets a type, a description or a value set on a declared field is refused
**by name** at load, as is one claiming a role the module does not declare or
adding a field to one it does. A domain that needs a field of its own
declares an object of its own, with no role.

The reason is drift. Before catalog's CAT5, each domain restated the module's
whole object set — 62 field declarations of which 60 were identical bar the
description — so renaming a field under one domain left the other wrong with
nothing failing until somebody loaded it.

A spec that needs its blueprint and is loaded with plain `spec.Load` says so
in the error, rather than reporting an object with no fields.

## What a field may say

Types are `string`, `text`, `int`, `bool`, `json`, `timestamp` and `enum`.
The set is closed: a type with no arm fails when the spec loads rather than
producing a column nobody meant. A timestamp is text, so every dialect
compares it the same way; `json` is `jsonb` on Postgres and text on SQLite,
which is what lets the unit tests run on SQLite while document queries run on
jsonb.

Flags are `primary` (several make a composite key, in declared order),
`required`, `unique`, `immutable` and `default`. An enum adds `values`, and
`matches` names a validation pattern — `slug`, `json_object`. **The pattern
registry is the module's**, not this package's: the spec names a pattern and
the module says what the name means, so a domain cannot invent one and a
module cannot be forced to carry another module's vocabulary. A spec naming a
pattern nobody supplies is an error, not a rule that quietly never runs.

Two refusals are worth knowing before they surprise anyone:

- **A description is required**, on every object and every field. A column
  nobody can explain is the one that rots, and the spec is the only place the
  explanation would live — it is also what the MCP tool schemas read.
- **A required field may not carry a default.** The default is exactly what
  would let an omitted value pass unnoticed.

## Indexes

An index is over columns (`"fields": ["state"]`) or over a **document path**
(`"path": {"field": "doc", "path": "identifiers.isbn"}`), which is how a
domain makes its own vocabulary fast without the module learning the word.
`unique` and `not_deleted` are the two modifiers; `not_deleted` is a named
condition rather than free SQL, because free SQL here would be an injection
surface with no upside.

`DocPathExpr` builds the expression, and both the index and the query filter
go through it. That is not tidiness: an expression index applies only to a
query whose expression matches it exactly, so a copy that drifted by a space
would leave every document filter doing a full scan with nothing reporting
it.

## Names reach the database as SQL text

A table name cannot be a bound parameter, so identifiers are **validated
against a strict alphabet rather than escaped**. `NamePattern` is lowercase,
starting with a letter, words joined by single underscores. Never relax this
to "escape it instead".

A physical name is `<instance>_<module>_<table>` — `agency_catalog_agencies`,
`agency_agency_goals`. The module segment is not decoration: without it a
catalog instance named `agency` and an agency instance of the same name both
want `agency_agencies`, and neither module notices, because `create table if
not exists` is a no-op against a table that already exists and GORM's
`AutoMigrate` would simply adopt it. That collision is silent data mixing,
not an error, which is why `Validate` refuses it rather than leaving it to
the database:

- `instance` and `module` match `SegmentPattern` — `NamePattern` minus the
  underscore — so the three segments can be read back apart. With `_` both
  the separator and legal inside a segment, `a_b_c_goals` does not say which
  part is which.
- Every emitted name is measured against `MaxIdentifier` (63). **Postgres
  truncates past it without complaining**, so two names agreeing that far are
  one relation. Index names are measured in the same namespace as tables,
  because Postgres keeps them there and `<table>_<index>_idx` overflows
  before its own table does.
- Every problem is reported, not just the first: a spec is edited by hand, and
  a list beats one round trip per mistake.

## Migrate is the whole storage story

`spec.Migrate(db, s)` creates a table per declared object and its indexes,
idempotently. There is no `AutoMigrate` and there are no row structs.
`Spec.DDL(dialect)` returns the same statements without running them, which
is what lets a spec be reviewed as SQL before it touches a database, and what
lets a test assert on the statements rather than on their effects.

What `Migrate` does **not** do is drop or alter anything. A column that stops
being declared stays in the database; a retirement is a migration the module
writes itself.

## The store

`specstore.New(db, spec, carriers)` takes a map of role to a zero value of
the Go type that carries it, and refuses to build when the two disagree — a
declared column with no field to hold it, or a carried field nothing
declares. That check runs once, at construction, because a column with no
field is a value that would be dropped on every single write with nothing to
show for it.

The join between a declared field and a Go field is **the field's name**, put
through `ColumnName`: `SubmittedBy` is `submitted_by`, `RunID` is `run_id`, a
run of capitals is one word. Deliberately not the json tag, which is a
presentation choice and gets this wrong exactly where it hurts:
`ShareLink.KeyHash` is `json:"-"` so a stored hash never reaches an API
response, and a tag-reading codec would quietly stop storing it — every share
key issued would then fail to match.

**Every declared column is written on every write.** A map missing a key
means "leave it alone" to an update, so omitting empty values would make
clearing a field impossible: an entry would keep the `approved_at` that its
document changing is supposed to drop. An empty `json` value is stored as
SQL NULL, so a column never written stays distinguishable from one holding
`""`.

The rest is four wrappers every converted repo would otherwise rewrite:
`Query(ctx, role)` starts a query against the role's table, `Take` reads at
most one row and answers the caller's own not-found sentinel, `List[T]` reads
many, and `Insert` writes one. `NewID` mints a storage key — the row structs
minted theirs in a GORM `BeforeCreate` hook, which a map-shaped write never
reaches.

Rows come back as `map[string]any`, and `Decode` takes the dialects as they
come: SQLite hands back `int64` for a boolean and either a string or a byte
slice for text, Postgres hands back a byte slice for jsonb.

## What stays in Go

Everything a spec cannot state, and it should live with the type it is about:
a field whose presence depends on another's value, a count that may not go
negative, the enumeration refusal, the stall counter, the write-once key,
merge-don't-erase. `required`, an enum's `values` and `matches` are read off
the spec and applied on the way in, so a domain that adds a fourth state gets
it validated with no Go change.

If the module keeps Go constants for an enum's values, hold them and the
blueprint to each other **in both directions** with a test. A stored value
outlives a rename, which makes a drifted constant a data bug rather than a
compile error.

## Converting a repo

The order matters, because each step is verifiable on its own:

1. **Declare the blueprint** from the existing row structs and their GORM
   tags, moving each field's Go doc comment into its `description`. Assert
   that it parses and covers every column the old migration creates.
2. **Write the domain spec**, pick the instance, and move to
   `<instance>_<module>_<table>` with a rename migration — plus the matching
   edit to the gateway's hand-maintained SQL mirror, which is what lets
   `cmd/migrate up` still provision a fresh database.
3. **Replace the store**: `specstore.New` with one carrier per role, the
   `models/` structs kept as carriers, the `gormstore/` row structs deleted.
4. **Declare the operations** and move `routes/` onto `dispatch` — see
   [dispatcher.md](dispatcher.md). The route type becomes `httpwire.Route`,
   which carries an `Action`, so every route arrives gated.
5. **Derive the MCP tools** from the same operations spec, keeping only the
   composites — a tool over several operations is not an operation.
6. **Rebuild every consumer**, then update the repo's `CLAUDE.md`, which will
   still be telling the next session to write row structs.

## Not solved here

- **Sequences.** Nothing declares a per-type counter, so a module minting a
  human-readable code (`G-1`) keeps that in Go. Agency is the first to need
  it; if a second does, it belongs here.
- **Filters.** Scope, case-insensitive `LIKE` and exact-match are rebuilt per
  repo. Portability is the catch: `ILIKE` is Postgres-only, and these tests
  run on SQLite.
- **Pagination**, which neither half of the shape declares yet.
- **Per-instance provisioning.** `mwanachama-wakala-api` creates a table set
  per registered instance on demand rather than migrating once, and a
  registry that claims each physical name it creates does not exist yet.
