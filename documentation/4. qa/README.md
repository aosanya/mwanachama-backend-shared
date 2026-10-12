# QA

Test coverage and results for `mwanachama-backend-shared`.

## How to run it

`go test ./...` runs every package offline, on SQLite (`glebarez/sqlite`) or
in memory. `orgpolicy/postgres_integration_test.go` and
`orgsettings/postgres_integration_test.go` skip unless `POSTGRES_URL` is set,
so a real Postgres is never reached by default.

## What the suite covers

Test functions per package (`go test ./<pkg>/ -list '.*'`), counted
2026-10-03 and re-counted for `dispatch` and `spec` on 2026-10-09 (S54's
`querykey_fallback_test.go` and S55's `default_literal_security_test.go`) and for `dispatch` on 2026-10-12 (S57's `refers_test.go`, S58's `conversation_test.go` and the security sweep's `refers_alphabet_security_test.go` and `conversation_alphabet_security_test.go`):

| Package | Tests | What it holds |
|---------|-------|---------------|
| `dispatch` | 95 | the operations engine — binding, routes, MCP tools and the authorizer gate |
| `spec` | 56 | loading, validation and blueprint merge of a declared domain; the migrator |
| `entitygraph` | 23 | the entity-graph contract |
| `specstore` | 31 | the codec and query helpers every declared module stores through |
| `gormutil` | 13 | shared GORM helpers |
| `httpwire` | 8 | the `Route` type and JSON error writing |
| `orgpolicy`, `orgpolicy/models` | 9, 6 | org policy store and types |
| `orgsettings`, `orgsettings/models` | 6, 6 | org settings store and types |
| `postgres` | 3 | the Postgres entity-graph backend |
| `gormtest`, `vocab`, `memory` | 2, 2, 1 | test helpers, vocabulary, in-memory backend |

## The authorizer gate

`dispatch/authorize_test.go` pins the gate every declared module mounts
behind: a refused route answers `403` and never reaches the manager, a refused
tool is refused the same way, the refusal does not name the action, and the
authorizer is asked before any argument is read. The same gate is exercised
end to end through a real module's route table by
`mwanachama-backend-catalog/routes/deny_all_security_test.go` (security sweep,
2026-09-29), which was mutation-checked against `handlerFor` and `invokerFor`
here: skipping either gate for one action turns it red.

The anonymous split keys on the action id, so it is only as narrow as one
action per operation. `dispatch/gate_declaration_security_test.go` (security
sweep, 2026-10-10) pins that `Parse` refuses a DELETE claiming the action an
open GET carries, refuses an operation declaring no action, and that opening
one action publishes exactly one route (mutation-checked by disabling both
refusals in `Spec.Validate`: two of the three turn red). A duplicated JSON
key in an operations file is decoded last-wins — an operation or its
`action` can be silently replaced — but no shipped declaration in the six
repos carries one, and none is built from caller data.

## Red by design

- `specstore/store_listcap_test.go`'s `TestS24_OpenBug_ListHasNoDefaultCap`
  (S24) — the integration sweep's, left failing on purpose until `List`
  gains a default cap.
- `spec/migrate_evolve_test.go`'s
  `TestS52_OpenBug_SecondMigrateDoesNotAddANewColumn` (S52) — the integration
  sweep's, left failing on purpose until a second `Migrate` against an
  already-provisioned table alters it to add a field the blueprint gained.
- `spec/migrate_drop_field_test.go`'s
  `TestS53_OpenBug_DroppedRequiredFieldOrphansANotNullColumn` (S53) — the
  integration sweep's, left failing on purpose until dropping a `required`
  field's column (or relaxing its `not null`) is part of the same fix S52
  needs.
- `dispatch/querykey_fallback_test.go`'s
  `TestS54_OpenBug_UntaggedQueryFieldBindsOnlyAllLowercaseNotSnakeCase` (S54)
  — the integration sweep's, left failing on purpose until an untagged query
  field binds on its snake_case name.
