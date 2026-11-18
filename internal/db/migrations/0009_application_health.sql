-- Whether a running application accepts connections on its discovered port.
-- Empty health means unknown: not running, not bound yet, or not checked.
ALTER TABLE applications ADD COLUMN health TEXT NOT NULL DEFAULT ''
	CHECK (health IN ('', 'healthy', 'unhealthy'));
ALTER TABLE applications ADD COLUMN health_detail TEXT NOT NULL DEFAULT '';
ALTER TABLE applications ADD COLUMN health_checked_at TEXT NOT NULL DEFAULT '';
