-- Seed default platform users. Runtime authorization is stored in user_roles.
INSERT INTO users (created_at, updated_at, username, email, password)
SELECT CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'admin', 'admin@paap.local', '$2a$10$3MKp8pOQ7lX40kQ499EezOmjI.9Xuuc3wFaG7xwcZNHjY2KqLQS.y'
WHERE NOT EXISTS (SELECT 1 FROM users WHERE username = 'admin');

INSERT INTO users (created_at, updated_at, username, email, password)
SELECT CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'user', 'user@paap.local', '$2a$10$3MKp8pOQ7lX40kQ499EezOmjI.9Xuuc3wFaG7xwcZNHjY2KqLQS.y'
WHERE NOT EXISTS (SELECT 1 FROM users WHERE username = 'user');

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
SELECT CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, id, 'user'
FROM users
WHERE username = 'user'
ON CONFLICT (user_id, role) DO NOTHING;
