# mwanachama-backend-shared

The Go module every mwanachama backend repo builds on. Module path `github.com/aosanya/mwanachama-backend-shared`; a plain Go module with no gRPC and no sub-service shape, resolved by each consumer through a `replace` line to the sibling checkout.

| Package | What it is |
|---|---|
| `spec` | The declared domain: a blueprint of roles and fields, a domain spec naming them, validation, and `Migrate`, which emits the DDL |
| `specstore` | Reads and writes the rows of a declared domain through GORM, one `Store` per spec |
| `dispatch` | The operations half: the route table and the MCP tools built from a module's `*.operations.json` |
| `identity` | A stable name for a row that survives versions (`Owner`, `Parse`, `New`, `Written`, `Reactor`) |
| `httpwire` | The JSON wire helpers a route table is served with |
| `recur` | Repeating schedules as iCalendar RRULEs |
| `geo` | Points, polygons, distance and bounding boxes for geometry fields |
| `vocab` | The closed-membership-set-plus-transition-table idiom |
| `mcpui` | The shared stylesheet for hand-rolled MCP Apps dashboard views |
| `events` | The trivial publish contract for domain events |
| `gormutil`, `gormtest` | GORM helpers and the sqlite and Postgres test-database harness |
| `orgsettings`, `orgpolicy` | Organization-wide branding and policy numbers |
| `entitygraph`, `postgres`, `memory`, `schema`, `entitygraphtest` | The original Postgres entity-graph engine (`DataManager`, `SchemaManager`). No module outside this one imports `entitygraph` any more; `postgres.Open` still opens the database in the integration tests of git, taskmanager, comm, forms and actor |

The reference for the engine is [documentation/2. design/declared-domains.md](documentation/2.%20design/declared-domains.md) for `spec` and `specstore` and [dispatcher.md](documentation/2.%20design/dispatcher.md) for `dispatch`. [mwanachama-backend-catalog](../mwanachama-backend-catalog) is the module whose shape the others follow.

See [documentation/](documentation/) for design and the task board, and [CLAUDE.md](CLAUDE.md) for the invariants.
