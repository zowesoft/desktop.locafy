CREATE TABLE install_jobs (
	id         TEXT PRIMARY KEY,
	kind_id    TEXT NOT NULL,
	label      TEXT NOT NULL,
	-- The installable spec as JSON, so a retry can re-run the same job
	-- without the catalogue having stayed identical.
	spec       TEXT NOT NULL,
	state      TEXT NOT NULL DEFAULT 'queued'
		CHECK (state IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
	progress   INTEGER NOT NULL DEFAULT 0,
	total      INTEGER NOT NULL DEFAULT 0,
	error      TEXT NOT NULL DEFAULT '',
	log_path   TEXT NOT NULL DEFAULT '',
	target_dir TEXT NOT NULL DEFAULT '',
	created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX idx_install_jobs_created ON install_jobs (created_at DESC);
