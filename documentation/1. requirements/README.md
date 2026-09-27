# Requirements

`mwanachama-backend-shared` has grown three unrelated roles since bootstrap
(2026-09-02), none of which shares code with the others. This page is the
orientation for all three; a full decision record for any one of them is
this repo's `CLAUDE.md`, `documentation/2. design/`, and
`documentation/3. implementation/todo_done.md`, not a separate requirements
document per role.

## Role 1 — the Postgres entity-graph engine (`entitygraph/`, `schema/`,
`memory/`, `postgres/`, `gormtest/`, `gormutil/`, `orgpolicy/`, `orgsettings/`)

Ported from `CodeValdSharedLib/entitygraph`, an ArangoDB-backed generic
entity/relationship graph store, and retargeted to Postgres: one `entities`
table (`type_id` + `jsonb properties`) rather than one collection per type,
one `relationships` table for edges, no `CodeValdSharedLib` dependency and no
gRPC/proto shape. It exists so `mwanachama-backend-git` and
`mwanachama-backend-taskmanager` can port their CodeValdGit/CodeValdWork
business logic against one storage contract (`DataManager`/`SchemaManager`)
without each repo reimplementing the engine. See CLAUDE.md's "Key invariants"
for what a Postgres port of this had to fix rather than carry forward
(`SchemaManager.Activate` as a real transaction, vertex uniqueness as a real
index, not a pre-check-then-insert race).

## Role 2 — `mcpui/`, a shared MCP-dashboard design system

Added 2026-09-11 so every backend repo's hand-rolled MCP Apps (SEP-1865)
dashboard View shares one CSS palette (`mwanachama-wakala-studio`'s look —
IBM Plex Sans/Mono + Big Shoulders Display) instead of each repo copying and
drifting from its own snapshot. `mwanachama-backend-agency/mcp/dashboard_ui.go`
is the first, reference consumer (that repo's CLAUDE.md, AG17, has the full
record). Nothing here to specify further — it is a stylesheet plus its
`//go:embed`, versioned like any other file in this repo.

## Role 3 — `spec/`, `specstore/`, `dispatch/`: the declared-domain engine

The newest and now the most active role. Moved here from
`mwanachama-backend-catalog` on 2026-09-24 (S18) so
`mwanachama-backend-agency` (AGD-007) and `mwanachama-backend-permissions`
(built 2026-09-27 directly on this shape) could build on the same engine
rather than each repo growing its own loader. The problem it answers: a
module such as catalog, agency or permissions used to declare its storage as
hand-written Go row structs and its HTTP/MCP surface as hand-written
handlers — both of which drift from what the domain actually needs the
moment a domain adds a field the module's author didn't anticipate. Instead:

- a **module** ships a blueprint (`spec.Blueprint`/`ParseSpec`) declaring the
  objects it stores, their fields and indexes, once;
- a **domain** ships a spec naming which object fills each declared role,
  where it lands, and its own indexes — never re-declaring a field the
  blueprint already owns (see `documentation/2. design/declared-domains.md`);
- `spec.Migrate` emits the DDL directly from that declaration — there is no
  `AutoMigrate` and no row structs;
- `dispatch/` does the same thing for the HTTP route table and MCP tool
  surface: a module's `*.operations.json` declares one address, one manager
  method and one authorization action per operation, and `dispatch.Dispatch`/
  `dispatch.Tools` builds the real `[]Route`/`[]Tool` from it, checked against
  the manager's actual method signatures at build time (`checkSignature`).

The org-wide strategy this serves is
[`mwanachama-developer`'s architecture-spec-driven-modules.md](../../../mwanachama-developer/documentation/2.%20design/architecture-spec-driven-modules.md);
this repo's own reference is `documentation/2. design/declared-domains.md`
(the storage half) and `documentation/2. design/dispatcher.md` (the
operations half). `mwanachama-backend-catalog` is the first consumer,
`mwanachama-backend-agency` the second, `mwanachama-backend-permissions` the
third.

## What this repo refuses to know

Nothing in `spec/`, `specstore/` or `dispatch/` knows what any domain's
fields mean — a table name, a field name and an index are opaque strings
compared for equality and validated against a strict identifier alphabet
(`spec.NamePattern`, `spec.SegmentPattern`) because they reach SQL as text
rather than as bound parameters. That is a property of the engine, not of
any one consumer; `mwanachama-backend-catalog` and
`mwanachama-backend-permissions` each carry their own
`domain_agnostic_test.go` proving *their* code names no domain, and this
repo's own `spec/testdata/` deliberately holds domains no consumer ships —
`clinic.record.json` and `garage.record.json` — precisely so the engine's
own tests exercise a shape its authors never anticipated, rather than only
the shipped examples every consumer already agrees on.
