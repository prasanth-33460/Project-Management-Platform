# Project Management Platform

A production-ready backend for a Jira-like project management system built with **Go + Fiber + PostgreSQL + Redis**.

---

## Architecture Overview

```
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
└──────┬───────────────────┬────────────────────── ┘
       │                   │
  PostgreSQL 16          Redis 7
  (data + FTS)         (Pub/Sub + Presence TTL)
```

### Key Design Decisions

| Concern | Choice | Rationale |
|---|---|---|
| HTTP Framework | Fiber v2 | Zero-alloc, Express-like API, built-in WS upgrade |
| Database | PostgreSQL 16 | JSONB custom fields, `tsvector` FTS, row-level locking |
| Cache/PubSub | Redis 7 | WS fan-out across instances, presence TTL keys |
| Concurrency | Optimistic locking via `version` column | No DB-level locks; 409 on conflict |
| Search | PostgreSQL FTS (`tsvector` + GIN index) | Avoids Elasticsearch complexity |
| Pagination | Cursor-based `(created_at, id)` | Stable under concurrent inserts |
| Auth | JWT HS256, 24h TTL | Stateless, no session store required |

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

# 3. Load seed data (optional)
docker exec -i <postgres-container> psql -U postgres projectmgmt < scripts/seed.sql
```

API available at `http://localhost:8080`

### Option B — Run locally

```bash
# 1. Copy env
cp .env.example .env  # Edit DATABASE_URL and REDIS_URL

# 2. Start dependencies
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
|--------|----------|-------------|
| `POST` | `/api/auth/register` | Register a new user |
| `POST` | `/api/auth/login` | Login, receive JWT token |

All other endpoints require `Authorization: Bearer <token>`.

### Projects

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/projects` | List user's projects |
| `POST` | `/api/projects` | Create project (auto-seeds workflow) |
| `GET` | `/api/projects/:id` | Get project details |
| `PATCH` | `/api/projects/:id` | Update project |
| `DELETE` | `/api/projects/:id` | Delete project |
| `GET` | `/api/projects/:id/board` | Get board state with issues |
| `GET` | `/api/projects/:id/backlog` | Get unassigned (backlog) issues |
| `POST` | `/api/projects/:id/issues` | Create issue |
| `GET` | `/api/projects/:id/activity` | Paginated activity feed |
| `GET` | `/api/projects/:id/workflow/statuses` | List workflow statuses |
| `POST` | `/api/projects/:id/workflow/statuses` | Add custom status |
| `POST` | `/api/projects/:id/workflow/transitions` | Define allowed transition |

### Issues

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/issues/:id` | Get issue with status, assignee, watchers |
| `PATCH` | `/api/issues/:id` | Update issue fields |
| `DELETE` | `/api/issues/:id` | Delete issue |
| `POST` | `/api/issues/:id/transitions` | **Transition status** (workflow enforced) |
| `POST` | `/api/issues/:id/watch` | Watch issue |
| `DELETE` | `/api/issues/:id/watch` | Unwatch issue |
| `GET` | `/api/issues/:id/comments` | List threaded comments |
| `POST` | `/api/issues/:id/comments` | Add comment (supports `parent_id` threading) |

### Sprints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/projects/:id/sprints` | List sprints |
| `POST` | `/api/projects/:id/sprints` | Create sprint |
| `GET` | `/api/sprints/:id` | Get sprint |
| `PATCH` | `/api/sprints/:id` | Update sprint |
| `DELETE` | `/api/sprints/:id` | Delete sprint (not if active) |
| `POST` | `/api/sprints/:id/start` | Start sprint (enforces 1 active sprint) |
| `POST` | `/api/sprints/:id/complete` | Complete sprint (surfaces incomplete, carry-over) |
| `POST` | `/api/sprints/:id/move-issue` | Move issue into sprint |
| `POST` | `/api/sprints/move-to-backlog` | Move issue to backlog |

### Search

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/search` | Full-text + structured search across issues |

Query params: `q`, `project_id`, `status_id`, `assignee_id`, `priority`, `type`, `cursor`, `limit`

### Notifications

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/notifications` | List notifications (paginated) |
| `POST` | `/api/notifications/:id/read` | Mark one read |
| `POST` | `/api/notifications/read-all` | Mark all read |

### WebSocket

```
ws://localhost:8080/api/ws?user_id=<uuid>&project_id=<uuid>
```

Optional: add `&issue_id=<uuid>` to also subscribe to a specific issue room.

**Event types received:**
- `issue_created` — new issue in the project
- `issue_updated` — field or status changed
- `issue_moved` — issue moved between sprints
- `issue_deleted` — issue deleted
- `comment_added` — new comment on an issue
- `sprint_updated` — sprint started / completed / modified
- `presence_update` — who is currently viewing the board

**Heartbeat** — send `{"type":"heartbeat"}` every 25s; server replies with `{"type":"heartbeat_ack"}`.

---

## Workflow Engine — Transition Rules

Creating a project auto-seeds this workflow:

```
To Do ──► In Progress ──► In Review ──► Done
            ◄──────────────────
```

Attempting an illegal transition (e.g. `To Do → Done`) returns:

```json
HTTP 422
{
  "message": "transition not allowed",
  "details": {
    "message": "cannot move from \"To Do\" directly to the requested status",
    "allowed_transitions": [
      { "id": "...", "name": "In Progress", "color": "#3B82F6" }
    ]
  }
}
```

---

## Concurrent Update Handling

Issues carry a `version` integer (optimistic lock). The transition endpoint issues:

```sql
UPDATE issues
SET status_id = $1, version = version + 1
WHERE id = $2 AND version = $3   -- exact version match
```

If `rows_affected == 0`, another writer won the race — the API returns **HTTP 409** and the client retries.

---

## Database Schema

```
users ─────────────────────────────────────────────────────────┐
  │                                                             │
projects ──► project_members                                    │
  │                                                             │
  ├──► workflow_statuses ──► workflow_transitions               │
  │                                                             │
  ├──► sprints                                                  │
  │                                                             │
  └──► issues (self-ref parent_id: epic→story→subtask)         │
         │                                                      │
         ├──► comments (threaded via parent_id)                │
         ├──► watchers                                          │
         ├──► activity_log                                      │
         └──► issue_custom_field_values ──► custom_field_defs  │
                                                                │
notifications ─────────────────────────────────────────────────┘
```

Key indexes:
- `GIN(issues.search_vector)` — full-text search
- `GIN(issues.labels)` — label filtering
- `(project_id, created_at DESC)` on activity_log — feed queries
- Partial index on `notifications WHERE read = FALSE` — unread count

---

## Trade-offs

**Optimized for:** correctness, clean code structure, demonstrability.

**What I'd add with more time:**
- Rate limiting per user (Redis token bucket)
- Refresh token rotation
- Cursor-based pagination on all list endpoints (currently issues + activity feed)
- E2E tests with `testcontainers-go`
- OpenAPI/Swagger via `swaggo`
- Custom field value validation per field type
- Webhook delivery for external integrations
- Full @mention resolution in comments
