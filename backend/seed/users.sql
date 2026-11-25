-- Seed users for development only. Never run this against a real database: every account has the
-- same published password, dev-password-123.
-- Usage: cat backend/seed/users.sql | docker compose exec -T database psql -U openctf -d openctf
-- Users that already exist (same username or email) are skipped. The default 'admin' user is
-- created by the server on start, so it isn't seeded here.

\set ON_ERROR_STOP on
BEGIN;

INSERT INTO users (username, email, password_hash, role, banned)
SELECT v.username, v.username || '@example.com', 'XdNkcZ+ulT4CtVZVE71i/FEDfxM05e7gyFeExMtKuthAMIY9chCqKd+7Leh2U/zv', v.role, v.banned FROM (VALUES
    ('dev_admin',       'admin',      FALSE),
    ('dev_maintainer',  'maintainer', FALSE),
    ('alice_ctf',       'user',       FALSE),
    ('bob_ctf',         'user',       FALSE),
    ('carol_ctf',       'user',       FALSE),
    ('dave_ctf',        'user',       FALSE),
    ('erin_ctf',        'user',       FALSE),
    ('frank_ctf',       'user',       FALSE),
    ('grace_ctf',       'user',       FALSE),
    ('heidi_ctf',       'user',       FALSE),
    ('ivan_ctf',        'user',       FALSE),
    ('judy_ctf',        'user',       FALSE),
    ('mallory_ctf',     'user',       TRUE)
) AS v (username, role, banned)
WHERE NOT EXISTS (
    SELECT 1 FROM users u WHERE lower(u.username) = lower(v.username) OR u.email = v.username || '@example.com'
);

COMMIT;
