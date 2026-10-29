-- The app inserts UUIDv7 ids; the defaults cover rows added by hand (e.g. challenges).

-- +goose Up
CREATE TABLE teams (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,
    invite_code TEXT UNIQUE
);

CREATE TABLE users (
    id            UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    username      TEXT    NOT NULL UNIQUE,
    email         TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    score         INTEGER NOT NULL DEFAULT 0,
    is_admin      BOOLEAN NOT NULL DEFAULT FALSE,
    team_id       UUID REFERENCES teams (id) ON DELETE SET NULL
);
CREATE INDEX users_team_id_idx ON users (team_id);

CREATE TABLE challenges (
    id              UUID    PRIMARY KEY DEFAULT gen_random_uuid(),
    title           TEXT    NOT NULL,
    description     TEXT,
    author          TEXT,
    points          INTEGER NOT NULL,
    category        TEXT    NOT NULL,
    difficulty      TEXT    NOT NULL DEFAULT 'Easy',
    solves          INTEGER NOT NULL DEFAULT 0,
    connection_info TEXT,
    -- Array of plain strings or {"type": "static"|"regex", "content": ..., "data": "case_insensitive"?}
    flags           JSONB   NOT NULL DEFAULT '[]',
    -- Filenames in the upload dir
    files           TEXT[]  NOT NULL DEFAULT '{}',
    docker_image    TEXT,
    docker_port     INTEGER DEFAULT 80,
    is_visible      BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE solves (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    challenge_id UUID        NOT NULL REFERENCES challenges (id) ON DELETE CASCADE,
    solved_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, challenge_id)
);
CREATE INDEX solves_challenge_id_idx ON solves (challenge_id);

-- +goose Down
DROP TABLE solves;
DROP TABLE challenges;
DROP TABLE users;
DROP TABLE teams;
