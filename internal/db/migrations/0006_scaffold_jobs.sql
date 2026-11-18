CREATE TABLE scaffold_jobs (
	id           TEXT PRIMARY KEY,
	-- NULL until a project is registered for a scaffold that creates one.
	project_id   TEXT REFERENCES projects(id) ON DELETE CASCADE,
	recipe_id    TEXT NOT NULL,
	directory    TEXT NOT NULL,
	target_path  TEXT NOT NULL,
	status       TEXT NOT NULL DEFAULT 'queued'
		CHECK (status IN ('queued', 'running', 'succeeded', 'failed', 'cancelled')),
	error        TEXT NOT NULL DEFAULT '',
	log_path     TEXT NOT NULL DEFAULT '',
	commands     TEXT NOT NULL DEFAULT '',
	created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	updated_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

CREATE INDEX idx_scaffold_jobs_created ON scaffold_jobs (created_at DESC);

-- The recipe that produced an application, empty for detected or manual ones.
-- Kept additive so the applications table's source check stays as it was.
ALTER TABLE applications ADD COLUMN scaffold_recipe TEXT NOT NULL DEFAULT '';
ALTER TABLE applications ADD COLUMN scaffold_options TEXT NOT NULL DEFAULT '';
