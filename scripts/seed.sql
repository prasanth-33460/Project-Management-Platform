-- =============================================================
-- Seed Data for Project Management Platform Demo
-- Run AFTER migrations/000001_init.up.sql
-- =============================================================

-- ── Users ──────────────────────────────────────────────────────
-- Passwords are bcrypt of "password123"
INSERT INTO users (id, email, display_name, password_hash) VALUES
  ('11111111-1111-1111-1111-111111111111', 'alice@example.com',  'Alice Johnson', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy'),
  ('22222222-2222-2222-2222-222222222222', 'bob@example.com',    'Bob Chen',      '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy'),
  ('33333333-3333-3333-3333-333333333333', 'carol@example.com',  'Carol Davis',   '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy')
ON CONFLICT DO NOTHING;

-- ── Project ─────────────────────────────────────────────────────
INSERT INTO projects (id, key, name, description, lead_user_id) VALUES
  ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'DEMO', 'Demo Project',
   'A sample project to showcase the platform features.', '11111111-1111-1111-1111-111111111111')
ON CONFLICT DO NOTHING;

INSERT INTO project_members (project_id, user_id, role) VALUES
  ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '11111111-1111-1111-1111-111111111111', 'admin'),
  ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '22222222-2222-2222-2222-222222222222', 'member'),
  ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', '33333333-3333-3333-3333-333333333333', 'member')
ON CONFLICT DO NOTHING;

-- ── Workflow Statuses ───────────────────────────────────────────
INSERT INTO workflow_statuses (id, project_id, name, color, position, is_default, is_done) VALUES
  ('s1000000-0000-0000-0000-000000000000', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'To Do',      '#6B7280', 0, TRUE,  FALSE),
  ('s2000000-0000-0000-0000-000000000000', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'In Progress','#3B82F6', 1, FALSE, FALSE),
  ('s3000000-0000-0000-0000-000000000000', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'In Review',  '#F59E0B', 2, FALSE, FALSE),
  ('s4000000-0000-0000-0000-000000000000', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'Done',       '#10B981', 3, FALSE, TRUE)
ON CONFLICT DO NOTHING;

-- Transitions: To Do→In Progress, In Progress→In Review, In Review→Done
--              In Review→In Progress, In Progress→To Do (reversals)
INSERT INTO workflow_transitions (project_id, from_status_id, to_status_id, auto_actions) VALUES
  ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 's1000000-0000-0000-0000-000000000000', 's2000000-0000-0000-0000-000000000000', '[]'),
  ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 's2000000-0000-0000-0000-000000000000', 's3000000-0000-0000-0000-000000000000', '[]'),
  ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 's3000000-0000-0000-0000-000000000000', 's4000000-0000-0000-0000-000000000000', '[]'),
  ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 's3000000-0000-0000-0000-000000000000', 's2000000-0000-0000-0000-000000000000', '[]'),
  ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 's2000000-0000-0000-0000-000000000000', 's1000000-0000-0000-0000-000000000000', '[]')
ON CONFLICT DO NOTHING;

-- ── Sprints ─────────────────────────────────────────────────────
INSERT INTO sprints (id, project_id, name, goal, start_date, end_date, status) VALUES
  ('b1000000-0000-0000-0000-000000000000', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
   'Sprint 1', 'Ship the auth and onboarding flow', '2024-01-15', '2024-01-29', 'completed'),
  ('b2000000-0000-0000-0000-000000000000', 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
   'Sprint 2', 'Complete dashboard and reporting', '2024-01-29', '2024-02-12', 'active')
ON CONFLICT DO NOTHING;

-- Update project issue counter
UPDATE projects SET issue_counter = 10 WHERE id = 'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa';

