-- 001_orgsettings.sql — first-domain proof-of-swap schema for the api-gateway
-- Postgres store. Sibling migrations land per domain as they are implemented.
--
-- Runner: `psql "$POSTGRES_URL" -f schema/001_orgsettings.sql`, or the
-- docker-compose service that lands in a later firing.

CREATE TABLE IF NOT EXISTS org_settings (
    slug          TEXT PRIMARY KEY,
    display_name  TEXT NOT NULL DEFAULT '',
    primary_color TEXT NOT NULL DEFAULT '',
    accent_color  TEXT NOT NULL DEFAULT '',
    logo_url      TEXT NOT NULL DEFAULT '',
    support_email TEXT NOT NULL DEFAULT '',
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
