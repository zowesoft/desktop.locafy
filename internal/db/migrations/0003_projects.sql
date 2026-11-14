CREATE TABLE projects (
	id              TEXT PRIMARY KEY,
	name            TEXT NOT NULL CHECK (length(trim(name)) > 0),
	root_path       TEXT NOT NULL CHECK (length(trim(root_path)) > 0),
	description     TEXT NOT NULL DEFAULT '',
	kind            TEXT NOT NULL DEFAULT 'linked' CHECK (kind IN ('managed', 'linked')),
	status          TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled', 'archived')),
	lifecycle_state TEXT NOT NULL DEFAULT 'stopped' CHECK (lifecycle_state IN ('stopped', 'starting', 'running', 'stopping', 'error')),
	configuration   TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(configuration)),
	created_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	updated_at      TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	deleted_at      TEXT
);

-- A path is used by at most one live project; removed projects release it.
CREATE UNIQUE INDEX idx_projects_root_path ON projects (root_path) WHERE deleted_at IS NULL;
CREATE INDEX idx_projects_name ON projects (name COLLATE NOCASE);

CREATE TRIGGER trg_projects_updated_at AFTER UPDATE ON projects
BEGIN
	UPDATE projects SET updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = NEW.id;
END;
