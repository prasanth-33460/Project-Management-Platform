# Project Management Platform

A production-ready backend for a Jira-like project management system built with **Go + Fiber + PostgreSQL + Redis**.

**Live demo:** `https://project-management-platform-x0ml.onrender.com`  
**API docs (Swagger UI):** `https://project-management-platform-x0ml.onrender.com/docs`  
**Full curl walkthrough:** [docs/README.md](docs/README.md)

---

## Architecture Overview

```text
Client (HTTP / WebSocket)
        │
┌───────▼──────────────────────────────────────────┐
│              Fiber HTTP Router                    │
│  JWT Auth Middleware  │  CORS  │  Error Handler   │
└──────┬───────────────┬────────┬──────────────────┘
       │               │        │
  REST APIs        WebSocket   /health
       │            Hub (Redis Pub/Sub)
┌──────▼──────────────────────────────────────────┐
│                  Service Layer                   │
│  AuthService  │ IssueService │ WorkflowEngine    │
│  SprintService│ Collaboration│ NotificationSvc   │
└──────┬──────────────────────────────────────────┘
       │
┌──────▼──────────────────────────────────────────┐
│               Repository Layer                   │
│  Raw SQL via pgx v5  │  Connection Pool (30 max) │
└──────┬───────────────────┬──────────────────────┘
       │                   │
  PostgreSQL 16          Redis 7
  (data + FTS)         (Pub/Sub + Presence TTL)
```

### Request lifecycle

1. **Handler** — parses and validates the HTTP request, extracts JWT claims, calls the service.
2. **Service** — enforces business rules (workflow transitions, hierarchy constraints, optimistic locking), coordinates transactions, fires WebSocket events and notifications.
3. **Repository** — executes raw SQL via `pgxpool`. All write-heavy operations that need atomicity go through the `Transactor` interface (`WithTx`), keeping `pgx.Tx` out of the service layer entirely.

---

## Architecture Decisions

### Why Go + Fiber?

Go's goroutine model handles concurrent WebSocket connections cheaply. Fiber's zero-alloc routing and built-in WebSocket upgrade (via `fasthttp`) avoided the impedance mismatch that comes from layering gorilla/websocket on top of `net/http`. The Express-style API also keeps handlers readable without ceremony.

### Why raw SQL (pgx) instead of an ORM?

The domain has several non-trivial queries: keyset pagination joining four tables, FTS with `tsvector` + `GIN`, JSONB reads for custom field values, and the optimistic-lock `UPDATE ... WHERE version=$3`. An ORM would fight all of these. Raw SQL makes the queries explicit and the indexes predictable.

### Why optimistic locking instead of `SELECT FOR UPDATE`?

`SELECT FOR UPDATE` serialises all transition requests behind a row lock. Optimistic locking (`version` column) lets concurrent reads proceed freely and only conflicts on the actual write — the right tradeoff for a system where reads vastly outnumber writes and conflicts are rare. When they do happen the client retries, which is a natural fit for frontend state machines.

### Why the WorkflowEngine is a separate service

`IssueService.Update` does field patching; `WorkflowEngine.Transition` enforces the state machine. Keeping them separate means transition logic (graph lookup, JSONB validation rules, auto-actions) has one home and can be unit-tested independently of generic CRUD.

### Why Redis for WebSocket fan-out instead of in-process channels?

An in-process channel would break the moment a second app instance starts. Redis pub/sub lets any instance publish to a room and every other instance's Hub forwards the event to its local connections. The sorted-set replay store (`ws:replay:{room}`) gives disconnected clients a catch-up window without a second database.

### Why cursor-based pagination?

`OFFSET N` on large tables requires scanning N rows every time. Keyset pagination (`WHERE (created_at, id) < ($cursor_ts, $cursor_id)`) uses the existing `(project_id, created_at DESC, id DESC)` index and remains constant-time regardless of page depth. Stable under concurrent inserts too — a row inserted before the cursor never shifts results behind it.

### Why PostgreSQL FTS instead of Elasticsearch?

