# API Walkthrough — Project Management Platform

Complete end-to-end guide covering every feature via `curl`. Commands are written to be run in sequence; each step captures IDs used by later steps.

**Live demo:** `https://project-management-platform-x0ml.onrender.com`

---

## Setup

```bash
BASE="https://project-management-platform-x0ml.onrender.com"
# For local Docker stack:
# BASE="http://localhost:8080"
```

---

## 1. Health Check

Verify the service is up before running anything else.

```bash
curl -s "$BASE/health" | jq .
```

Expected:
```json
{ "service": "project-management-platform", "status": "ok" }
```

---

## 2. Authentication

### Register two users

The platform derives a unique `@handle` from the display name (lowercase, spaces → `_`, non-alnum stripped). If the handle already exists, it retries with a random 4-digit suffix.

```bash
# Alice — will be the project admin
curl -s -X POST "$BASE/api/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"secret123","name":"Alice Smith"}' | jq .

# Bob — will be a team member
curl -s -X POST "$BASE/api/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"bob@example.com","password":"secret123","name":"Bob Jones"}' | jq .
```

Response includes `id`, `email`, `name`, and the derived `handle` (`alice_smith`, `bob_jones`).

### Login and capture tokens

```bash
ALICE_TOKEN=$(curl -s -X POST "$BASE/api/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"secret123"}' | jq -r '.token')

BOB_TOKEN=$(curl -s -X POST "$BASE/api/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"bob@example.com","password":"secret123"}' | jq -r '.token')

echo "Alice: $ALICE_TOKEN"
echo "Bob:   $BOB_TOKEN"
```

All subsequent requests use `Authorization: Bearer $ALICE_TOKEN` (or `$BOB_TOKEN`).

---

## 3. Project Setup

### Create a project

Creating a project automatically:
- Seeds four default workflow statuses: **To Do → In Progress → In Review → Done**
- Creates five allowed transitions (forward path + two reversal paths)
- Adds the creator as project `admin`

```bash
PROJECT_ID=$(curl -s -X POST "$BASE/api/projects" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Platform Team","key":"PLT","description":"Core platform work"}' \
  | jq -r '.id')

echo "Project: $PROJECT_ID"
```

The `key` is stored uppercase and becomes the prefix for issue keys (PLT-1, PLT-2, …).

### Add Bob as a member

```bash
curl -s -X POST "$BASE/api/projects/$PROJECT_ID/members" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"$BOB_ID\",\"role\":\"member\"}" | jq .
```

### Fetch the auto-seeded workflow statuses

```bash
curl -s "$BASE/api/projects/$PROJECT_ID/workflow/statuses" \
  -H "Authorization: Bearer $ALICE_TOKEN" | jq .
```

Capture the four status IDs for use in issue creation and transitions:

```bash
STATUSES=$(curl -s "$BASE/api/projects/$PROJECT_ID/workflow/statuses" \
  -H "Authorization: Bearer $ALICE_TOKEN")

TODO_ID=$(echo "$STATUSES"    | jq -r '.[] | select(.name=="To Do")      | .id')
INPROG_ID=$(echo "$STATUSES"  | jq -r '.[] | select(.name=="In Progress") | .id')
REVIEW_ID=$(echo "$STATUSES"  | jq -r '.[] | select(.name=="In Review")   | .id')
DONE_ID=$(echo "$STATUSES"    | jq -r '.[] | select(.name=="Done")        | .id')
```

---

## 4. Workflow — Validation Rules

Validation rules are stored as JSONB on a transition and checked before any DB write. This keeps business constraints in the workflow definition, not scattered across handlers.

### Add a rule: issue must have an assignee before moving In Progress → In Review

```bash
# First get the transition ID
TRANSITIONS=$(curl -s "$BASE/api/projects/$PROJECT_ID/workflow/transitions" \
  -H "Authorization: Bearer $ALICE_TOKEN")

TRANS_ID=$(echo "$TRANSITIONS" \
  | jq -r --arg from "$INPROG_ID" --arg to "$REVIEW_ID" \
    '.[] | select(.from_status_id==$from and .to_status_id==$to) | .id')

# Attach the validation rule
curl -s -X POST "$BASE/api/projects/$PROJECT_ID/workflow/transitions/$TRANS_ID/rules" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "field": "assignee_id",
    "operator": "not_empty",
    "message": "Assign someone before moving to review"
  }' | jq .
```

Supported fields: `assignee_id`, `story_points`, `description`. Supported operators: `not_empty`, `is_empty`.