-- ── Issues ──────────────────────────────────────────────────────
INSERT INTO issues (id, project_id, issue_key, sprint_id, type, title, description,
                    status_id, priority, assignee_id, reporter_id, story_points, labels) VALUES
  -- Epic
  ('c1000000-0000-0000-0000-000000000000',
   'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'DEMO-1',
   NULL, 'epic', 'User Authentication System',
   'Implement complete OAuth 2.0 + JWT authentication flow.',
   's1000000-0000-0000-0000-000000000000', 'high',
   '11111111-1111-1111-1111-111111111111', '11111111-1111-1111-1111-111111111111',
   NULL, ARRAY['auth', 'backend']),

  -- Stories under the epic
  ('c2000000-0000-0000-0000-000000000000',
   'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'DEMO-2',
   'b2000000-0000-0000-0000-000000000000', 'story',
   'Add user registration endpoint',
   'POST /api/auth/register — validate email, hash password, return JWT.',
   's2000000-0000-0000-0000-000000000000', 'high',
   '22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111',
   5, ARRAY['auth', 'api']),

  ('c3000000-0000-0000-0000-000000000000',
   'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'DEMO-3',
   'b2000000-0000-0000-0000-000000000000', 'story',
   'Add login endpoint with JWT',
   'POST /api/auth/login — verify credentials, return signed JWT.',
   's3000000-0000-0000-0000-000000000000', 'high',
   '22222222-2222-2222-2222-222222222222', '11111111-1111-1111-1111-111111111111',
   3, ARRAY['auth', 'api']),

  -- Bug
  ('c4000000-0000-0000-0000-000000000000',
   'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'DEMO-4',
   'b2000000-0000-0000-0000-000000000000', 'bug',
   'JWT token not expiring correctly',
   'Tokens issued before the secret rotation are still valid.',
   's1000000-0000-0000-0000-000000000000', 'critical',
   '33333333-3333-3333-3333-333333333333', '22222222-2222-2222-2222-222222222222',
   2, ARRAY['auth', 'security']),

  -- Tasks in backlog
  ('c5000000-0000-0000-0000-000000000000',
   'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'DEMO-5',
   NULL, 'task', 'Set up CI/CD pipeline',
   'Configure GitHub Actions for lint, test, and Docker build.',
   's1000000-0000-0000-0000-000000000000', 'medium',
   NULL, '11111111-1111-1111-1111-111111111111',
   3, ARRAY['devops']),

  ('c6000000-0000-0000-0000-000000000000',
   'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 'DEMO-6',
   'b2000000-0000-0000-0000-000000000000', 'story',
   'Build sprint velocity dashboard',
   'Chart showing story points completed per sprint.',
   's1000000-0000-0000-0000-000000000000', 'medium',
   '33333333-3333-3333-3333-333333333333', '11111111-1111-1111-1111-111111111111',
   8, ARRAY['dashboard', 'frontend'])
ON CONFLICT DO NOTHING;

-- ── Watchers ────────────────────────────────────────────────────
INSERT INTO watchers (issue_id, user_id) VALUES
  ('c2000000-0000-0000-0000-000000000000', '11111111-1111-1111-1111-111111111111'),
  ('c2000000-0000-0000-0000-000000000000', '22222222-2222-2222-2222-222222222222'),
  ('c3000000-0000-0000-0000-000000000000', '11111111-1111-1111-1111-111111111111'),
  ('c4000000-0000-0000-0000-000000000000', '11111111-1111-1111-1111-111111111111'),
  ('c4000000-0000-0000-0000-000000000000', '33333333-3333-3333-3333-333333333333')
ON CONFLICT DO NOTHING;

-- ── Comments ────────────────────────────────────────────────────
INSERT INTO comments (issue_id, author_id, body) VALUES
  ('c2000000-0000-0000-0000-000000000000', '22222222-2222-2222-2222-222222222222',
   'Started working on this. Will need to validate email format and uniqueness.'),
  ('c4000000-0000-0000-0000-000000000000', '33333333-3333-3333-3333-333333333333',
   'Reproduced in staging. Root cause: exp claim is set to Unix 0 when secret is empty string.'),
  ('c4000000-0000-0000-0000-000000000000', '11111111-1111-1111-1111-111111111111',
   '@carol can you send a PR with the fix? This is blocking the release.')
ON CONFLICT DO NOTHING;

-- ── Activity Log ─────────────────────────────────────────────────
INSERT INTO activity_log (issue_id, project_id, actor_id, event_type, field_name, old_value, new_value) VALUES
  ('c2000000-0000-0000-0000-000000000000',
   'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
   '11111111-1111-1111-1111-111111111111',
   'issue_created', NULL, NULL, 'DEMO-2'),

  ('c2000000-0000-0000-0000-000000000000',
   'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
   '22222222-2222-2222-2222-222222222222',
   'status_changed', 'status', 'To Do', 'In Progress'),

  ('c3000000-0000-0000-0000-000000000000',
   'aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa',
   '22222222-2222-2222-2222-222222222222',
   'status_changed', 'status', 'In Progress', 'In Review')
ON CONFLICT DO NOTHING;
