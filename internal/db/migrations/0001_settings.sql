CREATE TABLE settings (
	key        TEXT PRIMARY KEY,
	value      TEXT NOT NULL,
	value_type TEXT NOT NULL CHECK (value_type IN ('string', 'boolean', 'number', 'json')),
	updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
