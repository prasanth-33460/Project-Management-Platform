-- @mention handles — derived from display_name at registration
ALTER TABLE users ADD COLUMN IF NOT EXISTS handle VARCHAR(50);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_handle ON users(handle) WHERE handle IS NOT NULL;

-- validation rules on transitions
-- e.g. [{"field":"assignee_id","operator":"not_empty","message":"Assignee required before In Review"}]
ALTER TABLE workflow_transitions
    ADD COLUMN IF NOT EXISTS validation_rules JSONB NOT NULL DEFAULT '[]';

-- GIN-indexed search vector on comment bodies
ALTER TABLE comments
    ADD COLUMN IF NOT EXISTS search_vector tsvector
    GENERATED ALWAYS AS (to_tsvector('english', coalesce(body, ''))) STORED;

CREATE INDEX IF NOT EXISTS idx_comments_search ON comments USING GIN(search_vector);

-- track which migration files have been applied
CREATE TABLE IF NOT EXISTS schema_migrations (
    filename   TEXT        PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
