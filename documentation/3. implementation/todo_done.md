# mwanachama-go-shared — completed tasks

| Task | Title | Completed | Notes |
|------|-------|-----------|-------|
| S1 | Bootstrap repo: `git init`, `go.mod`, `.gitignore`, `Makefile`, four-phase `documentation/` skeleton, `todo.md`/`todo_done.md` | 2026-09-02 | New repo, created alongside [mwanachama-git](../../../mwanachama-git) and [mwanachama-taskmanager](../../../mwanachama-taskmanager) as the shared Postgres entity-graph engine (avoids duplicating that engine in both consumers). |
| S2 | Define storage-agnostic entity-graph contract: `Entity`, `Relationship`, `DataManager`, `SchemaManager`, `Schema`/`TypeDefinition`/`RelationshipDefinition`/`PropertyDefinition` | 2026-09-02 | Ported from `CodeValdSharedLib/entitygraph` + `types` into two packages: [`schema/`](../../../schema) (pure type-system structs, renamed from the original's generic `types` package name) and [`entitygraph/`](../../../entitygraph) (`DataManager`/`SchemaManager` interfaces, `Entity`/`Relationship`/request/filter/result types, sentinel errors, `FindTypeDef`/`FindRelationshipDef`/`ValidateCreateRelationship`/`ValidateSchema`, and the `StringProp`/`BoolProp`/`Int64Prop`/`Float64Prop` converters). Interfaces are otherwise unchanged from the original — this is the seam `mwanachama-git`/`mwanachama-taskmanager` port their business logic against. Dropped along the way: `PathSegment`/`EntityIDParam`/`Code`/`RefCode`/`PublishEvents` fields and the route/topic-generation machinery they supported (`schemaroutes`, `TopicsFromSchema`) — the gateway registers routes and events by hand, it doesn't derive them from a schema; ported `entitygraph_test.go` accordingly (dropped the PathSegment-uniqueness cases, added two new ones for the `UniqueKey`-must-reference-a-declared-property rule, which the original tested only via doc comment). `go build ./...`, `go vet ./...`, `go test ./...` all clean. |

## Archived board context

`mwanachama-go-shared` — Go library, module `github.com/aosanya/mwanachama-go-shared`.
No audience of its own; a dependency of [mwanachama-git](../../../mwanachama-git)
and [mwanachama-taskmanager](../../../mwanachama-taskmanager), which are in turn
wired into [mwanachama-api-gateway](../../../mwanachama-api-gateway) for
[mwanachama-kazi](../../../mwanachama-kazi).

### Why this repo exists

`CodeValdGit` and `CodeValdWork` are ArangoDB-backed Go libraries built for
CodeValdCortex agencies. `mwanachama-kazi` needs the same git-like versioned
content and task/workflow management, but `mwanachama-api-gateway` standardizes
on Postgres and runs as a single service (no sub-services). Both source
libraries lean on a shared package, `CodeValdSharedLib/entitygraph` — a
generic entity/relationship graph store — for almost everything that touches
storage; their own business logic (`git_impl_*.go`, `task*.go`) is already
storage-agnostic Go that only calls the `entitygraph.DataManager` interface.

Porting that one shared engine to Postgres once, here, means
`mwanachama-git` and `mwanachama-taskmanager` can port their respective
business logic with minimal churn instead of each reinventing entity/edge
storage and graph traversal.

Full task breakdown and rationale: see the plan this was scoped from
(`/Users/tony/.claude/plans/kind-snacking-rose.md` at the time of writing —
noted here since it predates any file in this repo).
