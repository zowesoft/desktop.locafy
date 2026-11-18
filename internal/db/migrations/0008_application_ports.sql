-- The last port an application listened on, and why a start was refused when
-- that port was already held. Both are observations, never configuration.
ALTER TABLE applications ADD COLUMN port INTEGER;
ALTER TABLE applications ADD COLUMN port_conflict TEXT NOT NULL DEFAULT '';
