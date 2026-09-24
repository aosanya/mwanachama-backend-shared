# Design

Postgres entity-graph design lands here: schema (`entities`/`relationships`
tables), the `DataManager`/`SchemaManager` contract, and the recursive-CTE
traversal approach that replaces AQL graph traversal. Empty at bootstrap —
populate alongside S2–S5.

[declared-domains.md](declared-domains.md) — the reference for `spec/` and
`specstore/`: what a module's blueprint declares, what a domain's spec adds,
how a physical name is built and why it is validated rather than escaped, what
`Migrate` emits, how the store joins a declared column to a Go field, and the
order a repo is converted in. Its sibling is [dispatcher.md](dispatcher.md),
which is the same idea applied to the route table.

Also here: [httpwire.md](httpwire.md) — the contract of the `httpwire/`
package: what it deliberately leaves to the caller (handlers, error-to-status
tables, auth), the `Route` shape, and what `Route.Action` means.

[gorm-shared-infrastructure.md](gorm-shared-infrastructure.md) —
an audit of the GORM-era infrastructure every `mwanachama-backend-*` repo
that migrated off entitygraph (`actor`, `assetmanager`, `git`, `comm`,
`forms`) independently rebuilt, and a proposed package layout for what this
repo should absorb.
