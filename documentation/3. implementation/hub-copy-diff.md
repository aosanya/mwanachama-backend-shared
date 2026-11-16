# Hub copy diff

`mwanachama-backend-shared` is the custodian of the shared module. The hub (`projects/hub-steward/code/wakala-api/modules/shared`) holds a copy made on 2026-11-04 and changed it afterwards. Every change made in that copy exists here, so the two differ by the import path only (hub issue S65, the shared half of ISS-105).

## Command

```
scripts/hub-copy-diff.sh [path-to-hub-modules-shared]
```

The default path is `../projects/hub-steward/code/wakala-api/modules/shared`, relative to this repository's parent folder. The exit code is 1 when the trees differ, 0 when they do not. The script and `scripts/hubdiff` are the ones of `mwanachama-backend-git` (G78), adapted to compare package folders.

## What it does

1. Copies the `*.go` files of the hub folder, and the same packages from this repository, into two temporary folders. Packages that exist only here (`entitygraph`, `entitygraphtest`, `gormtest`, `memory`, `orgpolicy`, `orgsettings`, `postgres`, `schema`, `vocab`) are left out: the hub never imported them.
2. Rewrites the hub's imports: `wakalaserver/modules/shared/` becomes `github.com/aosanya/mwanachama-backend-shared/`.
3. Runs every Go file on both sides through `scripts/hubdiff`, which parses it, drops all comments and prints it with `gofmt`, so comments and layout do not show as differences.
4. Runs `diff -r backend hub`.

Files that are not Go (`mcpui/theme.css`) are not compared.

## Result of 2026-11-16

Every non-test Go file is identical after the rewrite, and every test file of the hub exists here and is identical. Nothing in the output is `diff` or `Only in hub`. What remains is `Only in backend` lines, all test files the hub copy never carried or that were written here:

| Difference | Reason |
|---|---|
| Only here: the tests of `dispatch`, `geo`, `gormutil`, `httpwire`, `identity`, `recur`, `spec` and `specstore` (65 files) | The hub copy of 2026-11-04 was made without test files. The shared repository kept them. They test code that is identical in both. |
| Only here: `dispatch/required_query_test.go` | The red test written for hub 31c46340 (a missing required query argument answers 400); the hub commit had none. |
| Only here: `gormutil/unique_violation_test.go` | The red test written for hub 7d1530a9 (`IsUniqueViolation`); the hub commit tested it only through the API. |
| Only here: `specstore/in_transaction_test.go` | The red test written for hub eaa9e35e (`InTransaction`); the hub commit tested it only through the API. |

## The hub commits carried over

One commit here per hub commit, each naming the hub hash: 1ccd0d50, 931ae1d6, ec77c144, 31c46340, 92c070dd, 27df7d13, 4b25c600, a4ef4f1a, 7d1530a9, 4d3c768e, a31eeca9, b168c25f, eaa9e35e, c401434b, cd42c031, 7b0b14d6, e97a230a, 57265cc6, c702fd78, 0440533c, 05e3df42.

Files a commit touched lost their comments (the repository rule is no comments in code). `go.mod` gained `golang.org/x/text` and `gorm.io/datatypes` as direct requires with the commit that first needs `x/text` (cd42c031); `go mod tidy` also moved `datatypes`, which was already imported, and added the `go.sum` lines it needs.
