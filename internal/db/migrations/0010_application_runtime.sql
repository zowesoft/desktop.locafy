-- The runtime copy an application pins, as kind@path from the registry.
-- Empty means the machine's default PATH order resolves the start command.
ALTER TABLE applications ADD COLUMN runtime_id TEXT NOT NULL DEFAULT '';