---

## 5. Issue Hierarchy

The platform enforces a strict parent-child hierarchy:

```
Epic
 └── Story / Task / Bug
        └── Subtask
```

### Create an Epic

```bash
EPIC_ID=$(curl -s -X POST "$BASE/api/projects/$PROJECT_ID/issues" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "epic",
    "title": "User Authentication System",
    "description": "End-to-end auth flow",
    "priority": "high"
  }' | jq -r '.id')
```

### Create a Story under the Epic

```bash
STORY_ID=$(curl -s -X POST "$BASE/api/projects/$PROJECT_ID/issues" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"type\": \"story\",
    \"title\": \"JWT token issuance\",
    \"parent_id\": \"$EPIC_ID\",
    \"priority\": \"high\",
    \"story_points\": 5
  }" | jq -r '.id')
```

### Create a Task and a Bug

```bash
TASK_ID=$(curl -s -X POST "$BASE/api/projects/$PROJECT_ID/issues" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"type\": \"task\",
    \"title\": \"Write auth middleware\",
    \"parent_id\": \"$EPIC_ID\",
    \"priority\": \"medium\",
    \"story_points\": 3
  }" | jq -r '.id')

BUG_ID=$(curl -s -X POST "$BASE/api/projects/$PROJECT_ID/issues" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"type\": \"bug\",
    \"title\": \"Token expiry not enforced on refresh\",
    \"parent_id\": \"$EPIC_ID\",
    \"priority\": \"critical\"
  }" | jq -r '.id')
```

### Attempt an illegal hierarchy (should return 422)

A subtask cannot be a parent of another issue, and an epic cannot be a child of anything.

```bash
# Bug cannot be parent of a story — expect HTTP 422
curl -s -X POST "$BASE/api/projects/$PROJECT_ID/issues" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"type\": \"story\",
    \"title\": \"Invalid nesting\",
    \"parent_id\": \"$BUG_ID\"
  }" | jq .
```

---

## 6. Sprint Lifecycle

### Create a sprint

```bash
SPRINT_ID=$(curl -s -X POST "$BASE/api/projects/$PROJECT_ID/sprints" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Sprint 1",
    "goal": "Ship auth system",
    "start_date": "2025-05-01",
    "end_date": "2025-05-14"
  }' | jq -r '.id')
```

### Move issues into the sprint

```bash
curl -s -X POST "$BASE/api/sprints/$SPRINT_ID/move-issue" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"issue_id\":\"$STORY_ID\"}" | jq .

curl -s -X POST "$BASE/api/sprints/$SPRINT_ID/move-issue" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"issue_id\":\"$TASK_ID\"}" | jq .

curl -s -X POST "$BASE/api/sprints/$SPRINT_ID/move-issue" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"issue_id\":\"$BUG_ID\"}" | jq .
```

### Start the sprint

Only one sprint per project can be active at a time. Starting a second one while another is active returns 409.

```bash
curl -s -X POST "$BASE/api/sprints/$SPRINT_ID/start" \
  -H "Authorization: Bearer $ALICE_TOKEN" | jq .
```

### Complete an issue to generate velocity

```bash
# Assign the story first (required by our validation rule)
curl -s -X PATCH "$BASE/api/issues/$STORY_ID" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"assignee_id\":\"$BOB_ID\"}" | jq .

# Move through the transition chain: To Do → In Progress → In Review → Done
curl -s -X POST "$BASE/api/issues/$STORY_ID/transitions" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"target_status_id\":\"$INPROG_ID\"}" | jq .

curl -s -X POST "$BASE/api/issues/$STORY_ID/transitions" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"target_status_id\":\"$REVIEW_ID\"}" | jq .

curl -s -X POST "$BASE/api/issues/$STORY_ID/transitions" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"target_status_id\":\"$DONE_ID\"}" | jq .
```

### Complete the sprint with carry-over

`carry_over_issue_ids` moves those issues to `next_sprint_id` (or backlog if omitted). Issues not listed go straight to backlog.

```bash
curl -s -X POST "$BASE/api/sprints/$SPRINT_ID/complete" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"carry_over_issue_ids\": [\"$TASK_ID\"],
    \"next_sprint_id\": null
  }" | jq .
```

Response includes `sprint` (with `velocity = 5`), `completed_points`, and `incomplete_issues`.

---

## 7. Assignment Scenario A — Direct Assignment + Notification

When an issue is assigned to someone other than the reporter, they automatically:
1. Receive an in-app notification (type: `assigned`)
2. Get added as a watcher

