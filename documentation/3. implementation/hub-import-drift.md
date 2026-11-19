# Hub import drift

The hub keeps an in-tree copy of this module at `projects/hub-steward/code/wakala-api/modules/shared`. The plan is that the hub stops carrying it and imports this module at a tagged version instead. Before it can, everything the hub copy does that this repository does not has to move here. [hub-copy-diff.md](hub-copy-diff.md) describes the comparison script; this page is the result of running it on 2026-11-19 against the hub's working tree (branch `dev`, shared folder with no uncommitted changes of its own) and the disposition of every difference.

## Command

```
scripts/hub-copy-diff.sh
```

The exit code is 1 when the trees differ. Imports are rewritten from `wakalaserver/modules/shared/` to `github.com/aosanya/mwanachama-backend-shared/`, comments and layout are dropped before comparing, and `mcpui/theme.css` (not Go) was compared separately with `cmp` and is identical.

## What differs

Ten packages carry no difference at all: `consent`, `disclosure`, `events`, `geo`, `gormutil`, `httpwire`, `identity`, `mcpui`, `spec` and `specstore` are identical after the import rewrite.

| Side | Where | Difference | Proposed disposition |
|---|---|---|---|
| Hub only | `answerrules/` (package and its test) | One home for the time-range rule (`HH:MM` start before end, the "from X to Y" words) and the shape of a place reference (`address:` or `street:` followed by text). Hub commit fed51ac0. | Moved as is, with its test. |
| Hub only | `versioncheck/` (package and its test) | The stale-version check the edit screens' update tools use: a version stamp that is not in the same second as the stored update time refuses, naming the first changed field. Hub commit 15179245. | Moved as is, with its test. |
| Hub only | `dispatch/busy.go` | A call that fails with a lock or serialisation message (`database is locked`, `SQLITE_BUSY`, `could not serialize access`, `deadlock detected`) answers 409 "another write got there first, try again" instead of 500. Hub commit 09e613ca. | Moved; test added (`dispatch/hub_import_test.go`). |
| Hub only | `dispatch/dispatch.go`, `dispatch/tools.go` | The two places that call the handler check `isBusy` (HTTP route answers 409, tool call returns a plain error). Hub commit 09e613ca. | Moved with `busy.go`. |
| Hub only | `dispatch/issue_alias.go`, `dispatch/tools.go` | A tool that takes `taskID` and not `id` accepts `id` for it; both with different values is refused; the refusal for an unknown argument names the alias. Hub commit 7ef8764c. | Moved; test added. |
| Hub only | `dispatch/spec.go`, `dispatch/bind.go` | `Arg.Optional`: an empty request body is allowed on a whole-body argument declared optional. Hub commit 47472669. | Moved; test added. |
| Hub only | `dispatch/bind_cover_test.go` | Hub commit 56bea9bb pins argument binding (snake_case column keys, query minimums, whole-body and whole-query refusals, variadic spreading, caller and path overwrite of body claims). | Moved with the code. It passes against this repository's `dispatch` as it stands. |
| Hub only | `recur/rule.go` | An `Interval` above the largest that can reach year 9999 yields no occurrence; day and week counts between dates use Unix seconds instead of `Duration.Hours`. Hub commit 09e613ca. | Moved; tests added. |
| Conflict | `recur/rule.go` `Validate` | The hub refuses a rule whose `BYMONTHDAY` names a day none of its `BYMONTH` months has (`FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30` is `ErrInvalidRule`). This repository accepts it and `Next` gives up on it, and `TestNextGivesUpOnARuleThatNeverMatches` pins that. Hub commit 09e613ca. | Not chosen: this repository's behaviour stays and the hub's check is not moved. Owner question 1. |
| Hub only | `recur/schedule.go` | The walk stops after year 9999; `Period` looks back through growing windows (3, 45, 800 and 36500 days) before walking from the start, so a late query on an old schedule is not a walk over every occurrence since the start. Hub commit 09e613ca. | Moved; tests added. |
| Differs | `plaintext/unstorable.go` | Hub reads the refusal's SQLSTATE from `*pgconn.PgError`. This repository reads it through the interface `interface { error; SQLState() string }`, so `dispatch` and the git path do not import the Postgres driver. The codes tested (22021, 22P05) are the same. | Keep this repository's version. Not a conflict: behaviour is the same, the import differs on purpose. The hub copy is deleted by the import, so nothing moves back. |
| Shared only | 66 test files of `dispatch`, `geo`, `gormutil`, `httpwire`, `identity`, `recur`, `spec` and `specstore` | The hub copy was made without most test files. | Keep. They test code that is identical on both sides. |
| Shared only | `dispatch/required_query_test.go`, `gormutil/unique_violation_test.go`, `specstore/in_transaction_test.go`, `plaintext/unstorable_sqlstate_test.go` | Red tests written here for hub changes that shipped with no test of their own. | Keep. |
| Shared only | `entitygraph`, `entitygraphtest`, `gormtest`, `memory`, `orgpolicy`, `orgsettings`, `postgres`, `schema`, `vocab` | Packages the hub never copied. | Keep. The hub does not import them. |

The first run printed 70 `Only in backend` lines, all test files (66 plus the four named). After the move it prints 72: the two files written for the moved code, `dispatch/hub_import_test.go` and `recur/hub_import_test.go`, which the hub does not carry.

## Conflicts

One. A rule that can never occur is accepted here and refused by the hub (the `recur/rule.go` `Validate` row above). The other differences are additions on top of otherwise identical code, or the `plaintext` import difference that gives the same answer.

### Owner question 1

Should `recur.Rule.Validate` refuse a rule whose `BYMONTHDAY` names a day that none of its `BYMONTH` months has? The hub refuses it when a rule is saved, so a person sees the mistake at once. This repository accepts it and the schedule simply never produces a date. Importing the hub onto this module without the check means a saved rule of that kind is no longer refused. Until the owner chooses, `Validate` is as this repository had it and `TestNextGivesUpOnARuleThatNeverMatches` still holds.

## Result

After the move, `scripts/hub-copy-diff.sh` shows `plaintext/unstorable.go` (on purpose) and the `recur/rule.go` `Validate` check (owner question 1) as the only differences, plus the `Only in backend` test files. `go test ./...` passes. No pinned count changed: the union needed none.

## go.mod

`go.mod` declares `module github.com/aosanya/mwanachama-backend-shared`, `go 1.25.0` (the hub's `go.mod` says the same), and has no `replace`. The repository's own requirements are at or below the hub's (`pgx` 5.10.0 against 5.11.0, `gorm.io/driver/postgres` 1.6.2 against 1.6.3, `golang.org/x/text` 0.29.0 against 0.31.0), so the hub's own `go.mod` decides the versions that build. The `v0.0.0` plus `replace ../mwanachama-backend-shared` lines are in the consuming repositories' `go.mod` files, not here; each consumer drops them when it moves to a tag.

## Tags

The remote already carries `v0.1.0` (44eaf29) and `v0.2.0` (4f8d35d). The release number for the import is for the owner to decide.
