-- Ensure seeded and existing PAAP platform admin accounts use the approved
-- login password hash. The plaintext password is not stored in code.
UPDATE users
SET password = '$2a$10$NyVh8MbDEoNYb0q1uX69xeWP144iBpnxX5odXF4NeA662qFY7muc6',
    updated_at = CURRENT_TIMESTAMP
WHERE username = 'admin';