```bash
# Alice assigns the bug to Bob
curl -s -X PATCH "$BASE/api/issues/$BUG_ID" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"assignee_id\":\"$BOB_ID\"}" | jq .

# Bob checks his notifications — should see "assigned" entry
curl -s "$BASE/api/notifications" \
  -H "Authorization: Bearer $BOB_TOKEN" | jq '.items[0]'
```

---

## 8. Assignment Scenario B — Auto-assign via Workflow Action

A transition can declare `auto_actions` that fire after the commit. Here we wire up "assign to Alice automatically when moved to In Review."

```bash
# Get the In Progress → In Review transition
TRANS_ID=$(curl -s "$BASE/api/projects/$PROJECT_ID/workflow/transitions" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  | jq -r --arg from "$INPROG_ID" --arg to "$REVIEW_ID" \
    '.[] | select(.from_status_id==$from and .to_status_id==$to) | .id')

# Add auto-assign action
curl -s -X POST "$BASE/api/projects/$PROJECT_ID/workflow/transitions/$TRANS_ID/actions" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"type\": \"assign_field\",
    \"field\": \"assignee_id\",
    \"value\": \"$ALICE_ID\"
  }" | jq .

# Now move the task through In Progress → In Review; Alice gets auto-assigned
curl -s -X POST "$BASE/api/issues/$TASK_ID/transitions" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"target_status_id\":\"$INPROG_ID\"}" | jq .

curl -s -X POST "$BASE/api/issues/$TASK_ID/transitions" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"target_status_id\":\"$REVIEW_ID\"}" | jq '.assignee_id'
# Should return Alice's ID
```

---

## 9. Assignment Scenario C — @mention in Comment

