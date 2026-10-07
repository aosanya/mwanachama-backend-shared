# Implementation

Where work on `mwanachama-backend-shared` is tracked. Two files carry the board:

- [todo.md](todo.md) — **open rows only**, ids in the `S` prefix, status one of
  🚀 In Progress · 📋 Not Started · ⏸️ Blocked. No prose and no finished rows.
- [todo_done.md](todo_done.md) — every finished row, and the board context that
  used to sit above the open list. A finished id is never reused: scan this
  file before minting the next one.
- [coding_sessions/](coding_sessions/) — dated write-ups of individual sessions, one file each.

## Rows that arrive with a failing test

Two scheduled routines file rows here, and each files a **deliberately red**
test beside its row, in this repo's ordinary suite:

- `Test<ID>_OpenHole_<slug>` — a security hole the security sweep reproduced
  (`🐞 **SECURITY — …**` rows, with Did · Observed · Threat · Control · Verdict).
- `Test<ID>_OpenBug_<slug>` — a defect the integration sweep reproduced
  (`🐞 **BUG — …**` rows).

The test asserts what the code *should* do, so the fix turns it green; it is
never edited to pass. Each red is listed under `Red by design` in
`mwanachama-developer/documentation/4. qa/security-sweeps/` or
`…/integration-sweeps/`, so a red named there is a known open row, and a red
named in neither is new. Clearing one — fix the code, move the row to
`todo_done.md`, rename the test to the guard it has become — is the
`fix-red-tests` skill's job.

## Verifying a change

`go test ./...`. In-memory SQLite across `spec/`, `specstore/`, `dispatch/`, `httpwire/` and the entity-graph packages.
