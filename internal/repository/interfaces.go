// Package repository defines the persistence interfaces consumed by the service layer,
// as well as the concrete pgx-backed implementations that satisfy them.
//
// Design principle: services depend on the interfaces defined here, never on
// concrete structs. This keeps the domain logic decoupled from the database driver
// and makes every service independently unit-testable.
package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
)

// ─────────────────────────────────────────────────────────────────────────────
// Store interfaces
// ─────────────────────────────────────────────────────────────────────────────

// UserStore defines all user persistence operations.
type UserStore interface {
	Create(ctx context.Context, email, displayName, passwordHash string) (*models.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByHandle(ctx context.Context, handle string) (*models.User, error)
	ListByIDs(ctx context.Context, ids []uuid.UUID) ([]*models.UserResponse, error)
}

// ProjectStore defines all project and membership persistence operations.
type ProjectStore interface {
	Create(ctx context.Context, req *models.CreateProjectRequest) (*models.Project, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Project, error)
	List(ctx context.Context, userID uuid.UUID) ([]*models.Project, error)
	Update(ctx context.Context, id uuid.UUID, req *models.UpdateProjectRequest) (*models.Project, error)
	Delete(ctx context.Context, id uuid.UUID) error
	AddMember(ctx context.Context, projectID, userID uuid.UUID, role string) error
	IsMember(ctx context.Context, projectID, userID uuid.UUID) (bool, error)
	// NextIssueKey atomically increments the project counter and returns the key (e.g. "PROJ-42").
	NextIssueKey(ctx context.Context, projectID uuid.UUID) (string, error)
}

// IssueStore defines all issue, watcher, and activity-log persistence operations.
type IssueStore interface {
	Create(ctx context.Context, issue *models.Issue) (*models.Issue, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Issue, error)
	Update(ctx context.Context, id uuid.UUID, req *models.UpdateIssueRequest) (*models.Issue, error)
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, f models.IssueFilter) ([]*models.Issue, int, error)
	GetBacklog(ctx context.Context, projectID uuid.UUID) ([]*models.Issue, error)
	GetIncompleteBySprintID(ctx context.Context, sprintID uuid.UUID) ([]*models.Issue, error)
	SumCompletedPoints(ctx context.Context, sprintID uuid.UUID) (int, error)

	// Watchers
	GetWatchers(ctx context.Context, issueID uuid.UUID) ([]uuid.UUID, error)
	AddWatcher(ctx context.Context, issueID, userID uuid.UUID) error
	RemoveWatcher(ctx context.Context, issueID, userID uuid.UUID) error

	// Activity
	LogActivity(ctx context.Context, entry *models.ActivityLog) error
	GetActivityFeed(ctx context.Context, projectID uuid.UUID, cursor string, limit int) ([]*models.ActivityLog, int, error)

	// Pool exposes the underlying connection pool for use by the Transactor.
	Pool() *Database
}

// SprintStore defines all sprint persistence operations.
type SprintStore interface {
	Create(ctx context.Context, projectID uuid.UUID, req *models.CreateSprintRequest) (*models.Sprint, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Sprint, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*models.Sprint, error)
	GetActiveSprint(ctx context.Context, projectID uuid.UUID) (*models.Sprint, error)
	Update(ctx context.Context, id uuid.UUID, req *models.UpdateSprintRequest) (*models.Sprint, error)
	Start(ctx context.Context, id uuid.UUID) (*models.Sprint, error)
	Complete(ctx context.Context, id uuid.UUID, velocity int) (*models.Sprint, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// WorkflowStore defines all workflow status and transition persistence operations.
type WorkflowStore interface {
	CreateStatus(ctx context.Context, projectID uuid.UUID, req *models.CreateStatusRequest) (*models.WorkflowStatus, error)
	GetStatusByID(ctx context.Context, id uuid.UUID) (*models.WorkflowStatus, error)
	ListStatuses(ctx context.Context, projectID uuid.UUID) ([]*models.WorkflowStatus, error)
	GetDefaultStatus(ctx context.Context, projectID uuid.UUID) (*models.WorkflowStatus, error)
	CreateTransition(ctx context.Context, projectID uuid.UUID, req *models.CreateTransitionRequest) (*models.WorkflowTransition, error)
	GetAllowedTransitions(ctx context.Context, projectID, fromStatusID uuid.UUID) ([]*models.WorkflowTransition, error)
	DeleteTransition(ctx context.Context, id uuid.UUID) error
}

// CommentStore defines all comment persistence operations.
type CommentStore interface {
	Create(ctx context.Context, issueID, authorID uuid.UUID, body string, parentID *uuid.UUID) (*models.Comment, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.Comment, error)
	ListByIssue(ctx context.Context, issueID uuid.UUID) ([]*models.Comment, error)
	Update(ctx context.Context, id uuid.UUID, body string) (*models.Comment, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// NotificationStore defines all notification persistence operations.
type NotificationStore interface {
	Create(ctx context.Context, n *models.Notification) error
	List(ctx context.Context, userID uuid.UUID, cursor string, limit int) ([]*models.Notification, int, error)
	MarkRead(ctx context.Context, id, userID uuid.UUID) error
	MarkAllRead(ctx context.Context, userID uuid.UUID) error
	UnreadCount(ctx context.Context, userID uuid.UUID) (int, error)
}

// CustomFieldStore manages per-project field definitions and per-issue values.
type CustomFieldStore interface {
	CreateDefinition(ctx context.Context, projectID uuid.UUID, req *models.CreateCustomFieldRequest) (*models.CustomFieldDefinition, error)
	ListDefinitions(ctx context.Context, projectID uuid.UUID) ([]*models.CustomFieldDefinition, error)
	GetDefinition(ctx context.Context, id uuid.UUID) (*models.CustomFieldDefinition, error)
	DeleteDefinition(ctx context.Context, id uuid.UUID) error

	SetValue(ctx context.Context, issueID, fieldID uuid.UUID, value *string) error
	GetValues(ctx context.Context, issueID uuid.UUID) ([]*models.CustomFieldValue, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// Transaction abstraction
// ─────────────────────────────────────────────────────────────────────────────

// TxStore provides the subset of persistence operations that must execute within
// a single database transaction. Services receive this interface inside a WithTx
// callback — they never interact with pgx.Tx directly.
type TxStore interface {
	// UpdateIssueStatus applies an optimistic-locked status transition.
	// Returns the number of rows affected (0 means a concurrent writer changed the version).
	UpdateIssueStatus(ctx context.Context, issueID, statusID uuid.UUID, version int) (rowsAffected int64, err error)

	// UpdateIssueSprint moves an issue to a sprint (nil = backlog).
	UpdateIssueSprint(ctx context.Context, issueID uuid.UUID, sprintID *uuid.UUID) error

	// LogActivity appends an immutable audit entry.
	LogActivity(ctx context.Context, entry *models.ActivityLog) error
}

// Transactor provides atomic, ACID-compliant execution of a TxFunc.
// Implementations must roll back on any error and commit only on success.
type Transactor interface {
	// WithTx begins a transaction, executes fn, and commits.
	// If fn returns a non-nil error, the transaction is rolled back.
	WithTx(ctx context.Context, fn func(ctx context.Context, tx TxStore) error) error
}