Mentioning `@handle` in a comment body creates a "mentioned" notification for that user (distinct from watcher notifications so they don't double-up).

```bash
curl -s -X POST "$BASE/api/issues/$BUG_ID/comments" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"body": "Hey @bob_jones — can you take a look at the token expiry logic?"}' | jq .

# Bob gets a "mentioned" notification
curl -s "$BASE/api/notifications" \
  -H "Authorization: Bearer $BOB_TOKEN" | jq '[.items[] | select(.type=="mentioned")]'
```

---

## 10. Workflow Violations

### Illegal transition (skipping a step)

The engine looks up the allowed transition graph per-project. Jumping from `To Do` directly to `Done` has no edge in the graph — 422 with the list of valid next statuses.

```bash
# Create a fresh issue (starts in To Do)
NEW_ID=$(curl -s -X POST "$BASE/api/projects/$PROJECT_ID/issues" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"type":"task","title":"Workflow test","priority":"low"}' | jq -r '.id')

# Try jumping directly to Done — expect 422
curl -s -X POST "$BASE/api/issues/$NEW_ID/transitions" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"target_status_id\":\"$DONE_ID\"}" | jq .
```

Expected:
```json
{
  "message": "transition not allowed",
  "details": {
    "message": "cannot move from \"To Do\" directly to the requested status",
    "allowed_transitions": [{ "name": "In Progress", ... }]
  }
}
```

### Validation rule blocking a transition

Recall the rule we attached: `assignee_id must not be empty` on In Progress → In Review. Try it with an unassigned issue.

```bash
# Unassigned issue: move to In Progress first (allowed)
curl -s -X POST "$BASE/api/issues/$NEW_ID/transitions" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"target_status_id\":\"$INPROG_ID\"}" | jq .

# Now try In Progress → In Review with no assignee — expect 422
curl -s -X POST "$BASE/api/issues/$NEW_ID/transitions" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"target_status_id\":\"$REVIEW_ID\"}" | jq .
```

Expected:
```json
{
  "message": "transition blocked by validation rule",
  "details": { "field": "assignee_id", "message": "Assign someone before moving to review" }
}
```

---

## 11. Concurrent Update Handling (Optimistic Locking)

Issues carry a `version` integer. The transition SQL is:
```sql
UPDATE issues SET status_id=$1, version=version+1 WHERE id=$2 AND version=$3
```

If two requests arrive simultaneously, one wins (rows_affected=1) and the other gets 0 rows — the server returns **HTTP 409**. Clients are expected to re-fetch and retry.

```bash
# Simulate by sending two identical transitions back-to-back (race condition demo)
curl -s -X POST "$BASE/api/issues/$NEW_ID/transitions" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"target_status_id\":\"$REVIEW_ID\"}" &

curl -s -X POST "$BASE/api/issues/$NEW_ID/transitions" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"target_status_id\":\"$REVIEW_ID\"}" &

wait
```

One will succeed (200), the other will return 409 `"version conflict"`.

---

## 12. Threaded Comments

Comments support one level of threading via `parent_id`. The reporter is auto-watched on issue creation; commenters are auto-watched when they post.

```bash
# Top-level comment
COMMENT_ID=$(curl -s -X POST "$BASE/api/issues/$BUG_ID/comments" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"body":"Reproduces consistently on staging."}' | jq -r '.id')

# Reply to that comment
curl -s -X POST "$BASE/api/issues/$BUG_ID/comments" \
  -H "Authorization: Bearer $BOB_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"body\": \"I can reproduce too. Will fix today.\",
    \"parent_id\": \"$COMMENT_ID\"
  }" | jq .

# List all comments — replies appear nested under their parent
curl -s "$BASE/api/issues/$BUG_ID/comments" \
  -H "Authorization: Bearer $ALICE_TOKEN" | jq .
```

---

## 13. Watch / Unwatch

Watchers receive notifications when the issue's status changes or a comment is added (excluding their own actions).

```bash
# Alice explicitly watches the task
curl -s -X POST "$BASE/api/issues/$TASK_ID/watch" \
  -H "Authorization: Bearer $ALICE_TOKEN" | jq .

# Unwatch
curl -s -X DELETE "$BASE/api/issues/$TASK_ID/watch" \
  -H "Authorization: Bearer $ALICE_TOKEN" | jq .
```

---

## 14. Custom Fields

Custom fields are project-scoped and typed (`text`, `number`, `select`). Values are stored as JSONB per issue.

### Define a field

```bash
FIELD_ID=$(curl -s -X POST "$BASE/api/projects/$PROJECT_ID/custom-fields" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Severity",
    "type": "select",
    "options": ["P0","P1","P2","P3"]
  }' | jq -r '.id')
```

### Set a value on an issue

```bash
curl -s -X PUT "$BASE/api/issues/$BUG_ID/custom-fields/$FIELD_ID" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"value":"P0"}' | jq .
```

### Read back

```bash
curl -s "$BASE/api/issues/$BUG_ID" \
  -H "Authorization: Bearer $ALICE_TOKEN" | jq '.custom_fields'
```

---

## 15. Full-Text Search

PostgreSQL `tsvector` (with a GIN index) powers search across issue titles, descriptions, and comment bodies. Standard structured filters compose with the FTS query.

### Search by keyword in title/description

```bash
curl -s "$BASE/api/search?q=token&project_id=$PROJECT_ID" \
  -H "Authorization: Bearer $ALICE_TOKEN" | jq '.items[].title'
```

### Search inside comment bodies

```bash
# The FTS index includes comment bodies joined via subquery
curl -s "$BASE/api/search?q=expiry+logic&project_id=$PROJECT_ID" \
  -H "Authorization: Bearer $ALICE_TOKEN" | jq .
```

### Structured filters

```bash
# All critical bugs assigned to Bob
curl -s "$BASE/api/search?project_id=$PROJECT_ID&priority=critical&type=bug&assignee_id=$BOB_ID" \
  -H "Authorization: Bearer $ALICE_TOKEN" | jq '.items[].title'

# Combine FTS + structured
curl -s "$BASE/api/search?q=auth&project_id=$PROJECT_ID&type=story" \
  -H "Authorization: Bearer $ALICE_TOKEN" | jq .
```

---

## 16. Notifications

### List with pagination

```bash
curl -s "$BASE/api/notifications?limit=20" \
  -H "Authorization: Bearer $BOB_TOKEN" | jq .
```

### Mark a single notification read

```bash
NOTIF_ID=$(curl -s "$BASE/api/notifications" \
  -H "Authorization: Bearer $BOB_TOKEN" | jq -r '.items[0].id')

curl -s -X POST "$BASE/api/notifications/$NOTIF_ID/read" \
  -H "Authorization: Bearer $BOB_TOKEN" | jq .
```

### Mark all read

```bash
curl -s -X POST "$BASE/api/notifications/read-all" \
  -H "Authorization: Bearer $BOB_TOKEN" | jq .
```

---

## 17. Board, Backlog, Sprints

### Kanban board

Returns the project's active issues grouped by the current sprint, with status metadata. The response includes `project`, `issues`, and `total`.

```bash
curl -s "$BASE/api/projects/$PROJECT_ID/board" \
  -H "Authorization: Bearer $ALICE_TOKEN" | jq '{total:.total, statuses: [.issues[].status.name] | unique}'
```

### Backlog

Issues not assigned to any sprint.

```bash
curl -s "$BASE/api/projects/$PROJECT_ID/backlog" \
  -H "Authorization: Bearer $ALICE_TOKEN" | jq '[.[] | .title]'
```

### All sprints for a project

```bash
curl -s "$BASE/api/projects/$PROJECT_ID/sprints" \
  -H "Authorization: Bearer $ALICE_TOKEN" | jq '.[] | {name:.name, status:.status, velocity:.velocity}'
```

---

## 18. Activity Feed

Cursor-based paginated feed of everything that happened in the project: issue created, status changed, sprint carry-over, assignee changed, etc.

```bash
# First page (most recent 20 events)
FEED=$(curl -s "$BASE/api/projects/$PROJECT_ID/activity?limit=20" \
  -H "Authorization: Bearer $ALICE_TOKEN")

echo "$FEED" | jq '.items[] | {event:.event_type, field:.field_name, old:.old_value, new:.new_value}'

# Next page (use cursor from previous response)
CURSOR=$(echo "$FEED" | jq -r '.next_cursor')
curl -s "$BASE/api/projects/$PROJECT_ID/activity?limit=20&cursor=$CURSOR" \
  -H "Authorization: Bearer $ALICE_TOKEN" | jq '.items | length'
```

---

## 19. WebSocket — Real-time Events

Connect to a project room to receive live updates. An `issue_id` param also subscribes to that specific issue's room.

```bash
# Requires websocat: brew install websocat
websocat "wss://project-management-platform-x0ml.onrender.com/api/ws?user_id=$ALICE_ID&project_id=$PROJECT_ID"
```

Heartbeat — send every 25 seconds to keep the connection alive:
```json
{"type":"heartbeat"}
```

Server replies:
```json
{"type":"heartbeat_ack"}
```

**Event types:**

| Event | When fired |
| ----- | --------- |
| `issue_created` | New issue in the project |
| `issue_updated` | Field or status change |
| `issue_moved` | Issue moved between sprints |
| `issue_deleted` | Issue deleted |
| `comment_added` | New comment posted |
| `sprint_updated` | Sprint started, completed, or modified |
| `presence_update` | User connected/disconnected from board |

### Missed-event replay

If a client reconnects after being offline, it can replay events it missed:

```bash
# Pass the Unix millisecond timestamp of the last event seen
curl -s "$BASE/api/projects/$PROJECT_ID/events?since=1700000000000" \
  -H "Authorization: Bearer $ALICE_TOKEN" | jq .
```

---

## 20. Swagger UI

Interactive API documentation served directly from the binary via `go:embed`.

```
https://project-management-platform-x0ml.onrender.com/docs
```

The raw OpenAPI spec:
```
https://project-management-platform-x0ml.onrender.com/docs/openapi.json
```

---

## Complete End-to-End Script

The following script chains all steps above in one run. Copy it, set `BASE`, and execute from a shell that has `curl` and `jq` installed.

```bash
#!/usr/bin/env bash
set -euo pipefail

BASE="https://project-management-platform-x0ml.onrender.com"

echo "=== Health ==="
curl -s "$BASE/health" | jq .

echo "=== Register ==="
curl -s -X POST "$BASE/api/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"secret123","name":"Alice Smith"}' | jq .
curl -s -X POST "$BASE/api/auth/register" \
  -H "Content-Type: application/json" \
  -d '{"email":"bob@example.com","password":"secret123","name":"Bob Jones"}' | jq .

echo "=== Login ==="
ALICE_TOKEN=$(curl -s -X POST "$BASE/api/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"secret123"}' | jq -r '.token')
BOB_TOKEN=$(curl -s -X POST "$BASE/api/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"bob@example.com","password":"secret123"}' | jq -r '.token')

BOB_ID=$(curl -s -X POST "$BASE/api/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"bob@example.com","password":"secret123"}' | jq -r '.user.id')
ALICE_ID=$(curl -s -X POST "$BASE/api/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"secret123"}' | jq -r '.user.id')

echo "=== Create Project ==="
PROJECT_ID=$(curl -s -X POST "$BASE/api/projects" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Platform Team","key":"PLT","description":"Core platform work"}' | jq -r '.id')
echo "Project: $PROJECT_ID"

echo "=== Capture Status IDs ==="
STATUSES=$(curl -s "$BASE/api/projects/$PROJECT_ID/workflow/statuses" \
  -H "Authorization: Bearer $ALICE_TOKEN")
TODO_ID=$(echo "$STATUSES"    | jq -r '.[] | select(.name=="To Do")      | .id')
INPROG_ID=$(echo "$STATUSES"  | jq -r '.[] | select(.name=="In Progress") | .id')
REVIEW_ID=$(echo "$STATUSES"  | jq -r '.[] | select(.name=="In Review")   | .id')
DONE_ID=$(echo "$STATUSES"    | jq -r '.[] | select(.name=="Done")        | .id')

echo "=== Create Issues ==="
EPIC_ID=$(curl -s -X POST "$BASE/api/projects/$PROJECT_ID/issues" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"type":"epic","title":"Auth System","priority":"high"}' | jq -r '.id')

STORY_ID=$(curl -s -X POST "$BASE/api/projects/$PROJECT_ID/issues" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"type\":\"story\",\"title\":\"JWT issuance\",\"parent_id\":\"$EPIC_ID\",\"story_points\":5,\"assignee_id\":\"$BOB_ID\"}" | jq -r '.id')

BUG_ID=$(curl -s -X POST "$BASE/api/projects/$PROJECT_ID/issues" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"type\":\"bug\",\"title\":\"Token expiry bug\",\"parent_id\":\"$EPIC_ID\",\"priority\":\"critical\"}" | jq -r '.id')

echo "=== Sprint Lifecycle ==="
SPRINT_ID=$(curl -s -X POST "$BASE/api/projects/$PROJECT_ID/sprints" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"Sprint 1","goal":"Ship auth","start_date":"2025-05-01","end_date":"2025-05-14"}' | jq -r '.id')

curl -s -X POST "$BASE/api/sprints/$SPRINT_ID/move-issue" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"issue_id\":\"$STORY_ID\"}" > /dev/null

curl -s -X POST "$BASE/api/sprints/$SPRINT_ID/start" \
  -H "Authorization: Bearer $ALICE_TOKEN" | jq '{status:.status}'

echo "=== Transition Chain ==="
curl -s -X POST "$BASE/api/issues/$STORY_ID/transitions" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"target_status_id\":\"$INPROG_ID\"}" | jq '{status:.status.name}'

curl -s -X POST "$BASE/api/issues/$STORY_ID/transitions" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"target_status_id\":\"$REVIEW_ID\"}" | jq '{status:.status.name}'

curl -s -X POST "$BASE/api/issues/$STORY_ID/transitions" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"target_status_id\":\"$DONE_ID\"}" | jq '{status:.status.name}'

echo "=== Complete Sprint ==="
curl -s -X POST "$BASE/api/sprints/$SPRINT_ID/complete" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"carry_over_issue_ids\":[\"$BUG_ID\"],\"next_sprint_id\":null}" \
  | jq '{velocity:.completed_points, incomplete_count:(.incomplete_issues|length)}'

echo "=== @mention ==="
curl -s -X POST "$BASE/api/issues/$BUG_ID/comments" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"body":"@bob_jones please investigate the expiry logic"}' | jq .

echo "=== Bob Notifications ==="
curl -s "$BASE/api/notifications" \
  -H "Authorization: Bearer $BOB_TOKEN" | jq '[.items[] | {type:.type, title:.title}]'

echo "=== FTS ==="
curl -s "$BASE/api/search?q=expiry&project_id=$PROJECT_ID" \
  -H "Authorization: Bearer $ALICE_TOKEN" | jq '[.items[] | .title]'

echo "=== Activity Feed ==="
curl -s "$BASE/api/projects/$PROJECT_ID/activity?limit=5" \
  -H "Authorization: Bearer $ALICE_TOKEN" | jq '[.items[] | {event:.event_type, field:.field_name}]'

echo "=== Board ==="
curl -s "$BASE/api/projects/$PROJECT_ID/board" \
  -H "Authorization: Bearer $ALICE_TOKEN" | jq '{total:.total}'

echo "=== Done ==="
```

---

## Notes

- **JWT tokens expire after 24 hours.** Re-run the login step if you get 401 responses.
- **Render free tier spins down after 15 minutes of inactivity.** The first request after a cold start may take 30–60 seconds.
- The `key` in `POST /api/projects` must be unique across all projects. Use a different key if you get a 409 conflict.
- The `target_status_id` field name (not `to_status_id`) is what the transition endpoint expects.
