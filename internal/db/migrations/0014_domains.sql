-- A local hostname routed through Locafy's proxy to one application.
CREATE TABLE domains (
	id             TEXT PRIMARY KEY,
	project_id     TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
	application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
	-- Stored lowercased and validated at write time; the proxy routes on it.
	hostname       TEXT NOT NULL CHECK (length(trim(hostname)) > 0),
	created_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	updated_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
	deleted_at     TEXT
);

CREATE INDEX idx_domains_project ON domains (project_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_domains_application ON domains (application_id) WHERE deleted_at IS NULL;
-- One live claim per hostname, whichever project or application makes it.
CREATE UNIQUE INDEX idx_domains_hostname ON domains (lower(hostname)) WHERE deleted_at IS NULL;

CREATE TRIGGER trg_domains_updated_at AFTER UPDATE ON domains
BEGIN
	UPDATE domains SET updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE id = NEW.id;
END;
