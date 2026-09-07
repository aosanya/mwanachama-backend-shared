-- DEV-1258 · reverse `org_settings.default_dialling_region`.
--
-- Worth saying plainly, as 000019 and 000022 do: what this drops is not a
-- display preference. It is the input every already-written `phone_hash` was
-- computed under, and G366's accepted trade-off is that a number canonicalized
-- under the wrong region cannot be re-hashed — the raw number was discarded at
-- entry and there is nothing to recompute from.
--
-- So dropping this column on a plane that has enrolled anybody does not
-- restore a previous state; it erases the record of which rule the existing
-- hashes were written under, while leaving the hashes themselves in place.
-- Re-applying 000023 afterwards gives back an empty column, not the region
-- that was there, and an operator who then sets a different one has silently
-- split the register into numbers hashed two ways with no way to tell which is
-- which.
--
-- This is safe on a plane that has hashed nothing. On one that has, the honest
-- recovery is to set the column back to the same region, never to drop it.

ALTER TABLE org_settings
    DROP CONSTRAINT IF EXISTS org_settings_dialling_region_shape;

ALTER TABLE org_settings
    DROP COLUMN IF EXISTS default_dialling_region;
