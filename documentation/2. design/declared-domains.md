# Declared domains

`spec/` and `specstore/` are the storage half of the org-wide shape designed
in
[architecture-spec-driven-modules.md](../../../mwanachama-developer/documentation/2.%20design/architecture-spec-driven-modules.md):
a module's objects are **data**, not Go row structs, and its tables come from
that data. [dispatcher.md](dispatcher.md) is the other half — the same idea
applied to the route table and the MCP tools.

Both packages arrived here on 2026-09-24 (S18, S19) from
`mwanachama-backend-catalog`, which proved the shape first and is still the
reference consumer. `mwanachama-backend-agency` is the second, converting
under AGD-007 on its own board. This page is what a third repo is converted
against.

## Two declarations, two owners

A **blueprint** is the module's. It names the roles the module's own rules
operate on and, for each, every field and the indexes every domain needs.

A **spec** is one domain's. It names which object fills each role, what the
domain calls it, which table it lands in, its own indexes, and — on a field
the module declared — a default, plus further `values` where that field is
an enum.

| The module's blueprint | A domain's spec |
| --- | --- |
| the roles that exist | which object fills each role, and what it is called |
| every field: name, type, description, `required`/`unique`/`immutable`/`primary`, `values`, `matches` | `module`, `domain`, `instance`, an optional `mount`, and each object's `table` |
| the indexes every domain needs | its own indexes, including document-path ones over its own vocabulary |
| — | a **default** on a field it fills a role with, and **further `values`** on an enum |
| — | objects of its own, which declare their own fields and take no role |

`Blueprint.Load`/`Blueprint.Parse` read a domain spec through the blueprint;
`Blueprint.Apply` is the merge, and it is deliberately unforgiving. A domain
that sets a type, a description or any rule on a declared field is refused
**by name** at load, as is one claiming a role the module does not declare or
adding a field to one it does. A domain that needs a field of its own
declares an object of its own, with no role.

The reason is drift. Before catalog's CAT5, each domain restated the module's
whole object set — 62 field declarations of which 60 were identical bar the
description — so renaming a field under one domain left the other wrong with
nothing failing until somebody loaded it.

A spec that needs its blueprint and is loaded with plain `spec.Load` says so
in the error, rather than reporting an object with no fields.

### Widening an enum

A domain may add values to an enum the module declares. It **widens** the
module's set; it never replaces it:

```json
{"role": "notification", "name": "announcement", "table": "announcements",
 "description": "...",
 "fields": [{"name": "event", "values": ["report_card_published"]}]}
```

The module's own values come first and are always kept, because a module
writes some of them itself — comm's `message_removed` is its own moderation's
removal receipt, and a domain that forgot to restate it would break the
module rather than its own instance. Restating a value the module already
permits is refused, since it reads as an override and is not one, and
`values` on a field that is not an enum is refused by the field's declared
type. The merge runs before validation, so a domain may default to a value it
adds in the same breath.

The alternative was worse in both directions. A closed vocabulary had only
two homes: the module blueprint, where it is closed against every domain, or
nowhere, unvalidated. comm took the first and ended up declaring fourteen
civic-programme words — `survey`, `contribution`, `merchandise` — in a module
that raises exactly one of them itself. A carrier should not own its callers'
vocabulary.

What this does **not** solve is a value that constrains another field's
value. comm pairs each notification event with the category it is muted
under, in Go; a domain that adds an event still has nowhere to declare its
category. That is comm's CM26 and is open.

## What a field may say

Types are `string`, `text`, `int`, `float`, `bool`, `json`, `timestamp`,
`enum`, `bytes` and `geometry`. The set is closed: a type with no arm fails when the spec loads
rather than producing a column nobody meant. A timestamp is text, so every
dialect compares it the same way; `json` is `jsonb` on Postgres and text on
SQLite, which is what lets the unit tests run on SQLite while document
queries run on jsonb.

### Carrying opaque bytes

`bytes` is `bytea` on Postgres and `blob` on SQLite, and is carried as a Go
`[]byte`. It exists because **a text column cannot hold key material**: raw
bytes are not valid UTF-8, so storing them as `text` is lossy, and storing
them as `json` would base64 them into a document — a strange home for a
secret and a different column type from the `bytea` such a value already
lives in. `mwanachama-backend-auth`'s phone-hashing salt is the first
consumer.

A bytes column is **opaque to the module**, so two declarations are refused
by name rather than quietly ignored: it cannot carry a `default` (there is
no way to write raw bytes as a SQL default), and it cannot name a `matches`
pattern (a pattern over bytes could never be applied).

It **may** be `primary`. Nothing in the engine renders a key as text —
`Primary` is read only to emit the key columns, the not-null and the absence
of a default — and both dialects index and compare `bytea`/`blob` natively.
`mwanachama-backend-comm`'s address hash is exactly this: the keyed hash of
an address, which is the row's whole identity and never a path parameter. It
is `bytea` in the live database, so declaring it `text` instead would have
meant converting every existing row on adoption.

A `string` carrier on a declared bytes column is refused at `specstore.New`, the same direction as
every other carrier disagreement — this is the `reflect.Value.String()`
placeholder class of bug the kind guards exist to stop.

**Only a Postgres run proves the column type.** SQLite is dynamically typed
and stores whatever it is handed, so a `bytes` field wrongly emitted as
`text` still round-trips there — the same blindness that hides an adopted
`timestamptz`. `specstore/bytes_test.go` therefore asserts on the generated
DDL as well as on the round trip.

