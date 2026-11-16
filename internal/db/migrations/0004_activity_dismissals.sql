-- Dismissing an entry hides it from the top bar without erasing it: the activity
-- history stays complete for diagnostics, while the bell counts only what the
-- operator has not dealt with yet.
ALTER TABLE activity_logs ADD COLUMN dismissed_at TEXT;

CREATE INDEX idx_activity_logs_undismissed ON activity_logs (created_at DESC)
	WHERE dismissed_at IS NULL;
