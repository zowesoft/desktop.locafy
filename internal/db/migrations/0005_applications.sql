CREATE TABLE applications (
	id            TEXT PRIMARY KEY,
	project_id    TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	name          TEXT NOT NULL CHECK (length(trim(name)) > 0),
	-- Path relative to the project root, with forward slashes; '.' is the root.
	directory     TEXT NOT NULL CHECK (length(trim(directory)) > 0),
	language      TEXT NOT NULL DEFAULT '',
	framework     TEXT NOT NULL DEFAULT '',
	-- The file that suggested this application, empty for a manual record.
	marker        TEXT NOT NULL DEFAULT '',
	start_command TEXT NOT NULL DEFAULT '',
	stop_command  TEXT NOT NULL DEFAULT '',
	status        TEXT NOT NULL DEFAULT 'stopped' CHECK (status IN ('stopped', 'starting', 'running', 'stopping', 'error')),
	source        TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('detected', 'manual')),
	created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	updated_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	deleted_at    TEXT
);

-- One live application per directory inside a project; removal releases it.
CREATE UNIQUE INDEX idx_applications_project_directory
	ON applications (project_id, lower(directory)) WHERE deleted_at IS NULL;
CREATE INDEX idx_applications_project ON applications (project_id, name COLLATE NOCASE) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_applications_updated_at AFTER UPDATE ON applications
BEGIN
	UPDATE applications SET updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = NEW.id;
END;

-- Direction: application_id needs depends_on_id to be running first. The graph
-- is validated for cycles by the service before rows are written.
CREATE TABLE application_dependencies (
	application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
	depends_on_id  TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
	PRIMARY KEY (application_id, depends_on_id),
	CHECK (application_id <> depends_on_id)
);

CREATE INDEX idx_application_dependencies_target ON application_dependencies (depends_on_id);
