-- Migration: 001_initial_schema.sql (PostgreSQL)

-- Users
CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL PRIMARY KEY,
    username      VARCHAR(64)  NOT NULL UNIQUE,
    email         VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_email    ON users (email);
CREATE INDEX IF NOT EXISTS idx_users_username ON users (username);

-- Teams
CREATE TABLE IF NOT EXISTS teams (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(128) NOT NULL,
    description TEXT,
    created_by  BIGINT       NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_teams_created_by ON teams (created_by);

-- Team Members (many-to-many with role)
CREATE TYPE team_role AS ENUM ('owner', 'admin', 'member');

CREATE TABLE IF NOT EXISTS team_members (
    user_id   BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    team_id   BIGINT      NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    role      team_role   NOT NULL DEFAULT 'member',
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, team_id)
);

CREATE INDEX IF NOT EXISTS idx_tm_team_id ON team_members (team_id);
CREATE INDEX IF NOT EXISTS idx_tm_user_id ON team_members (user_id);

-- Task status and priority enums
CREATE TYPE task_status   AS ENUM ('todo', 'in_progress', 'done');
CREATE TYPE task_priority AS ENUM ('low', 'medium', 'high');

-- Tasks
CREATE TABLE IF NOT EXISTS tasks (
    id          BIGSERIAL     PRIMARY KEY,
    title       VARCHAR(255)  NOT NULL,
    description TEXT,
    status      task_status   NOT NULL DEFAULT 'todo',
    priority    task_priority NOT NULL DEFAULT 'medium',
    assignee_id BIGINT        REFERENCES users(id) ON DELETE SET NULL,
    team_id     BIGINT        NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    created_by  BIGINT        NOT NULL REFERENCES users(id),
    due_date    TIMESTAMPTZ,
    created_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tasks_team_id     ON tasks (team_id);
CREATE INDEX IF NOT EXISTS idx_tasks_assignee    ON tasks (assignee_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status      ON tasks (status);
CREATE INDEX IF NOT EXISTS idx_tasks_created_by  ON tasks (created_by);
CREATE INDEX IF NOT EXISTS idx_tasks_team_status ON tasks (team_id, status);   -- composite for team+status filter
CREATE INDEX IF NOT EXISTS idx_tasks_created_at  ON tasks (created_at);        -- date-range analytics

-- Auto-update updated_at via trigger
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE TRIGGER trg_tasks_updated_at
BEFORE UPDATE ON tasks
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Task History (audit log)
CREATE TABLE IF NOT EXISTS task_history (
    id         BIGSERIAL   PRIMARY KEY,
    task_id    BIGINT      NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    changed_by BIGINT      NOT NULL REFERENCES users(id),
    field_name VARCHAR(64) NOT NULL,
    old_value  TEXT,
    new_value  TEXT,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_th_task_id    ON task_history (task_id);
CREATE INDEX IF NOT EXISTS idx_th_changed_at ON task_history (changed_at);

-- Task Comments
CREATE TABLE IF NOT EXISTS task_comments (
    id         BIGSERIAL   PRIMARY KEY,
    task_id    BIGINT      NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id    BIGINT      NOT NULL REFERENCES users(id),
    content    TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tc_task_id ON task_comments (task_id);
CREATE INDEX IF NOT EXISTS idx_tc_user_id ON task_comments (user_id);
