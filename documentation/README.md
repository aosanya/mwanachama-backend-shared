# mwanachama-go-shared — documentation

## Layout

Four folders, in SDLC order, and everything lives under one of them.

| Folder | What's inside |
|--------|---------------|
| [1. requirements/](1.%20requirements/) | Problem, vision and scope for this shared library. |
| [2. design/](2.%20design/) | The Postgres entity-graph design: schema, the `DataManager`/`SchemaManager` contract, traversal-query approach. |
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
[mwanachama-git](../mwanachama-git) and
[mwanachama-taskmanager](../mwanachama-taskmanager) so both can port their
CodeValdGit/CodeValdWork business logic against the same storage contract
without duplicating the entity-graph engine in each.

Standalone by design — no dependency on `CodeValdSharedLib` (unpublished,
private) and no gRPC/proto/sub-service shape. Plain Go packages, imported
directly by `mwanachama-git`, `mwanachama-taskmanager`, and ultimately
[mwanachama-api-gateway](../mwanachama-api-gateway).