For a take-home scope with one database server, `tsvector` + GIN avoids a separate search cluster, operational overhead, and eventual-consistency lag. The search vector is maintained by a DB trigger and covers issue title, description, and comment bodies via a joined subquery.

---

## Database Schema (ERD)

```text
users
  id            UUID PK
  email         TEXT UNIQUE
  name          TEXT
  handle        TEXT UNIQUE   -- derived @mention handle
  password_hash TEXT
  created_at    TIMESTAMPTZ

projects
  id            UUID PK
  name          TEXT
  key           TEXT UNIQUE   -- e.g. PLT
  description   TEXT
  issue_counter INT           -- monotonic key counter (PLT-1, PLT-2 …)
  created_at    TIMESTAMPTZ

project_members
  project_id    UUID FK → projects
  user_id       UUID FK → users
  role          TEXT          -- admin | member

workflow_statuses
  id            UUID PK
  project_id    UUID FK → projects
  name          TEXT
  color         TEXT
  position      INT
  is_default    BOOL
  is_done       BOOL          -- counts toward sprint velocity

workflow_transitions
  id            UUID PK
  project_id    UUID FK → projects
  from_status_id UUID FK → workflow_statuses
  to_status_id  UUID FK → workflow_statuses
  validation_rules JSONB      -- [{field, operator, message}, …]
  auto_actions     JSONB      -- [{type, field, value}, …]

sprints
  id            UUID PK
  project_id    UUID FK → projects
  name          TEXT
  goal          TEXT
  status        TEXT          -- planned | active | completed
  start_date    DATE
  end_date      DATE
  velocity      INT           -- sum of completed story_points
  created_at    TIMESTAMPTZ

issues
  id            UUID PK
  project_id    UUID FK → projects
  issue_key     TEXT UNIQUE   -- PLT-N
  sprint_id     UUID FK → sprints (nullable)
  parent_id     UUID FK → issues (self-ref, nullable)
  type          TEXT          -- epic | story | task | bug | subtask
  title         TEXT
  description   TEXT
  status_id     UUID FK → workflow_statuses
  priority      TEXT          -- critical | high | medium | low
  assignee_id   UUID FK → users (nullable)
  reporter_id   UUID FK → users
  story_points  INT (nullable)
  labels        TEXT[]
  version       INT           -- optimistic lock counter
  search_vector TSVECTOR      -- maintained by trigger
  created_at    TIMESTAMPTZ
  updated_at    TIMESTAMPTZ

comments
  id            UUID PK
  issue_id      UUID FK → issues
  author_id     UUID FK → users
  parent_id     UUID FK → comments (self-ref, nullable — one level deep)
  body          TEXT
  created_at    TIMESTAMPTZ

issue_watchers
  issue_id      UUID FK → issues
  user_id       UUID FK → users
  PRIMARY KEY (issue_id, user_id)

activity_log
  id            UUID PK
  issue_id      UUID FK → issues (nullable)
  project_id    UUID FK → projects
  actor_id      UUID FK → users
  event_type    TEXT          -- issue_created | status_changed | field_updated | …
  field_name    TEXT (nullable)
  old_value     TEXT (nullable)
  new_value     TEXT (nullable)
  created_at    TIMESTAMPTZ

custom_field_defs
  id            UUID PK
  project_id    UUID FK → projects
  name          TEXT
  type          TEXT          -- text | number | select
  options       TEXT[]        -- valid values for select fields

issue_custom_field_values
  issue_id      UUID FK → issues
  field_id      UUID FK → custom_field_defs
  value         TEXT
  PRIMARY KEY (issue_id, field_id)

notifications
  id            UUID PK
  user_id       UUID FK → users
  type          TEXT          -- assigned | mentioned | status_changed | comment_added
  ref_id        UUID (nullable)
  ref_type      TEXT (nullable)
  title         TEXT
  read          BOOL DEFAULT FALSE
  created_at    TIMESTAMPTZ
```

**Key indexes:**

