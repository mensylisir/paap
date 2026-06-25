-- Create the platform user role table and move runtime authorization out of users.role.
CREATE TABLE IF NOT EXISTS user_roles (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMPTZ,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(30) NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_user_roles_user_role
ON user_roles (user_id, role);

CREATE INDEX IF NOT EXISTS idx_user_roles_deleted_at
ON user_roles (deleted_at);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'users'
          AND column_name = 'role'
    ) THEN
        INSERT INTO user_roles (created_at, updated_at, user_id, role)
        SELECT CURRENT_TIMESTAMP,
               CURRENT_TIMESTAMP,
               id,
               CASE WHEN role = 'admin' THEN 'platform_admin' ELSE role END
        FROM users
        WHERE role IN ('admin', 'platform_admin', 'app_admin', 'user')
        ON CONFLICT (user_id, role) DO NOTHING;

        INSERT INTO user_roles (created_at, updated_at, user_id, role)
        SELECT CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, id, 'app_admin'
        FROM users
        WHERE username = 'admin' OR role = 'admin'
        ON CONFLICT (user_id, role) DO NOTHING;
    END IF;
END $$;

INSERT INTO user_roles (created_at, updated_at, user_id, role)
SELECT CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, id, 'platform_admin'
FROM users
WHERE username = 'admin'
ON CONFLICT (user_id, role) DO NOTHING;

INSERT INTO user_roles (created_at, updated_at, user_id, role)
SELECT CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, id, 'app_admin'
FROM users
WHERE username = 'admin'
ON CONFLICT (user_id, role) DO NOTHING;

INSERT INTO user_roles (created_at, updated_at, user_id, role)
SELECT CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, users.id, 'user'
FROM users
WHERE NOT EXISTS (
    SELECT 1 FROM user_roles WHERE user_roles.user_id = users.id
)
ON CONFLICT (user_id, role) DO NOTHING;
