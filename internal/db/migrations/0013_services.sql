-- A service instance: one engine data directory, one allocated port, one
-- supervised process, owned by a project.
CREATE TABLE services (
	id             TEXT PRIMARY KEY,
	project_id     TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	name           TEXT NOT NULL CHECK (length(trim(name)) > 0),
	engine         TEXT NOT NULL CHECK (engine IN ('postgresql')),
	version        TEXT NOT NULL DEFAULT '',
	-- Allocated by Locafy, preferred first and unique while the instance lives.
	port           INTEGER NOT NULL CHECK (port BETWEEN 1 AND 65535),
	-- Absolute path to the data directory; never inside a linked project.
	data_directory TEXT NOT NULL CHECK (length(trim(data_directory)) > 0),
	database_name  TEXT NOT NULL DEFAULT 'locafy',
	user_name      TEXT NOT NULL DEFAULT 'locafy',
	status         TEXT NOT NULL DEFAULT 'stopped'
		CHECK (status IN ('stopped', 'starting', 'running', 'stopping', 'error')),
	error          TEXT NOT NULL DEFAULT '',
	started_at     TEXT NOT NULL DEFAULT '',
	health         TEXT NOT NULL DEFAULT '' CHECK (health IN ('', 'healthy', 'unhealthy')),
	health_detail  TEXT NOT NULL DEFAULT '',
	health_checked_at TEXT NOT NULL DEFAULT '',
	pid            INTEGER,
	created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	updated_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	deleted_at     TEXT
);

CREATE INDEX idx_services_project ON services (project_id, name COLLATE NOCASE) WHERE deleted_at IS NULL;
-- One live instance per name inside a project; a port is held by one live row.
CREATE UNIQUE INDEX idx_services_project_name
	ON services (project_id, lower(name)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_services_port ON services (port) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_services_updated_at AFTER UPDATE ON services
BEGIN
	UPDATE services SET updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = NEW.id;
END;