| Index | Type | Purpose |
| ----- | ---- | ------- |
| `issues.search_vector` | GIN | Full-text search |
| `issues.labels` | GIN | Label array filtering |
| `(project_id, created_at DESC, id DESC)` on `activity_log` | B-Tree | Feed pagination |
| `notifications WHERE read = FALSE` | Partial | Unread badge count |
| `(project_id, status)` on `sprints` | B-Tree | `GetActiveSprint` lookup |

---

## Local Development

### Prerequisites
- Docker & Docker Compose
- Go 1.22+

### Option A — Docker Compose (recommended)

```bash
# 1. Clone & enter
git clone <repo-url> && cd Project-Management-Platform

# 2. Start all services (PostgreSQL + Redis + App)
docker-compose up --build

# 3. (Optional) Seed demo data
docker exec -i <postgres-container> psql -U postgres projectmgmt < scripts/seed.sql
```

API available at `http://localhost:8080`.

### Option B — Run locally

```bash
# 1. Copy and edit environment variables
cp .env.example .env   # set DATABASE_URL, REDIS_URL, JWT_SECRET

# 2. Start only the backing services
docker-compose up -d postgres redis

# 3. Run migrations
psql $DATABASE_URL -f migrations/000001_init.up.sql

# 4. (Optional) Seed data
psql $DATABASE_URL -f scripts/seed.sql

# 5. Run the server
go run .
```

---

## API Reference

### Authentication

| Method | Endpoint | Description |
| ------ | -------- | ----------- |
| `POST` | `/api/auth/register` | Register — derives `@handle` from name |
| `POST` | `/api/auth/login` | Login, returns JWT (24h) |

All other endpoints require `Authorization: Bearer <token>`.

### Projects

| Method | Endpoint | Description |
| ------ | -------- | ----------- |
| `GET` | `/api/projects` | List user's projects |
| `POST` | `/api/projects` | Create project (auto-seeds workflow) |
| `GET` | `/api/projects/:id` | Get project details |
| `PATCH` | `/api/projects/:id` | Update project |
| `DELETE` | `/api/projects/:id` | Delete project |
| `POST` | `/api/projects/:id/members` | Add member |
| `GET` | `/api/projects/:id/board` | Kanban board state |
| `GET` | `/api/projects/:id/backlog` | Issues not in any sprint |
| `POST` | `/api/projects/:id/issues` | Create issue |
| `GET` | `/api/projects/:id/activity` | Paginated activity feed |
| `GET` | `/api/projects/:id/sprints` | List sprints |
| `GET` | `/api/projects/:id/workflow/statuses` | List workflow statuses |
| `POST` | `/api/projects/:id/workflow/statuses` | Add custom status |
| `GET` | `/api/projects/:id/workflow/transitions` | List allowed transitions |
| `POST` | `/api/projects/:id/workflow/transitions` | Define allowed transition |
| `POST` | `/api/projects/:id/workflow/transitions/:tid/rules` | Add validation rule |
| `POST` | `/api/projects/:id/workflow/transitions/:tid/actions` | Add auto-action |
| `GET` | `/api/projects/:id/custom-fields` | List custom field definitions |
| `POST` | `/api/projects/:id/custom-fields` | Define a custom field |

### Issues

| Method | Endpoint | Description |
| ------ | -------- | ----------- |
| `GET` | `/api/issues/:id` | Get issue (includes watchers, custom fields) |
| `PATCH` | `/api/issues/:id` | Update fields |
| `DELETE` | `/api/issues/:id` | Delete issue |
| `POST` | `/api/issues/:id/transitions` | **Transition status** (workflow enforced) |
| `POST` | `/api/issues/:id/watch` | Watch issue |
| `DELETE` | `/api/issues/:id/watch` | Unwatch issue |
| `GET` | `/api/issues/:id/comments` | List threaded comments |
| `POST` | `/api/issues/:id/comments` | Add comment (`parent_id` for threading) |
| `PUT` | `/api/issues/:id/custom-fields/:fid` | Set custom field value |

### Sprints

