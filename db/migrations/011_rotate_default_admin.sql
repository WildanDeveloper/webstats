-- 011_rotate_default_admin.sql
-- Older installs seeded admin@webstats.dev with the password 'admin123'
-- (migration 003, since removed). If that weak default is still in effect,
-- replace it with an unguessable random password so the account cannot be
-- used to log in until the owner resets it.
WITH rotated AS (
  UPDATE users
  SET password_hash = crypt(encode(gen_random_bytes(18), 'hex'), gen_salt('bf'))
  WHERE email = 'admin@webstats.dev'
    AND password_hash = crypt('admin123', password_hash)
  RETURNING id
), revoked AS (
  DELETE FROM sessions WHERE user_id IN (SELECT id FROM rotated)
  RETURNING user_id
)
SELECT count(*) AS rotated_default_admin FROM rotated;
