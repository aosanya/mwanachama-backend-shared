# mwanachama-go-shared (Go)

Open tasks only — 🚀 In Progress · 📋 Not Started · ⏸️ Blocked.
Everything else (completed rows, board context) is in [todo_done.md](todo_done.md).

| Task | Title | Status | Depends on |
|------|-------|--------|------------|
| S3 | Postgres migrations: `entities` (`type_id`, `jsonb properties`, soft-delete cols, per-type unique indexes), `relationships` (`from_id`/`to_id`/`name`/`jsonb properties`), schema draft/published tables | 📋 | S2 |
| S4 | Implement Postgres `DataManager`: entity CRUD/upsert/list, relationship CRUD/list, `TraverseGraph` via recursive CTE (direction + depth-limited) | 📋 | S3 |
| S5 | Implement Postgres `SchemaManager`: set/get draft, publish (snapshot+validate), activate (exactly-one-active, real transaction), list/get versions | 📋 | S3 |
| S6 | Postgres connection/pool helper (`pgxpool` wrapper — replaces `arangoutil.Connect`) | 📋 | S1 |
| S7 | Local `Publisher` interface (`Publish(ctx, topic string, payload any) error`) + a trivial in-process implementation callers can use as a default | 📋 | S1 |
| S8 | Unit tests + Postgres integration tests (`make test` / `make test-pg`) | 📋 | S4, S5 |
| S9 | Tag `v0.1.0` once `mwanachama-git`/`mwanachama-taskmanager` are ready to consume it | 📋 | S8 |
