-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- ============================================================
-- USERS
-- ============================================================
CREATE TABLE users (
    id           UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    email        VARCHAR(255) NOT NULL UNIQUE,
    display_name VARCHAR(100) NOT NULL,
    avatar_url   TEXT,
    password_hash TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- PROJECTS
-- ============================================================
CREATE TABLE projects (
    id            UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    key           VARCHAR(10) NOT NULL UNIQUE,  -- e.g. "PROJ"
    name          VARCHAR(100) NOT NULL,
    description   TEXT,
    lead_user_id  UUID REFERENCES users(id) ON DELETE SET NULL,
    issue_counter INT NOT NULL DEFAULT 0,       -- auto-increment per project
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE project_members (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       VARCHAR(20) NOT NULL DEFAULT 'member', -- admin | member | viewer
    joined_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, user_id)
);

-- ============================================================
-- WORKFLOW (statuses + transitions per project)
-- ============================================================
CREATE TABLE workflow_statuses (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name       VARCHAR(50) NOT NULL,
    color      VARCHAR(7)  NOT NULL DEFAULT '#6B7280',
    position   INT         NOT NULL DEFAULT 0,
    is_default BOOLEAN     NOT NULL DEFAULT FALSE,  -- assigned to new issues
    is_done    BOOLEAN     NOT NULL DEFAULT FALSE,  -- counts toward velocity
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE workflow_transitions (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id     UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    from_status_id UUID NOT NULL REFERENCES workflow_statuses(id) ON DELETE CASCADE,
    to_status_id   UUID NOT NULL REFERENCES workflow_statuses(id) ON DELETE CASCADE,
    -- auto_actions: array of {type, payload} executed on transition
    -- e.g. [{"type":"assign_field","field":"assignee_id","value":"reviewer_user_id"}]
    auto_actions   JSONB NOT NULL DEFAULT '[]',
    UNIQUE (project_id, from_status_id, to_status_id)
);

-- ============================================================
-- SPRINTS
-- ============================================================
CREATE TABLE sprints (
    id         UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID         NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name       VARCHAR(100) NOT NULL,
    goal       TEXT,
    start_date DATE,
    end_date   DATE,
    status     VARCHAR(20)  NOT NULL DEFAULT 'planned', -- planned | active | completed
    velocity   INT,                                      -- story points completed
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ============================================================
-- ISSUES
-- ============================================================
CREATE TABLE issues (
    id           UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id   UUID         NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    issue_key    VARCHAR(20)  NOT NULL UNIQUE,   -- e.g. "PROJ-123"
    sprint_id    UUID         REFERENCES sprints(id) ON DELETE SET NULL,
    parent_id    UUID         REFERENCES issues(id) ON DELETE SET NULL,
    type         VARCHAR(20)  NOT NULL,          -- epic | story | task | bug | subtask
    title        VARCHAR(500) NOT NULL,
    description  TEXT         NOT NULL DEFAULT '',
    status_id    UUID         NOT NULL REFERENCES workflow_statuses(id),
    priority     VARCHAR(20)  NOT NULL DEFAULT 'medium', -- low | medium | high | critical
    assignee_id  UUID         REFERENCES users(id) ON DELETE SET NULL,
    reporter_id  UUID         NOT NULL REFERENCES users(id),
    story_points INT,
    labels       TEXT[]       NOT NULL DEFAULT '{}',
    version      INT          NOT NULL DEFAULT 0,  -- optimistic lock counter
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    -- GIN-indexed full-text search vector (auto-updated)
    search_vector tsvector GENERATED ALWAYS AS (
        setweight(to_tsvector('english', coalesce(title, '')),       'A') ||
        setweight(to_tsvector('english', coalesce(description, '')), 'B')
    ) STORED
);

-- Custom field definitions per project
CREATE TABLE custom_field_definitions (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name       VARCHAR(100) NOT NULL,
    field_type VARCHAR(20)  NOT NULL, -- text | number | dropdown | date
    options    JSONB        NOT NULL DEFAULT '[]', -- for dropdown type
    required   BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE issue_custom_field_values (
    issue_id UUID NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    field_id UUID NOT NULL REFERENCES custom_field_definitions(id) ON DELETE CASCADE,
    value    TEXT,
    PRIMARY KEY (issue_id, field_id)
);

-- ============================================================
-- COLLABORATION
-- ============================================================
CREATE TABLE comments (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    issue_id   UUID        NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    author_id  UUID        NOT NULL REFERENCES users(id),
    body       TEXT        NOT NULL,
    parent_id  UUID        REFERENCES comments(id) ON DELETE CASCADE,  -- threading
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE watchers (
    issue_id   UUID        NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (issue_id, user_id)
);

-- ============================================================
-- ACTIVITY LOG  (immutable audit trail)
-- ============================================================
CREATE TABLE activity_log (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    issue_id   UUID        REFERENCES issues(id) ON DELETE SET NULL,
    project_id UUID        NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    actor_id   UUID        NOT NULL REFERENCES users(id),
    event_type VARCHAR(50) NOT NULL, -- issue_created | status_changed | field_updated | comment_added | sprint_updated ...
    field_name VARCHAR(100),
    old_value  TEXT,
    new_value  TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- NOTIFICATIONS
-- ============================================================
CREATE TABLE notifications (
    id         UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       VARCHAR(50) NOT NULL, -- assigned | mentioned | status_changed | comment_added | sprint_updated
    ref_id     UUID,                 -- issue_id or comment_id
    ref_type   VARCHAR(20),          -- issue | comment
    title      TEXT        NOT NULL,
    body       TEXT,
    read       BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- INDEXES
-- ============================================================

-- Issues
CREATE INDEX idx_issues_project_id   ON issues(project_id);
CREATE INDEX idx_issues_sprint_id    ON issues(sprint_id);
CREATE INDEX idx_issues_assignee_id  ON issues(assignee_id);
CREATE INDEX idx_issues_status_id    ON issues(status_id);
CREATE INDEX idx_issues_parent_id    ON issues(parent_id);
CREATE INDEX idx_issues_type         ON issues(project_id, type);
CREATE INDEX idx_issues_priority     ON issues(project_id, priority);
CREATE INDEX idx_issues_created_at   ON issues(created_at DESC);
CREATE INDEX idx_issues_search       ON issues USING GIN(search_vector);
CREATE INDEX idx_issues_labels       ON issues USING GIN(labels);

-- Activity log
CREATE INDEX idx_activity_issue_id   ON activity_log(issue_id);
CREATE INDEX idx_activity_project    ON activity_log(project_id, created_at DESC);

-- Comments
CREATE INDEX idx_comments_issue_id   ON comments(issue_id, created_at ASC);

-- Notifications
CREATE INDEX idx_notif_user          ON notifications(user_id, created_at DESC);
CREATE INDEX idx_notif_unread        ON notifications(user_id) WHERE read = FALSE;

-- Workflow
CREATE INDEX idx_wf_transitions      ON workflow_transitions(project_id, from_status_id);
CREATE INDEX idx_wf_statuses         ON workflow_statuses(project_id, position);
