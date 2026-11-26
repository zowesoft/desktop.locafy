-- MySQL joins the catalogue as the second engine ADR-0020 admits. The engine
-- column's CHECK constraint is part of the table definition, which SQLite cannot
-- alter, so the table is rebuilt with the same columns and constraints and one
-- wider engine list. Indexes and the updated_at trigger are recreated with it.
CREATE TABLE services_new (
	id             TEXT PRIMARY KEY,
	project_id     TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	name           TEXT NOT NULL CHECK (length(trim(name)) > 0),
	engine         TEXT NOT NULL CHECK (engine IN ('postgresql', 'mysql')),
	version        TEXT NOT NULL DEFAULT '',
	port           INTEGER NOT NULL CHECK (port BETWEEN 1 AND 65535),
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

INSERT INTO services_new (id, project_id, name, engine, version, port, data_directory,
	database_name, user_name, status, error, started_at, health, health_detail,
	health_checked_at, pid, created_at, updated_at, deleted_at)
SELECT id, project_id, name, engine, version, port, data_directory,
	database_name, user_name, status, error, started_at, health, health_detail,
	health_checked_at, pid, created_at, updated_at, deleted_at
FROM services;

DROP TABLE services;
ALTER TABLE services_new RENAME TO services;

CREATE INDEX idx_services_project ON services (project_id, name COLLATE NOCASE) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_services_project_name
	ON services (project_id, lower(name)) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_services_port ON services (port) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_services_updated_at AFTER UPDATE ON services
BEGIN
	UPDATE services SET updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = NEW.id;
END;
