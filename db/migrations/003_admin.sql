


ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user';
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);

ALTER TABLE sites ADD COLUMN IF NOT EXISTS color TEXT NOT NULL DEFAULT '';


INSERT INTO users (email, password_hash, name, role)
VALUES ('admin@webstats.dev', crypt('admin123', gen_salt('bf')), 'Admin', 'admin')
ON CONFLICT (email) DO NOTHING;