| Method | Endpoint | Description |
| ------ | -------- | ----------- |
| `POST` | `/api/projects/:id/sprints` | Create sprint |
| `GET` | `/api/sprints/:id` | Get sprint |
| `PATCH` | `/api/sprints/:id` | Update sprint |
| `DELETE` | `/api/sprints/:id` | Delete sprint (not if active) |
| `POST` | `/api/sprints/:id/start` | Start (enforces 1 active sprint) |
| `POST` | `/api/sprints/:id/complete` | Complete with carry-over |
| `POST` | `/api/sprints/:id/move-issue` | Move issue into sprint |
| `POST` | `/api/sprints/move-to-backlog` | Move issue to backlog |

### Search

| Method | Endpoint | Description |
| ------ | -------- | ----------- |
| `GET` | `/api/search` | FTS + structured search across issues |

Query params: `q`, `project_id`, `status_id`, `assignee_id`, `priority`, `type`, `cursor`, `limit`

### Notifications

| Method | Endpoint | Description |
| ------ | -------- | ----------- |
| `GET` | `/api/notifications` | List (paginated) |
| `POST` | `/api/notifications/:id/read` | Mark one read |
| `POST` | `/api/notifications/read-all` | Mark all read |

### WebSocket

```
ws://host/api/ws?user_id=<uuid>&project_id=<uuid>[&issue_id=<uuid>]
```

Event types: `issue_created`, `issue_updated`, `issue_moved`, `issue_deleted`, `comment_added`, `sprint_updated`, `presence_update`.  
Send `{"type":"heartbeat"}` every 25s; server replies `{"type":"heartbeat_ack"}`.

---

## Workflow Engine

Creating a project auto-seeds this transition graph:

```
To Do ──► In Progress ──► In Review ──► Done
                ◄──────────────────
```

### Transition enforcement

```json
POST /api/issues/:id/transitions
{ "target_status_id": "<uuid>" }
```

- **422** if the target has no edge from the current status
- **422** if a validation rule blocks it (e.g. `assignee_id must not be empty`)
- **409** on optimistic lock conflict (another writer won the race)

Illegal transition response:
```json
{
  "message": "transition not allowed",
  "details": {
    "message": "cannot move from \"To Do\" directly to the requested status",
    "allowed_transitions": [{ "id": "...", "name": "In Progress", "color": "#3B82F6" }]
  }
}
```

### Validation rules (JSONB)

Stored per-transition. Evaluated before touching the DB.

```json
{ "field": "assignee_id", "operator": "not_empty", "message": "Assign before review" }
```

Supported fields: `assignee_id`, `story_points`, `description`.  
Supported operators: `not_empty`, `is_empty`.

### Auto-actions (JSONB)

Fire after a successful transition commit.

```json
{ "type": "assign_field", "field": "assignee_id", "value": "<user-uuid>" }
```

---

## Concurrent Update Handling

Every issue carries a `version` integer. The transition SQL:

```sql
UPDATE issues
SET status_id = $1, version = version + 1, updated_at = NOW()
WHERE id = $2 AND version = $3
```

`rows_affected == 0` means another writer already bumped the version. The API returns **HTTP 409**. The client re-fetches the current state and retries.

---

## Trade-offs

**Optimized for:** correctness, explicitness, and demonstrability.

| Decision | Benefit | Cost |
| -------- | ------- | ---- |
| Raw SQL over ORM | Full control over complex queries (FTS, keyset pagination, JSONB) | More boilerplate for simple CRUD |
| Optimistic locking over `SELECT FOR UPDATE` | No row-level contention under normal load | Clients must handle 409 and retry |
| PostgreSQL FTS over Elasticsearch | Single infrastructure component | Less relevance tuning, no distributed search |
| Redis pub/sub for WS fan-out | Works correctly with multiple app instances | Adds Redis as a required dependency |
| Cursor pagination over OFFSET | Constant-time regardless of page depth | Cursor state must be passed per request |
| JWT stateless auth | No session store required | No server-side invalidation (logout is client-side) |

**What I'd add with more time:**
- Rate limiting per user (Redis token bucket)
- Refresh token rotation with a short-lived access token
- Cursor-based pagination on all list endpoints (currently issues + activity feed only)
- Integration tests with `testcontainers-go`
- Custom field value validation per type at write time
- Webhook delivery for external integrations
- Board column reordering persisted per-user
