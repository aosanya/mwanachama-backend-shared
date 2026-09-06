# mwanachama-backend-shared (Go)

Open tasks only — 🚀 In Progress · 📋 Not Started · ⏸️ Blocked.
Everything else (completed rows, board context) is in [todo_done.md](todo_done.md).

| Task | Title | Status | Depends on |
|------|-------|--------|------------|
| S9 | Tag `v0.1.0` once `mwanachama-backend-git`/`mwanachama-backend-taskmanager` are ready to consume it | 📋 | S8 |
| S11 | Decide nullable-pointer vs. empty-string-sentinel as the org convention (actor/assetmanager/comm/git split 3-vs-1, forms avoids both) | 📋 | S10 |
| S12 | Extract `gormtest/`: `OpenSQLiteDB(t)`, `OpenPostgresDB(t, prefix)` wrapping `postgres.Open` | 📋 | S10 |
| S13 | Extract `httpwire/`: `WriteJSON`/`WriteErr`/`ReadJSON`, `Route`/`Routes`/`Pattern`, `StatusFor` | 📋 | S10 |
| S14 | Extract `gormutil/`: `ClassifyError` (from comm's `classify`), dialect-aware constraint-sync helper, `SetUUIDIfEmpty` | 📋 | S11 |
| S15 | Extract `vocab/`: generic closed-membership set + transition-table helper | 📋 | S10 |
| S16 | First adopter pass: swap `mwanachama-backend-assetmanager`'s local copies for `gormtest`/`httpwire` imports, to validate the extraction before larger repos adopt it | 📋 | S12, S13 |