### Carrying a place

`geometry` holds a place as a GeoJSON geometry, and only one of four kinds:
`Point`, `LineString`, `Polygon` or `MultiPolygon` (member app decisions 302
and 306, row S65; `LineString` added for a street's line, digitaltwin W28). A
line needs at least two positions; it holds a point only when the point lies
on one of its segments, and its bounding box is its positions'. It is stored exactly as a `json` document is — `jsonb` on Postgres,
text on SQLite — so a path index can read into it like any other document.
What differs is that **it is checked on every write**: `specstore` parses
the value with `geo.Parse` before the row is written and refuses anything
else, so a one-point line, a ring that does not close, a ring of fewer than
four positions, a longitude outside -180..180 or a latitude outside -90..90
never reaches the table. A third coordinate (a height) is accepted and
dropped, and foreign members such as `bbox` are ignored.

It may be carried as a `geo.Geometry`, which reads back already parsed, or
as a `string` holding the GeoJSON as a client sent it. An empty string or
the zero `geo.Geometry` is SQL NULL, and a nullable field is carried by a
`*geo.Geometry` like any other. Three declarations are refused by name: a
`default` (no domain has one place every row starts at), a `matches`
pattern (the value is checked as GeoJSON, not against a pattern) and
`primary` (a shape is not an identity).

`geo/` is the small package of pure functions over those three kinds, with
no storage and no import of `spec`:

- `Contains(g, p)` is point in polygon by ray casting. A point on an edge
  or a vertex is inside, and a point inside a hole is outside (on the hole's
  edge it is still inside). For a `Point` it is equality.
- `Distance(a, b)` is the great-circle distance in metres (haversine, mean
  Earth radius 6 371 008.8 m), good to well under a percent at the scale of
  a town or a county.
- `BoundingBox(g)` is the smallest longitude/latitude box holding the
  outer rings. It does not handle a shape crossing the antimeridian, which
  no place the member app serves does.
- `RoundForPublic(p)` rounds both coordinates to three decimal places, a
  cell about 111 m tall and no wider, so a point read back on a public
  surface sits within about 80 m of the true one and every home in the
  same cell reads the same. A module applies it on a public read; the
  stored value is never rounded.

Positions are `[longitude, latitude]`, GeoJSON's order, never the other way
round.

### Carrying a timestamp

A `timestamp` column may be carried either as a `string` already holding
RFC 3339 text, which is what the first eight converted modules do, or as a
`time.Time`, which `specstore` converts at the edge. `*time.Time` pairs with
`nullable` the same way any pointer does.

The stored text is fixed-width and always UTC — `specstore.TimeLayout`,
nine fractional digits, a literal `Z`:

```text
2026-09-30T14:05:06.123456789Z
```

Both halves of that matter, because a timestamp is a **text** column and
`ORDER BY` over it is a string comparison. `time.RFC3339Nano` trims trailing
zeros, so it would sort `00:00:00.5Z` before `00:00:00.05Z`; a preserved zone
offset would sort two identical instants apart. A module that carries its
timestamps as strings is responsible for the same property in whatever it
writes.

Reading is lenient where writing is strict: a `time.Time` field accepts the
layout above, `RFC3339Nano`, a Postgres `timestamptz` rendering, a bare
`2006-01-02 15:04:05`, and a `time.Time` handed straight back by a driver —
which is what a column still typed `timestamptz` under an adopted legacy
table gives.

A carrier the codec has no arm for is **refused by name**. This is not
defensive: `reflect.Value.String()` does not panic on a non-string kind, it
answers `"<time.Time Value>"`, so before the refusal a `time.Time` or a
`[]byte` on a declared `string` column was written as that placeholder text
and nothing said so until the row was read back.

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
- **A numeric or boolean default must parse as its own type** (S55). An `int`
  default is read with `strconv.ParseInt`, a `float` with `ParseFloat` and only
  a finite result passes, a `bool` with `ParseBool`. `Validate` refuses anything
  else at load, and the column then carries the parsed value rather than the
  text, so `"+7"` becomes `default 7`, `"1e3"` becomes `default 1000` and a
  `bool` of `"1"` becomes `default true`. A default is the one declared value
  that reaches the DDL without an identifier alphabet to hold it, which is why
  it is typed rather than escaped. `Migrate` repeats the check for a spec that
  was built or edited without going through `Parse`, and `Spec.DDL`, which
  cannot return an error, quotes such a default as a string so it never reaches
  the statement as SQL.

## Indexes

An index is over columns (`"fields": ["state"]`) or over a **document path**
(`"path": {"field": "doc", "path": "identifiers.isbn"}`), which is how a
domain makes its own vocabulary fast without the module learning the word.
`unique`, `not_deleted` and `where` are the modifiers. `where` narrows the
index to the rows that satisfy every one of its `{field, op}` conditions,
over declared fields only, with `not_empty`, `is_true` and `is_false` as the
operators. They are named conditions rather than free SQL, because free SQL
here would be an injection surface with no upside, and `not_deleted` is now
simply the shorthand for `{"field": "deleted", "op": "is_false"}`.

```json
{ "name": "serial_tag_uniq", "fields": ["serial_tag"], "unique": true,
  "not_deleted": true, "where": [{ "field": "serial_tag", "op": "not_empty" }] }
```

A unique index usually needs this the moment the column is optional, and
getting it wrong is quiet: without the `not_empty` condition above, the
second row that leaves `serial_tag` blank collides with the first, because
`''` is a value like any other. The load-time checks refuse an unknown
operator, an unknown field, an operator the field's type cannot stand
(`not_empty` on an int) and a condition stated twice.

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

A physical name is **`<instance>_hashOf(<mount>)_hashOf(<module>_<object>)`**
— `wakala_0d6e4079_b21db826d3c0` for catalog's `suggestion_comments` object
mounted as `main` on instance `wakala` (verified by calling `TableFor`
directly). Only the instance stays readable, with one exception: the
`_test` suffix of a test copy (below). The mount and the object each get their
own digest rather than sharing one.

- **`instance`** separates tenants, and stays readable so a table can be
  attributed to one at a glance. It is the agency's `tableSlug()` for a module
  mounted per agency, or the configured spec instance for a global one.
- **`module`** separates modules. Without it a catalog instance named `agency`
  and an agency instance of the same name both want one table.
- **`mount`** separates two mounts of the *same* module within one instance —
  a tenant with a supplier catalog and a product catalog. `MountKey` is an
  8-hex-character SHA-256 digest of the mount name alone (`HashMount`), so
  every table of one mount sorts together as a block regardless of how long
  that mount's name is.
- **`object`** is a 12-hex-character SHA-256 digest (`HashName`, via
  `HashLength`) of the **raw name** `<module>_<object>` — not
  `<module>_<mount>_<object>`; the mount is hashed on its own, separately,
  into the segment above.

`RawNameFor` builds `<module>_<mount>_<object>` for the name registry;
`TableFor` joins `instance`, `MountKey()` and `HashName` of the *module/object*
pair (dropping the mount from that second digest, since it already has its
own segment); `IndexFor` keeps a readable suffix on the hashed table
(`<table>_<index>_idx`) — the one piece of debuggability left, and it fits.

### A test copy beside every table

Decision 85 (a) to (c): every declared object has a second table, named like
the live one with **`_test`** added (`TestSuffix`), so
`wakala_0d6e4079_b21db826d3c0` has
`wakala_0d6e4079_b21db826d3c0_test`. This adds one readable piece to the
rule above, at the very end of the name; a live name ends in hex, so the suffix
is never ambiguous. `TestTableFor` builds it, `LiveTableOf` strips it and
`Store.TestTable(role)` names it for a role. `RawNameFor` is unchanged: the raw
name belongs to the object, not to a copy of it, and the name registry records
live tables only. `LookupName` given a test table answers the live record.

- **Created together.** `Migrate` creates, evolves and indexes the copy in the
  same pass that does the live table, and `DDL` emits it after the live
  tables, so switching a module on is the only moment either exists and
  nothing is created while a test run is running. `AlterDDL` and the evolving
  `Migrate` give the copy every column the live table gains.
- **A run id column.** The copy has `run_id` (`RunIDColumn`) as its first
  column, `text not null` with no default, which the live table does not have.
  A row that does not name its run is refused by the database, and a spec
  field named `run_id` is refused at load. The name `run_id` is this change's
  choice; decision 85 left the column's name unsettled.
- **One copy shared by all runs.** So that two runs can each hold "ticket 1",
  the copy's primary key is `(run_id, <declared key>)`, every declared index
  starts with `run_id`, and a field marked `unique` becomes a unique index on
  `(run_id, field)` named `<test table>_<field>_uq` rather than a table-wide
  constraint. Uniqueness therefore holds inside one run and never across runs.
  The live table keeps its own key and constraints.
- **Not copied.** The settings tables (`spec_settings_` tables, see "A module declares its settings") are the
  engine's, not the module's data, so they have no copy; a run reads the live
  settings.
- **Length.** The copy's table name is five bytes longer and its indexes five
  bytes longer than the live ones, so the load-time check measures them too:
  an index that fits live at 59 to 63 bytes is refused, because the copy's
  name would be truncated silently. This is the one way a spec that loaded
  before can fail to load.
- **Counting tables.** The copies are real tables, so anything counting or
  listing tables must exclude `%_test` as it excludes `%_spec_table_names`.

### Writing into a copy, one run at a time (HS280)

`Store.ForRun(runID)` returns a store over the same database and spec whose
`Table` is the copy, whose `Query` is the copy filtered to `run_id = <run>`,
and whose `Insert` and `Row` stamp the run id on the row. A module that reaches
its tables through `Query`, `Row` and `Insert` therefore writes only to the
copy, and two runs that use the same key never meet. A module that builds its
own SQL against `Table` sees the copy's name but not the run filter, so it must
filter on `run_id` itself.

`RowsOfRun(ctx, db, spec, runID)` counts a run's rows in each table of the
spec, which is what a run's "what was stored" lists, and `RemoveRun` deletes
them, so removing a run leaves the live tables and every other run's rows
alone. An empty run id is refused by all three, so a missing id can never
widen to the whole copy or fall back to the live table.

### Why a hash, and what it costs

The readable form ran out of room. Measured against the real 11-character
slug `mwanachama-wakala-api` mints (`"agy"` + 8 hex), the longest identifiers
were already **62 of 63 bytes** (catalog's
`suggestion_comments_suggestion_created_idx`) and 61 (taskmanager's
`workflow_runs_parent_workflow_run_idx`). Adding any further segment refused
those two modules outright for every real instance — caught by a failing
provision, not by review, because every fixture uses a short instance name
like `wakala` or `clinic` and none of them reach the ceiling.

Hashing makes the length **constant**: 28 bytes for a table, 51 for the worst
index, however long the mount name is.

What it costs is legibility. `psql \dt`, a backup, an ad-hoc query and the
gateway's hand-maintained SQL mirror all see digests. That is what the name
registry is for, and it is why the registry is not optional.

### The name registry

`spec.Migrate` writes one row per object into **`spec_table_names`**
(`NameRegistryTable`), carrying `physical`, `instance`, `module`, `mount`,
`object` and `raw` — the parts split out so the table can be queried without
re-parsing a name. It is upserted, so re-migrating stays a no-op, and
`LookupName` reads one back.

**`TableFor` is pure and deterministic**: the physical name is derivable from
the spec alone, and the registry is a record for humans and tools, never a
lookup the code performs. A lost registry therefore costs legibility, not
correctness — but it costs *all* of it, so the registry belongs in whatever
gets backed up.

One practical consequence: the registry is a real table, so anything counting
or listing tables must exclude it.

Constraints on the segments:

- `instance`, `module` and `mount` match `SegmentPattern` — `NamePattern`
  minus the underscore — so the raw name can be read back apart. The object's
  own table name is last, so it keeps its underscores.
- Every emitted name is still measured against `MaxIdentifier` (63). A hashed
  table cannot overflow on its own, but the **instance is not hashed**, so a
  long enough instance still can, and an index suffix adds to it. **Postgres
  truncates past the limit without complaining**, so this check stays.
- Every problem is reported, not just the first: a spec is edited by hand, and
  a list beats one round trip per mistake.

## Migrate is the whole storage story

`spec.Migrate(db, s)` creates a table per declared object and its indexes,
and a `_test` copy of each (see [a test copy beside every
table](#a-test-copy-beside-every-table)), idempotently. There is no `AutoMigrate` and there are no row structs.
`Spec.DDL(dialect)` returns the create statements without running them, which
is what lets a spec be reviewed as SQL before it touches a database, and what
lets a test assert on the statements rather than on their effects.

`Migrate` evolves a table that already exists, additively, with one relaxation.
What it does **not** do is drop, rename or retype anything: a column that stops
being declared stays in the database with its data, and a retirement is a
migration the module writes itself. The one thing it does to such a column is
let go of its `not null` (see below).

### A table that already exists gains the columns it lacks

`create table if not exists` is a no-op on a table that is there, so a declared
object that gained a field after its table was made used to keep its old
columns, and the first write naming the new one failed with a raw driver error
(`column "chat_enabled" of relation "..." does not exist`, SQLSTATE 42703). The
AI backend's `Account` met it on 2026-10-28 when it gained `chat_enabled`,
`chat_label`, `last_test_prompt`, `last_test_reply` and `last_test_ms`.

For each stored object, in declared order, `Migrate` now:

1. runs the object's `create table if not exists`;
2. reads the table's columns (`information_schema.columns` on Postgres,
   `pragma_table_info` on SQLite);
3. runs one `alter table <table> add column <column>` for every declared field
   the table lacks, in declared order, all in one transaction, so a table
   either gains all of its new columns or none;
4. runs the object's index statements, last, so an index declared on a field
   that has only just arrived has its column.

`Spec.AlterDDL(dialect, existing)` returns the statements of step 3 without
running anything, given a map of physical table to the column names it holds.
A table missing from the map is skipped, because `DDL` creates it. `Spec.DDL`
is unchanged and still describes a table that does not exist yet. For the AI
`Account` on Postgres, against a table that lacks the five fields:

```sql
alter table <table> add column last_test_prompt text default ''
alter table <table> add column last_test_reply text default ''
alter table <table> add column last_test_ms bigint default 0
alter table <table> add column chat_enabled boolean default false
alter table <table> add column chat_label text default ''
```

The column is the one `create table` uses, with three differences that exist
because the table may already hold rows:

- **A required field takes its type's empty value as a default.** Validation
  refuses a default on a required field, so `create table` gives such a column
  `not null` and no default, which is right for a table that is empty and
  impossible for one that is not: SQLite refuses the alter outright and
  Postgres refuses it as soon as a row exists. The added column is
  `not null default false` (`bool`), `default 0` (`int`, `float`) or
  `default ''` (everything else), so existing rows read the empty value and the
  module still rejects an empty write. A required `json` or `geometry` field
  has no empty value, so it is added **nullable**: existing rows read `NULL`
  and the database does not enforce the requirement on that one column.
- **A `unique` field arrives as a unique index.** SQLite cannot add a unique
  column, so both dialects add the plain column and then
  `create unique index if not exists <table>_<field>_uq on <table> (<field>)`,
  in the same transaction. A non-null unique column added to a table that
  already holds two rows cannot hold, since both read the empty default; the
  transaction rolls back and `Migrate` returns the database's error. Declare
  such a field `nullable`.
- **Never a primary key.** A primary key column cannot be added to a table.
  `AlterDDL` refuses a table that lacks one, with `table <t> exists without
  its primary key column <c>`, because a statement list that quietly omits the
  key would read as complete. `Migrate` does **not** refuse: it adds the
  table's other missing columns, leaves the key alone and returns no error,
  which is what it did before it evolved tables. A module whose key grew
  after its table was made repairs it itself, and `mwanachama-backend-permissions`
  does exactly that, adding `partition` to a grant table from before partitions
  and re-keying it after `Migrate` returns. Refusing would break that module's
  legacy path; whether `Migrate` should one day refuse is a decision for the
  owner, and the one place to change it is the `evolveTable` call to
  `alterObject`.

### A column that stops being declared stops blocking writes (S53)

A `required` field is `not null` with no default, so once it is removed from
the blueprint its orphan column made every later insert fail with a raw driver
error, including inserts that named every column still declared. After the
additions, `Migrate` now reads the table's columns that are `not null`, have no
default, are not part of the primary key and are not declared (the test copy's
`run_id` counts as declared), and relaxes the constraint on each. The data stays.

- **Postgres** runs `alter table <table> alter column "<column>" drop not null`
  per orphan, in one transaction. Identity and generated columns are left alone.
- **SQLite** cannot alter a column's nullability, so the table is rebuilt in one
  transaction: a scratch table with the same columns, types, defaults, primary
  key and `unique` constraints, the rows copied across, the old table dropped,
  the scratch renamed, and the table's explicit indexes re-created from their
  stored SQL. An `autoincrement` marker on a legacy key is not carried over.
- A dropped column that was nullable, or `not null` with a default, is not
  touched, and a second `Migrate` finds nothing to do, so a table is never
  rebuilt twice.
- The same pass reaches the orphan columns of an adopted legacy table, so a
  hand-written `not null` column the blueprint never declared is relaxed too.
  The alternative, dropping the column, was rejected because it destroys data
  on a live database; whether a retired column should one day be dropped is the
  owner's call.
- The Postgres path is unit-tested only as text (`relaxStatements`); this
  repo's Postgres tests skip without `POSTGRES_URL`, and the column-read query
  has not run against a live Postgres.

Limits, all deliberate:

- It is **additive only**. A rename, a type change and a removal are manual:
  a renamed field is a new column plus a copy, an existing column is never
  altered, and a column that stops being declared stays, though no longer
  `not null` (S53, above).
- It looks at column **names** only. A column of the wrong type or nullability
  is not noticed, and an adopted table still keeps its own column types.
- Running it twice is a no-op: the second run finds every column.
- Two `Migrate` calls on the same instance at the same moment can both find a
  column missing, and the second `alter` then fails with "already exists".
  Provision an instance from one place at a time.
- This repo's tests run `Migrate` on SQLite only; the Postgres integration
  tests skip without `POSTGRES_URL`. The Postgres statements are pinned as text
  by `TestAlterDDLNamesTheStatementsForAPostgresAccountThatGainedFiveFields`
  and `TestAlterDDLSnapshotsEveryKindOfAddedColumnOnPostgres`, and the Postgres
  column read mirrors `ColumnIsTextual`'s.

### An adopted table keeps its own column types

That `create table if not exists` has a consequence worth stating plainly: a
table reached through `AdoptLegacy` keeps **every type its old hand-written
SQL gave it**. `Migrate` renames it and creates what is missing; it never
retypes what is there. On such a database the declaration still tells you a
column's name and whether it is nullable, but not its type — and the types do
diverge, because `Migrate` emits `timestamp` as `text` so the tests can run on
SQLite, while the SQL these tables came from used real `timestamptz`.

Anything generating SQL that depends on the type therefore has to **ask the
database, not the declaration**:

- `ColumnIsTextual(db, table, column)` reads the stored type
  (`information_schema.columns`, or `pragma_table_info` on SQLite).
- `DeclaresTextualColumn(o, column, dialect)` is the same answer from the
  declaration, for when there is no database to ask yet.
- `UnsetSQL(column, textual)` builds the "absent" predicate:
  `(c IS NULL OR c = '')` for a text column, because a non-nullable text
  column gets `default ''`, and `(c IS NULL)` for anything else.
- `UnsetClause(db, o, table, column)` is the two composed, falling back to the
  declaration when the database will not answer.
- `specstore.Store.Unset(role, field)` is what a module's query code calls. It
  caches per table and column, so this costs one query per column for the life
  of the store.

`mwanachama-backend-auth` is the worked case, and its
`documentation/2. design/adopted-column-types.md` records what the drift
actually broke there.

### Adoption refuses a column type the engine cannot read (S39)

`AdoptLegacy` is where the drift starts, so on **Postgres** it now checks the
types before it renames anything. For every object whose declared table does
not exist yet and whose legacy table does, each declared field's column is read
from `information_schema.columns` (under its old name, if `Legacy.Columns`
renames it) and compared with what `Migrate` would have declared, by **family**:

| family | held types that count |
| --- | --- |
| text | `text`, `character varying`, `character`, `varchar`, `bpchar` |
| int | `smallint`, `integer`, `bigint` |
| float | `real`, `double precision`, `numeric`, `decimal` |
| bool | `boolean` |
| bytes | `bytea` |
| json | `json`, `jsonb` |

Any other held type is its own family, so a `timestamp with time zone`, a
`uuid` or a `date` under a column declared `timestamp` or `string` is a
mismatch. A mismatch makes `AdoptLegacy` return one error naming every
`table.column is <held type> and the spec declares <field> as <type>`, and
nothing is renamed, in any object, because the check runs for the whole spec
first. The operator retypes the columns by hand (`alter table ... alter column
... type ... using ...`) and adopts again.

- **It refuses; it does not convert.** Coercing rewrites live data on a working
  database and changes the engine's contract for every adopting repo, which is
  the owner's decision and was not taken on 2026-10-01. The refusal is the
  smaller change and fails in the safer direction.
- **Only an adoption in progress is checked.** A table already on its declared
  name is never examined, so a database adopted before this check, such as
  wakala-api's with its `timestamptz` columns, keeps booting; the helpers above
  keep covering it.
- **SQLite is not checked.** It is dynamically typed, so a differing declared
  type is harmless there and a check would only refuse tests that work.
- The comparison is unit-tested as a pure function (`adoptionTypeProblems`);
  the Postgres-backed test is in `postgres_schema_integration_test.go` behind
  the `postgres` tag and `POSTGRES_URL`, and has been compiled but not run.

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

A field tagged **`spec:"-"`** is not a column. It is skipped by all four
struct walks — the construction-time check, `ColumnsOf`, `Encode` and
`Decode` — so it is never written, never read back, and never counted as a
column its object failed to declare. It exists for a value that is genuinely
held somewhere else: read from a join table, derived from an inverse foreign
key, or attached in process while a job runs. `mwanachama-backend-git` is
where this came from, with six — `Commit.ParentIDs` and `Tree.BlobIDs` come
from join tables, `Keyword.ChildIDs` from a query on the parent column, and
`Blob.TreeID` is carried for an API contract and never populated at all.

The tag is not an escape hatch for a field the object forgot. An **untagged**
field nothing declares is still refused by name at construction, which is the
direction that catches a typo and a rename; the tag is a claim that the
module fills this one itself, and a module that tags a real column has simply
stopped storing it.

**Every declared column is written on every write.** A map missing a key
means "leave it alone" to an update, so omitting empty values would make
clearing a field impossible: an entry would keep the `approved_at` that its
document changing is supposed to drop. An empty `json` value is stored as
SQL NULL, so a column never written stays distinguishable from one holding
`""`.

The rest is four wrappers every converted repo would otherwise rewrite:
`Query(ctx, role)` starts a query against the role's table, `Take` reads at
most one row and answers the caller's own not-found sentinel, `List[T]` reads
many, and `Insert` writes one. `List[T]` is uncapped until a store opts in with `CapLists(n)` (a run-scoped
store inherits it). Once it has, a query that carries no `LIMIT` returns at
most `n` rows, `DefaultListCap` (100) being the value to pass (S24: a caller
who named no limit used to get the whole table); a query with its own
`.Limit(k)` keeps it, `.Limit(-1)` opts out for the lookup that must see every
row, and the cap goes on a session of the query so the caller's own
`*gorm.DB` is not left limited. Opt-in is deliberate: `List[T]` also serves internal lookups and sweeps,
and a capped list does not signal that it was cut, so a default cap broke
taskmanager's repeating-task sweep (100 of 502 read) and agency's AG54 and
AG76 fixtures. A host turns the cap on after its internal readers say
`.Limit(-1)`. `NewID` mints a storage key — the row structs
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
2. **Write the domain spec**, pick the instance and mount, and move to
   `<instance>_<module>_<mount>_<table>` with a rename migration — plus the matching
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

## A thing has one identity

`identity/` (S56, 2026-10-11) is the mechanism behind §9 of the standard: a
row id is not an identity. Agency re-mints every uuid on a draft fork and git
mints a new blob row on every content change, so anything keyed on those
forgets the thing on the next ordinary edit. What survives is the owner's own
stable handle — agency's `Code`, git's repository plus path, taskmanager's
uuid — and **each module says which field that is by implementing one
interface**:

```go
type Owner interface {
	StableIdentity(ctx context.Context, object, id string) (string, error)
	Resolve(ctx context.Context, raw string) (object, id string, err error)
}
```

`StableIdentity` answers a **raw** of the form `<module>/<instance>/<identity>`
(`agency/agy1f2e3d4c/R-3`, `git/auth/internal/store.go`). `Resolve` is the way
back: the owner's *current* row id for a raw, so a consumer that only ever
held the constant name can still fetch the thing with the owner's own tools.
The identity rule lives in the owner and nowhere else; a consumer that wrote
"for agency it is Code" into its own files would be naming a domain, and
would be wrong the day agency changed its mind.

The raw's shape is this package's, not the owner's. `Parse` holds the first
two segments to `SegmentPattern` — the same alphabet as `spec.SegmentPattern`,
restated rather than imported so a consumer that only relates things does
not pull in the storage engine, and pinned equal by test — and the identity
to any non-empty UTF-8 with no control character, no format character
(bidi overrides, zero-width characters, the byte-order mark, the soft hyphen),
no line or paragraph separator, and no leading or trailing whitespace. Slashes inside the identity are allowed and the split is on the
first two only, because git's identity is a path. `New` builds one from its
parts under the same checks, and `Raw.String` is the one spelling.

`KeyOf(raw)` is the **constant key**: the full SHA-256 of the raw's bytes as
64 lower-case hex characters (`KeyPattern`). It is the same move `HashName`
and `TableFor` make for a table name, but untruncated — a key is compared,
never spent against Postgres's identifier budget, and
`mwanachama-backend-context` already stores the full digest as a vertex key,
so truncating here would have renamed every vertex. A consumer hashes the
raw and stores the key beside it; the owner never learns who asked.

Three sentinels carry the contract's answers, and an owner wraps them so
`errors.Is` reaches them through whatever it adds:

- `ErrInvalidRaw` — the string is not `<module>/<instance>/<identity>` or a
  segment is off the alphabet. `Parse` and `New` return it; an owner's
  `Resolve` returns it unchanged for a malformed raw.
- `ErrNotFound` — the row behind `(object, id)` or behind the raw is gone.
  From `Resolve` this is the **confirmed miss** a consumer's staleness
  handling keys on (context's #40, #56): a thing the owner itself no longer
  has.
- `ErrNotOwned` — the raw names another module or another instance. This is
  not a miss: the thing may well exist, under an owner the caller should
  have asked instead. Folding it into `ErrNotFound` would mark a live thing
  stale for having been asked of the wrong owner.

What this package does not do: it holds no registry of owners and resolves
nothing itself. Which owner answers for which `<module>` is the host's
wiring, and which arguments of an operation carry a raw at all is the
operations file's to declare: `refers` on `dispatch.Arg` (S57, 2026-10-12), enumerated by
`Spec.References()` — see [dispatcher.md](dispatcher.md#binding).

### A host says what it wrote

A module that reacts to another module's writes cannot watch them; the host
ran the import, the publish, the fork, and knows. `identity.Written` (S59,
2026-10-12) is the one message it sends after such a write, and
`identity.Reactor` is the seam a reacting module accepts it on:

```go
type Written struct {
	Partition string          `json:"partition"`
	Owner     string          `json:"owner"`
	Objects   []WrittenObject `json:"objects"`
}
type WrittenObject struct {
	Object string `json:"object"`
	ID     string `json:"id"`
}
type Reactor interface {
	Wrote(ctx context.Context, w Written) error
}
```

It is one in-process call after the write, not a bus or a queue: the host
decides when (and whether to batch), the module touches exactly the objects
named. Four choices are worth knowing:

- **An object is `(object, id)`, the pair `StableIdentity` takes**, not a
  raw. The host holds row ids at the moment it writes; a reacting module that
  wants the constant name asks the owner, so the identity rule stays with the
  owner. `WrittenObject.Refers(owner)` spells the pair as the
  `<module>.<object>` a `refers` mark names, and `ObjectPattern` is pinned to
  the object half of `dispatch.RefersPattern` by test, so a reported object
  and a marked argument always name things the same way.
- **There is no whole-owner form.** A report with no objects is refused
  (`ErrInvalidWritten`) rather than read as "everything": an empty list
  reaching a reactor by mistake would otherwise sweep a partition, which is
  exactly the load the rule exists to prevent. A new Instance or an import,
  where "what changed" is the whole owner, names every object it created —
  context's #42 says that needs no separate rule, and this keeps it so.
- **One owner per report.** An event that touches two owners sends two, the
  way context's #32 scopes a reindex to the owner an event changed.
- **`Check` holds the shape**: the partition and the owner to
  `SegmentPattern` (a partition is the tenant segment a raw's instance
  names), each object to `ObjectPattern`, each id to the same rule a raw's
  identity follows — non-empty UTF-8, no control, format, line or paragraph
  separator character, no edge whitespace, slashes allowed. A reactor calls it first; nothing here
  resolves or reads the objects.

What it does not do: it names no reacting module and no event. Which events a
host reports, to which reactors, is the host's wiring — for
`mwanachama-wakala-api` that is context's TR13 — and what a reactor does with
the objects (read them from the owner, emit facts, re-score) is its own.

## A domain attaches another module

A domain spec may say that **another module works on one of its objects**,
and the host mounts that module beside it. Ratings attached to a catalog is
the first case (ratings #23): every entry of the catalog is something a
caller can rate, and the catalog never learns that ratings exists.
`spec.Spec.Attach` (2026-10-12) carries it:

```json
{
  "module": "catalog",
  "domain": "agency",
  "instance": "wakala",
  "objects": [ … ],
  "attach": [
    { "module": "ratings", "items": "catalog.entry" }
  ]
}
```

- **`module`** is the module attached, in `SegmentPattern` — the same
  alphabet as a raw's first segment, so it names the module the way a key
  does.
- **`items`** is the object of *this* spec whose keys the attached module
  works on, spelled `<module>.<role>` — `AttachTargetPattern`, pinned equal
  to `dispatch.RefersPattern` by test, so an attachment and a `refers` mark
  name another module's object the same way. Its module half must be this
  spec's own module (an attachment points at this domain's objects, never
  a third module's), and its role half must be a role an object here
  claims. **A role, not the object's name**: the name is the domain's word
  (`agency`) and the role is the module's (`entry`), and the attached
  module's keys are identified through the owner's `StableIdentity`, which
  speaks the module's vocabulary.
- That is all it carries. Where the attached module's tables land, what its
  domain calls them and how it is tuned stay in **that** module's own spec
  and settings; the attachment says only *what* and *on which objects*.
  A module cannot attach itself, a pair cannot be attached twice, and an
  unknown field in an attachment is refused like anywhere else in a spec.
- **The spec engine reads and validates it; the host acts on it.** `Validate`
  holds the shape; `s.Attached(module)` lists the attachments naming one
  module and `Attachment.ItemsRole()` gives the role to resolve with
  `ByRole`. Mounting — which spec the attached module loads, at which path,
  behind which gate — is the host's, exactly as every other mount is
  (wakala-api's decision #22). For ratings that is its RT11.
- **Additive.** A spec with no `attach` decodes, validates, migrates and
  marshals exactly as before (`attach` is `omitempty`), and a blueprint
  cannot carry one: attaching is a deployment's choice, not a module's.

Before this, no declaration anywhere said "module X works on object Y of
module Z" — every such placement (taskmanager per agency, catalog mounted
twice) was decided in the host's Go code. Reach for `attach` when the
placement follows from the domain's own data; keep a host-code mount when it
follows from the host's own structure, as taskmanager-per-Instance does.

## A module declares its settings

A module's blueprint may name the settings it acts on (S64, member app
decisions 280 and 281, 2026-10-22). Each module stores and checks its own
settings; the engine supplies the declaration, the check and the layering, so
no module writes them again.

```json
"settings": [
  {"name": "approval", "type": "bool", "description": "Whether an answer needs approval.", "default": false},
  {"name": "language", "type": "enum", "description": "Language forms are written in.",
   "values": ["en", "sw"], "default": "en"}
]
```

- **The blueprint owns them, never the domain spec.** `Blueprint.Settings` is
  validated in `Blueprint.Validate`; `Apply` copies it onto the loaded
  `Spec.Settings`. A domain spec carrying a `settings` key is refused as an
  unknown field, because a setting's value belongs to the agency's
  definition and its owner, not to the file that names tables.
- **A setting is held to its shape at load.** Its name is in `NamePattern`,
  its type is one of `string`, `text`, `int`, `float`, `bool`, `enum` or
  `json`, its description is not empty, an enum carries `values` and nothing
  else does, and it **must carry a default that fits the shape**. The
  default is the module's own layer, the one every other layer falls back
  to, so a setting with none would resolve to nothing.
- **`CheckSettings(settings, values)`** (and `Spec.CheckSettings`) answers
  every problem with a set of values, sorted by name: a value off its shape,
  outside an enum's values, a fraction for an `int`, or a name the module
  does not declare. Publishing and an owner's change both go through it.
- **Three layers, one answer.** `Resolve(settings, definition, overrides,
  name)` answers an `Effective`: the value and the `Layer` that set it —
  `override` (the owner's live change, carrying `By` and `At`), else
  `definition` (the value the agency definition handed the module on
  publish), else `module` (the declared default). `ResolveAll` answers every
  declared setting in declared order.
- **A layer whose value no longer fits is passed over, not served.** A
  module that narrows an enum after an owner chose a value it dropped keeps
  answering a valid value, and the layer reported says where it came from.
  Reset to definition is deleting the override.

- **An owner's change is one step that yields its own log entry.**
  `SetOverride(settings, overrides, name, value, by, at)` refuses an
  undeclared name, a value off its shape (so an owner can never store what
  `Resolve` would pass over) and a change missing who or when
  (`ErrSettingChangeUnattributed`); it answers a new override map — the
  caller's is never edited — and a `SettingChange` (`name`, `from`, `to`,
  `by`, `at`). `ClearOverride` is reset to definition: it answers the map
  without the override and a change marked `cleared`, or `false` when there
  was nothing to clear. The `SettingChange` is the row decision 281's
  override log appends, whatever stores it.

### The engine stores them (decision 398, 2026-10-24)

The engine provisions every module's settings storage. When a spec carries
settings, `spec.Migrate` creates three more tables **inside that module's own
table set for that instance**, named exactly as an object's are
(`<instance>_<mount hash>_<hash of <module>_<table>>`) and recorded in the
instance's `spec_table_names` registry beside the module's objects. A spec
with no settings gets none of them. `Spec.SettingsObjects()` declares them and
`Spec.SettingsTables()` names them.

| Table | Holds |
| --- | --- |
| `spec_settings_definition` | `name`, `value`: the definition's values |
| `spec_settings_override` | `name`, `value`, `set_by`, `set_at`: the owner's overrides |
| `spec_settings_change` | `id`, `name`, `from_value`, `to_value`, `cleared`, `changed_by`, `changed_at`: every override set or cleared, ordered by `changed_at, id` |

Every value is a JSON document in a text column, so a value reads back as JSON
decodes it (a whole number as a float, which `CheckSettings` accepts for an
`int`). The `spec_settings_` prefix is reserved: a module object whose table
starts with it is refused at load, since it would land on the same hashed
name.

- **`PutDefinitionSettings(db, spec, values)`** replaces the whole definition
  layer with `values` after `CheckSettings`; anything off its shape, or not
  declared, stores nothing.
- **`LoadSettingLayers(db, spec)`** answers the definition and override maps
  `Resolve` takes; **`ResolveStored`** and **`ResolveAllStored`** do both
  steps and report the layer.
- **`StoreOverride(db, spec, name, value, by, at)`** runs `SetOverride` over
  the stored overrides and, in one transaction, writes the override and
  appends its `SettingChange` row. **`StoreClearOverride`** does the same for
  `ClearOverride`, and writes nothing when there was nothing to clear.
  **`SettingChanges`** reads the log back in order.
- Calling any of these for a module that declares no settings is an error.

**Still open, deliberately not built:** what an override that no longer fits
after a republish (a narrowed enum, a changed type) does — kept silently and
passed over by `Resolve`, or reported as a conflict. Today the stored row is
left as it was and `Resolve` passes over it, which is the existing layering
rule, not a policy; that question waits on the owner.

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
- **Reconciling an adopted table's column types with the declaration.** The
  predicate helpers above work around the drift; nothing closes it. Having
  `AdoptLegacy` alter each adopted column to its declared type would, but that
  rewrites live data and changes this engine's contract for every adopting
  repo, so it is a decision of its own.
