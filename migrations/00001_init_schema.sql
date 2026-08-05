-- +goose Up
CREATE TABLE users (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    telegram_id bigint      NOT NULL UNIQUE,
    chat_id     bigint      NOT NULL,
    last_seen   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE refresh_sessions (
    id                 bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id            bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    access_token_hash  bytea       NOT NULL UNIQUE,
    refresh_token_hash bytea       NOT NULL UNIQUE,
    created_at         timestamptz NOT NULL DEFAULT now(),
    expired_at         timestamptz NOT NULL
);

-- Refreshing inserts the new session before deleting the old one, so a user may
-- briefly own two rows and user_id must not be unique.
CREATE INDEX refresh_sessions_user_id_idx ON refresh_sessions (user_id);

CREATE TABLE tasks (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id     bigint  NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name        text    NOT NULL,
    description text,
    date        timestamptz,
    notify_at   timestamptz,
    completed   boolean NOT NULL DEFAULT false,
    deleted     boolean NOT NULL DEFAULT false
);

CREATE INDEX tasks_user_date_idx ON tasks (user_id, date) WHERE NOT deleted;

CREATE INDEX tasks_notify_at_idx ON tasks (notify_at)
    WHERE notify_at IS NOT NULL AND NOT completed AND NOT deleted;

CREATE TABLE notes (
    id      bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    task_id bigint      NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
    text    text        NOT NULL,
    date    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX notes_task_id_idx ON notes (task_id);

CREATE TABLE tags (
    id      bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name    text   NOT NULL,
    user_id bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    UNIQUE (user_id, name)
);

CREATE TABLE task_tags (
    task_id bigint NOT NULL REFERENCES tasks (id) ON DELETE CASCADE,
    tag_id  bigint NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    PRIMARY KEY (task_id, tag_id)
);

CREATE INDEX task_tags_tag_id_idx ON task_tags (tag_id);

-- +goose Down
DROP TABLE task_tags;
DROP TABLE tags;
DROP TABLE notes;
DROP TABLE tasks;
DROP TABLE refresh_sessions;
DROP TABLE users;
