-- Health describes a live process. Clear any reading left on a record that is
-- no longer starting or running, including one written by a probe that was in
-- flight while the process ended.
UPDATE applications
SET health = '', health_detail = '', health_checked_at = ''
WHERE status NOT IN ('starting', 'running');
