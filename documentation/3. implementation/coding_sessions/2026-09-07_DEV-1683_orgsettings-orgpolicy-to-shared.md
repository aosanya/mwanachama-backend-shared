# DEV-1683 — orgchrome and orgpolicy fold into mwanachama-backend-shared

**Date:** 2026-09-07 · **Surface:** api-gateway+shared · **Status:** ✅ done

Started from running `/dev-qa`'s first journey
(`01-1-onboarding-self-enrollment`) against a fresh Postgres, which 500'd on
its very first request — `org_chrome`'s migration had been archived with
nothing active replacing it. Root-caused, then executed the design-register
decision that archival was already anticipating: fold `orgchrome`/`orgpolicy`
into `mwanachama-backend-shared`, matching `mwanachama-backend-actor`'s
`models`/`gormstore`/`routes` split.

## What was built

- `mwanachama-backend-shared/orgsettings/` — new package: `models/` (`Settings`
  with `Slug` + an `Attributes` bag, `PublicSettings` guest projection,
  `Property`/`ValidateAttributes`), `gormstore/` (`SettingsRow`, JSONB
  `Attributes` column, `Migrate`), `routes/` (`GetOrgSettings`/
  `GetOwnOrgSettings`/`PutOrgSettings`), root `store_impl.go`/`region.go`.
  Renamed end-to-end from `orgchrome`/`org_chrome`/`/v1/org-chrome` — package,
  types, DB table, HTTP routes, `CapOrgChromeWrite`→`CapOrgSettingsWrite`.
- `mwanachama-backend-shared/orgpolicy/` — same split for the organization's
  settable caps (`Policy.Get`/`Set`) plus a new `org_policy_overrides` table
  for the per-member public-address-cap raise, gated by a `MemberExists`
  callback.
- `mwanachama-backend-api-gateway`: deleted `internal/domain/{orgchrome,
  orgpolicy}` and their Postgres/memory stores; `router.go` now mounts
  `orgsettingsroutes`/`orgpolicyroutes` directly; `org_policy_handlers.go`
  keeps only the two per-member composition handlers
  (`setMemberPublicAddressCap`/`getMemberPublicAddressCap`, which read
  `d.Addresses.CountPublic` and so must stay gateway-side); `cmd/server/
  stores.go` wires both domains' `Migrate`/`NewStore` on both backends.
- `internal/store/postgres/migrations/000004_org_settings.up.sql` and
  `000005_org_policy.up.sql` — active mirrors of each package's `gormstore`
  schema, so `cmd/migrate up` alone provisions a fresh database (it did not
  before; only actor/comm/form had one). Each domain's own archived
  migrations moved to live with the code, at
  `mwanachama-backend-shared/<pkg>/migrations_archive/`.
- `mwanachama-frontend-mobile` and `-kazi`: `OrganizationChrome`→
  `OrganizationSettings` rename (model, repository, route paths), done by two
  parallel background agents; both independently caught and fixed a live
  dependency the file list didn't name — `web-manual-sweep`'s `boot-app.sh`
  calling the old `/v1/org-chrome` route.

## Key decisions

- **`orgpolicy`'s per-member override gets a new table in `shared`, not a
  column restored on `member_actors`.** Its old home — a column on the
  gateway's own retired `member` table — has no equivalent on
  `mwanachama-backend-actor`'s `member_actors`; `shared` importing `actor`
  to reach one would be a new, unwanted cross-repo dependency. Owner's call
  when the fork was found mid-task.
- **Everything but `Slug` collapsed into an `Attributes` bag**, mirroring
  `mwanachama-backend-actor/models.Actor` exactly rather than keeping
  `DisplayName`/`PrimaryColor`/etc. as named columns — owner-directed,
  explicitly to "capture a bigger scope in future": new cross-repo settings
  can be declared as a `Property` with no schema migration. The DEV-1284
  guest-fence (`PublicSettings`) is unaffected — it stays an explicit named
  field list, just reading each field out of the bag by name instead of off
  a struct field, so nothing declared later reaches a guest by accident.
- **`org_policy`'s CHECK constraints stayed real database constraints**,
  not folded into Go-only validation the way `orgsettings`' dialling-region
  shape check was dropped — `orgpolicy`'s caps are fixed, known fields with
  no extensibility need, so there was no tension to resolve the way the
  guest-fence/attribute-bag one had.
- **Active migration mirrors are now a standing requirement for every
  GORM-backed domain**, not just a `cmd/migrate up` nicety — owner correction
  ("we have gorm in each backend repo that pushes the migrations to
  .../migrations") after a fresh database booted the server fine (self-heal)
  but left `cmd/migrate up` alone unable to provision the same shape.

## Validation

`mwanachama-backend-shared`: `go build`/`go vet`/`go test ./...` clean on
both new packages, including `POSTGRES_URL`-gated live tests against a real
Postgres (`org_settings`/`org_policy` JSONB round-trip, the four `org_policy`
CHECK constraints proven to refuse a bad row at the database, not just in
Go). `mwanachama-backend-api-gateway`: `go build`/`go vet`/`go test
-count=1 ./...` clean, both untagged and `-tags postgres`, except one
pre-existing failure unrelated to this work
(`TestPostmanCollectionHasNoOrphanRequests`, entirely `consent`/
`custody-log`/`act-log` routes). Live: `cmd/migrate up` reached version 5 on
a fresh database, gateway booted clean, `/v1/org-policy` confirmed correctly
capability-gated (401 with no session). `mwanachama-frontend-mobile`/`-kazi`:
`flutter analyze` clean, targeted tests 11/11 each; each repo's full suite
carries the same pre-existing, unrelated failures (survey/DM/reauth
integration tests needing a live gateway) confirmed via grep to not
reference chrome/settings.

## Follow-ups

- `domerr` — the third package the same design decision named alongside
  `orgpolicy`/`orgsettings` — is untouched; out of scope for this session.
- `orgpolicy_default_test.go`-equivalent coverage (Go constant vs. column
  default) wasn't ported; the two now agree by construction (one file,
  `gormstore`, defines both).
- The `consent`/`custody-log`/`act-log` Postman-coverage failure is a
  different, concurrent session's work landing in the same working tree —
  flagged, not fixed here.