- `spec/default_literal_security_test.go`'s
  `TestS55_OpenHole_NumericDefaultReachesTheDDLUnquoted` (S55) — the security
  sweep's, left failing on purpose until `spec.Parse` refuses an `int`,
  `float` or `bool` default that is not a literal of its type; today one
  carrying a stacked statement is executed by `spec.Migrate`.
- `identity/identity_security_test.go`'s
  `TestS60_OpenHole_ARawCarriesInvisibleAndLineBreakingCharacters` (S60) —
  the security sweep's, left failing on purpose until `identity.Parse`
  refuses a bidi override, a zero-width character or a line separator in the
  identity; today each is accepted as a distinct raw that renders as another.
- `dispatch/tool_name_collision_security_test.go`'s
  `TestS63_OpenHole_ExplicitToolNameTakesAnotherOperationsDerivedName` (S63)
  — the security sweep's, left failing on purpose until `Validate`'s
  `seenTool` set is built from every operation's *published* name,
  explicit or derived, not only an explicit one; today an operation
  giving itself an explicit `tool` name equal to another operation's
  derived name is accepted and both publish under it.

`spec/mount_collision_security_test.go`'s
`TestS28_OpenHole_MountNameSpellsTheDefaultMountsTable` (S28) is no longer
red: S29's hashed physical name (f96ad8b) gives the mount its own segment, so
a mount named `patient` can no longer spell the default mount's
`patient_page_views`. It passes on 51d191d (security sweep, 2026-10-01) and
still carries its `_OpenHole_` name until it is renamed as a regression guard.

The org routes are pinned green through their own route tables (security
sweep, 2026-10-09): `orgsettings/guest_fence_security_test.go` — an attribute
the property catalog does not declare is stored by `PUT /{slug}` yet never
reaches either guest `GET`, and a body `slug` never moves the write off the
path's (mutation-checked by returning the stored row instead of `Public()`);
`orgpolicy/caller_owned_security_test.go` — `updated_by` is the caller's, a
body-set `updated_by`/`updated_at` is never recorded, and a trailing second
value or a case-variant duplicate key is refused (mutation-checked by
decoding a body `updated_by` over the caller's).

The mount segment (S26) is pinned green by
`spec/mount_segment_security_test.go` (security sweep, 2026-09-29): an
underscore, capital, hyphen, space, `;` or `.` in `mount` is refused by name,
and a mount that pushes a name past 63 bytes is refused (mutation-checked by
dropping the `SegmentPattern` check on the mount).

The `refers` mark (S57) is pinned green by
`dispatch/refers_alphabet_security_test.go` (security sweep, 2026-10-12): a
mark carrying a newline, carriage return, line separator, NUL, zero-width
space, right-to-left override, byte order mark, soft hyphen, a Cyrillic or
accented lookalike, a leading or trailing space or a doubled dot is refused at
load, and a case-variant `REFERS`/`Refers` key is held to the same pattern —
the contrast with S60, where `identity.Parse` accepts the same characters
(mutation-checked by loosening `RefersPattern` to `^\S+\.\S+`: 14 red).

The conversation id (S58) is pinned green by
`dispatch/conversation_alphabet_security_test.go` (security sweep,
2026-10-12): an `X-Conversation-Id` header or a `_meta.conversation_id`
carrying a newline, carriage return, line separator, NUL, no-break or
zero-width space, right-to-left override, byte order mark, soft hyphen, a
fullwidth or Cyrillic lookalike, a comma-joined list, a leading `.`/`:`/`_` or
129 characters is refused before the handler runs and the refusal never
repeats the value; a case-variant `_meta` key or an empty or null value
leaves the host's conversation standing; and two spellings of the header on
the wire (`x-conversation-id` and `X-CONVERSATION-ID`) are one header sent
twice, a 400 (mutation-checked by loosening `ConversationPattern` to any
character: 4 red; by allowing a repeated header: 2 red). The id is asserted by
the caller and bound to nobody — nothing in the six reads `ConversationOf`
yet, so whichever module first records against it owns scoping it to the
caller.
