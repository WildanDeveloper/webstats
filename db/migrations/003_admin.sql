


ALTER TABLE users ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'user';
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);

ALTER TABLE sites ADD COLUMN IF NOT EXISTS color TEXT NOT NULL DEFAULT '';

-- NOTE: no default admin account is seeded. Create the first admin via
-- POST /api/auth/register, then promote:
--   UPDATE users SET role='admin' WHERE email='...';