# Design

Postgres entity-graph design lands here: schema (`entities`/`relationships`
tables), the `DataManager`/`SchemaManager` contract, and the recursive-CTE
traversal approach that replaces AQL graph traversal. Empty at bootstrap —
populate alongside S2–S5.

Also here: [gorm-shared-infrastructure.md](gorm-shared-infrastructure.md) —
an audit of the GORM-era infrastructure every `mwanachama-backend-*` repo
that migrated off entitygraph (`actor`, `assetmanager`, `git`, `comm`,
`forms`) independently rebuilt, and a proposed package layout for what this
repo should absorb.
