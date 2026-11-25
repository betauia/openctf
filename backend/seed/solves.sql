-- Seed solves for development only, so the scoreboards, activity feed and charts have data.
-- Run challenges.sql, users.sql and teams.sql first.
-- Usage: cat backend/seed/solves.sql | docker compose exec -T database psql -U openctf -d openctf
-- A solve that already exists is skipped. Afterwards the seeded users' scores and every challenge's
-- solve count are recalculated from the solves, the way the server keeps them.

\set ON_ERROR_STOP on
BEGIN;

-- Each player solves a challenge for their team; the time is how many hours ago.
INSERT INTO solves (user_id, team_id, challenge_id, solved_at)
SELECT u.id, u.team_id, c.id, now() - make_interval(hours => v.hours_ago)
FROM (VALUES
    ('alice_ctf', 'Hello World',      30), ('alice_ctf', 'SQL Safari',      28), ('alice_ctf', 'RSA Baby Steps',   20),
    ('bob_ctf',   'Idle No More',     27), ('bob_ctf',   'Crackme 001',     22), ('bob_ctf',   'Heap Havoc',       10),
    ('carol_ctf', 'Steg & Destroy',   26), ('carol_ctf', 'Log Digger',      14),
    ('dave_ctf',  'Pixel Secrets',    24),
    ('frank_ctf', 'Hello World',      29), ('frank_ctf', 'Fix My Script',   18),
    ('grace_ctf', 'Insta Stalker',    21),
    ('heidi_ctf', 'Hello World',      25),
    ('ivan_ctf',  'Idle No More',     23),
    ('judy_ctf',  'Hello World',      19)
) AS v (username, title, hours_ago)
JOIN users u ON u.username = v.username
JOIN challenges c ON c.title = v.title
ON CONFLICT DO NOTHING;

UPDATE users SET score = COALESCE((
    SELECT SUM(c.points) FROM solves s JOIN challenges c ON c.id = s.challenge_id WHERE s.user_id = users.id
), 0)::int
WHERE username LIKE '%\_ctf';

UPDATE challenges SET solves = (SELECT COUNT(*) FROM solves s WHERE s.challenge_id = challenges.id);

COMMIT;
