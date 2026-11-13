CREATE TABLE machine (
	id           TEXT PRIMARY KEY,
	hostname     TEXT NOT NULL,
	platform     TEXT NOT NULL,
	architecture TEXT NOT NULL,
	os_version   TEXT NOT NULL,
	created_at   TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at   TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE activity_logs (
	id         TEXT PRIMARY KEY,
	scope_type TEXT NOT NULL CHECK (scope_type IN ('machine', 'project', 'application', 'service')),
	scope_id   TEXT,
	action     TEXT NOT NULL,
	detail     TEXT,
	created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_activity_logs_created_at ON activity_logs (created_at DESC);

CREATE TRIGGER trg_machine_updated_at AFTER UPDATE ON machine
BEGIN
	UPDATE machine SET updated_at = CURRENT_TIMESTAMP WHERE id = NEW.id;
END;

CREATE TRIGGER trg_settings_updated_at AFTER UPDATE ON settings
BEGIN
	UPDATE settings SET updated_at = CURRENT_TIMESTAMP WHERE key = NEW.key;
END;
