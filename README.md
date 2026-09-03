# mwanachama-backend-shared

Shared Postgres entity-graph engine for the mwanachama Go libraries.

Provides a storage-agnostic `Entity`/`Relationship`/`DataManager`/
`SchemaManager` contract (ported from `CodeValdSharedLib/entitygraph`, minus
the ArangoDB coupling) plus a Postgres implementation of it, and a minimal
local `Publisher` interface for domain events. No gRPC, no sub-service shape
— a plain Go module.

Consumed by [mwanachama-backend-git](../mwanachama-backend-git) and
[mwanachama-backend-taskmanager](../mwanachama-backend-taskmanager).

See [documentation/](documentation/) for design and task board.
