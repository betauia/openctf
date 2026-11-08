-- The app inserts UUIDv7 ids; the defaults cover rows added by hand (e.g. challenges).

-- +goose Up
CREATE TABLE teams (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,
    captain_id  UUID, -- foreign key added below, once users exists
    invite_code TEXT UNIQUE,
    banned      BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE users (
    id            UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    username      TEXT    NOT NULL UNIQUE,
    email         TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    score         INTEGER NOT NULL DEFAULT 0,
    role          TEXT    NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'maintainer', 'admin')),
    team_id       UUID REFERENCES teams (id) ON DELETE SET NULL,
    banned        BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX users_team_id_idx ON users (team_id);

ALTER TABLE teams
    ADD CONSTRAINT teams_captain_id_fkey FOREIGN KEY (captain_id) REFERENCES users (id) ON DELETE SET NULL;

CREATE TABLE challenges (
    id              UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    title           TEXT    NOT NULL,
    description     TEXT,
    author          TEXT,
    points          INTEGER NOT NULL,
    category        TEXT    NOT NULL,
    difficulty      TEXT    NOT NULL DEFAULT 'Easy',
    solves          INT NOT NULL DEFAULT 0,
    connection_info TEXT,
    docker_image    TEXT,
    docker_port     INTEGER DEFAULT 80,
    is_visible      BOOLEAN NOT NULL DEFAULT FALSE,
    created_by_id   UUID REFERENCES users (id) ON DELETE SET NULL,
    uploaded_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE challenge_files (
    id              UUID PRIMARY KEY,
    challenge_id    UUID NOT NULL REFERENCES challenges(id) ON DELETE CASCADE,
    filename        TEXT NOT NULL, 
    filepath        TEXT NOT NULL,
    uploaded_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE solves (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    challenge_id UUID        NOT NULL REFERENCES challenges (id) ON DELETE CASCADE,
    solved_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, challenge_id)
);
CREATE INDEX solves_challenge_id_idx ON solves (challenge_id);

-- Exactly one row: id is always TRUE.
CREATE TABLE settings (
    id              BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    ctf_name        TEXT NOT NULL DEFAULT 'OpenCTF',
    ctf_start       TIMESTAMPTZ NOT NULL DEFAULT now(),
    ctf_end         TIMESTAMPTZ NOT NULL DEFAULT now() + INTERVAL '7 days',
    registration    BOOLEAN NOT NULL DEFAULT TRUE,
    max_team_size   INTEGER NOT NULL DEFAULT 5,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
INSERT INTO settings DEFAULT VALUES;

-- +goose Down
DROP TABLE settings;
DROP TABLE solves;
DROP TABLE challenges;
ALTER TABLE teams DROP CONSTRAINT teams_captain_id_fkey;
DROP TABLE users;
DROP TABLE teams;
