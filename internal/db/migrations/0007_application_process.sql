-- The outcome of an application's last run. `status` already carries the
-- process state (stopped, starting, running, stopping, error); these columns
-- keep what explains it: the exit code, the failure message and when the
-- process last started.
ALTER TABLE applications ADD COLUMN exit_code INTEGER;
ALTER TABLE applications ADD COLUMN error TEXT NOT NULL DEFAULT '';
ALTER TABLE applications ADD COLUMN started_at TEXT NOT NULL DEFAULT '';
