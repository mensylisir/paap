-- user_roles was replaced by roles + role_bindings during development.
-- Keep this migration id as a cleanup step for databases that still have the old table.
DROP TABLE IF EXISTS user_roles;
