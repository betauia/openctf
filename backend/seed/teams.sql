-- Seed teams for development only. Run users.sql first.
-- Usage: cat backend/seed/teams.sql | docker compose exec -T database psql -U openctf -d openctf
-- Teams that already exist (same name in any case) are skipped, and only users without a team are
-- put in one.
--   Alpha Squad    5 members, full at the default max_team_size
--   Beta Team      2 members
--   Gamma          just the captain
--   Banned Bandits a banned team
--   Shadow Ops     a hidden team
-- dev_admin, dev_maintainer, kate_ctf, leo_ctf and mallory_ctf (banned) are left without a team.

\set ON_ERROR_STOP on
BEGIN;

INSERT INTO teams (name, captain_id, invite_code, banned, hidden)
SELECT v.name, u.id, v.invite_code, v.banned, v.hidden FROM (VALUES
    ('Alpha Squad',    'alice_ctf', 'dev-alpha',   FALSE, FALSE),
    ('Beta Team',      'frank_ctf', 'dev-beta',    FALSE, FALSE),
    ('Gamma',          'heidi_ctf', 'dev-gamma',   FALSE, FALSE),
    ('Banned Bandits', 'ivan_ctf',  'dev-bandits', TRUE,  FALSE),
    ('Shadow Ops',     'judy_ctf',  'dev-shadow',  FALSE, TRUE)
) AS v (name, captain, invite_code, banned, hidden)
JOIN users u ON u.username = v.captain
WHERE NOT EXISTS (SELECT 1 FROM teams t WHERE lower(t.name) = lower(v.name));

UPDATE users SET team_id = t.id
FROM (VALUES
    ('alice_ctf', 'Alpha Squad'), ('bob_ctf', 'Alpha Squad'), ('carol_ctf', 'Alpha Squad'),
    ('dave_ctf', 'Alpha Squad'), ('erin_ctf', 'Alpha Squad'),
    ('frank_ctf', 'Beta Team'), ('grace_ctf', 'Beta Team'),
    ('heidi_ctf', 'Gamma'),
    ('ivan_ctf', 'Banned Bandits'),
    ('judy_ctf', 'Shadow Ops')
) AS m (username, team)
JOIN teams t ON lower(t.name) = lower(m.team)
WHERE users.username = m.username AND users.team_id IS NULL;

COMMIT;
