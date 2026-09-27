# mwanachama-backend-shared — documentation

## Layout

Four folders, in SDLC order, and everything lives under one of them.

| Folder | What's inside |
|--------|---------------|
| [1. requirements/](1.%20requirements/) | Problem, vision and scope for this repo's three roles: the Postgres entity-graph engine, `mcpui/`'s shared dashboard styling, and the `spec`/`specstore`/`dispatch` declared-domain engine. |
| [2. design/](2.%20design/) | The Postgres entity-graph design (schema, the `DataManager`/`SchemaManager` contract, traversal-query approach) alongside the declared-domain reference: [declared-domains.md](2.%20design/declared-domains.md), [dispatcher.md](2.%20design/dispatcher.md), [httpwire.md](2.%20design/httpwire.md). |
| [3. implementation/](3.%20implementation/) | The work: `todo.md` (open board), `todo_done.md` (completed rows + board context). |
| [4. qa/](4.%20qa/) | Test coverage and results. |

## Boards and status

| File | What it holds |
| --- | --- |
| [todo.md](3.%20implementation/todo.md) | Open task board |
| [todo_done.md](3.%20implementation/todo_done.md) | Completed rows + board context |

## What this repo is

The Postgres-backed replacement for `CodeValdSharedLib/entitygraph` — a
generic, schema-driven entity/relationship graph store (entities with typed
`jsonb` properties, labeled directed edges, recursive-CTE traversal) plus a
minimal local event-`Publisher` contract. Consumed by
[mwanachama-backend-git](../mwanachama-backend-git) and
[mwanachama-backend-taskmanager](../mwanachama-backend-taskmanager) so both can port their
CodeValdGit/CodeValdWork business logic against the same storage contract
without duplicating the entity-graph engine in each.

Standalone by design — no dependency on `CodeValdSharedLib` (unpublished,
private) and no gRPC/proto/sub-service shape. Plain Go packages, imported
directly by `mwanachama-backend-git`, `mwanachama-backend-taskmanager`, and ultimately
[mwanachama-backend-api-gateway](../mwanachama-backend-api-gateway).

Two more, unrelated roles have grown on top since: `mcpui/`, the CSS every
backend repo's own MCP Apps dashboard pulls from so they share one visual
design system, and `spec`/`specstore`/`dispatch`, the declared-domain engine
`mwanachama-backend-catalog`, `mwanachama-backend-agency` and
`mwanachama-backend-permissions` build their storage and route/MCP-tool
tables on. See [1. requirements/README.md](1.%20requirements/README.md) for
all three.
