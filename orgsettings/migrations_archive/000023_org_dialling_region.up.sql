-- DEV-1258 · `org_settings.default_dialling_region` — the region every
-- nationally-formatted phone number is canonicalized against, per G366's
-- decision (DSN-1479, 2026-08-22) and
-- documentation/2. design and architecture/datamodel/phone-salt.md.
--
-- New column on this plane, so it is built here and never on `supabase/party`,
-- which is retiring in full (CLAUDE.md, DSN-1349, owner decision 2026-08-21).
--
-- # Why this column exists at all
--
-- G16 matches a contribution to a member by comparing HMACs of their phone
-- numbers, and an HMAC has no notion of "nearly": `0712 445 678` and
-- `+254712445678` are two members to it. G366 decided the reconciliation is
-- E.164, and E.164 cannot be computed from a nationally-formatted number
-- without knowing which country it is national to. Nothing on this plane held
-- one — `SELECT ... WHERE column_name LIKE '%region%' OR LIKE '%dial%'`
-- returned 0 rows at migration 19 — so the rule G366 decided was undeployable.
-- This is that missing input.
--
-- # Why it lives on `org_settings` rather than in a new table
--
-- `org_settings` is already the per-organization record the **unauthenticated**
-- client shell reads, and the region's first consumer is an unauthenticated
-- screen: the country-code chip at `member/member-signin-phone.html:69`
-- (`🇰🇪 +254`), which G366 says is pre-filled from M88 admin-org-settings and
-- editable by the person typing. A region kept behind auth could not pre-fill
-- the screen that needs it before anyone has signed in. Its write path is
-- already the right one too — `PUT /v1/org-settings/{slug}` is gated by
-- `CapOrgSettingsWrite`, so "a person sets this on M88" is enforced by the
-- capability the route already carries rather than by a new one.
--
-- Publishing it is not a leak: a dialling region is the country the
-- organization operates in, which its own branding already announces.
--
-- # The empty string is "nobody has set one", and it is the default on purpose
--
-- phone-salt.md's accepted trade-off is that a wrong region mis-hashes every
-- nationally-formatted number entered under it **and there is nothing to
-- re-hash from** — G38's problem through a second door. That is why G366 makes
-- the region something a person chooses on a screen before the first
-- enrolment, never something inferred.
--
-- So this column must not carry a guess, and `NOT NULL DEFAULT ''` is how it
-- refuses to. There is no `'KE'` default: this repository is scoped beyond one
-- country (CLAUDE.md), and a default that is right for the flagship deployment
-- would be silently, permanently wrong for the next one — the exact failure
-- G366 names. Backfilling the existing rows with a country would be inventing
-- the one fact the design set says must be asked for.
--
-- The empty string therefore means **unset**, and the canonicalizer refuses to
-- run against it (internal/domain/phonenumber). An organization that has not
-- set a region cannot hash a national number — which is a loud provisioning
-- failure at the first enrolment, and is strictly better than a quiet one
-- discovered when a member's contributions do not match months later.
--
-- # The CHECK admits exactly two shapes, and no third
--
-- ISO-3166-1 alpha-2, uppercase, or empty. It is a CHECK and not a Go guard
-- for 000022's stated reason: a Go guard is one call site away from being
-- bypassed, and this is a value whose wrongness is unrecoverable. Lowercase is
-- refused rather than folded, because `libphonenumber` is case-sensitive on
-- region codes and a store that silently accepted `ke` would hand the parser a
-- region it does not know — failing open into "unparseable" for every national
-- number in the organization.
--
-- What is deliberately NOT checked here is whether the two letters name a
-- region the parser actually supports. That vocabulary belongs to the library
-- and changes when a regulator changes it; pinning it into a migration nobody
-- re-reads is 000019's mistake. An unsupported-but-well-formed code fails in
-- Go at the parse, where the error can say so.

ALTER TABLE org_settings
    ADD COLUMN IF NOT EXISTS default_dialling_region TEXT NOT NULL DEFAULT '';

ALTER TABLE org_settings
    DROP CONSTRAINT IF EXISTS org_settings_dialling_region_shape;

ALTER TABLE org_settings
    ADD CONSTRAINT org_settings_dialling_region_shape
    CHECK (default_dialling_region = '' OR default_dialling_region ~ '^[A-Z]{2}$');
