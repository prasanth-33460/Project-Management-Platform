package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/prasanth-33460/Project-Management-Platform/internal/apperror"
	"github.com/prasanth-33460/Project-Management-Platform/internal/api/websocket"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
	"github.com/prasanth-33460/Project-Management-Platform/internal/repository"
)

// WorkflowEngine handles status transitions with optimistic locking.
// The status update and activity log are committed together; WS broadcast and
// watcher notifications fire after a successful commit.
type WorkflowEngine struct {
	issues    repository.IssueStore
	workflows repository.WorkflowStore
	notifs    repository.NotificationStore
	tx        repository.Transactor
	hub       *websocket.Hub
}

func NewWorkflowEngine(
	issues repository.IssueStore,
	workflows repository.WorkflowStore,
	notifs repository.NotificationStore,
	tx repository.Transactor,
	hub *websocket.Hub,
) *WorkflowEngine {
	return &WorkflowEngine{
		issues:    issues,
		workflows: workflows,
		notifs:    notifs,
		tx:        tx,
		hub:       hub,
	}
}

// Transition moves an issue to a new status.
// Returns 422 with allowed transitions if the target isn't reachable from current status.
// Returns 409 if a concurrent writer changed the version between our read and write.
func (e *WorkflowEngine) Transition(
	ctx context.Context,
	issueID uuid.UUID,
	req *models.TransitionRequest,
	actorID uuid.UUID,
) (*models.Issue, error) {
	issue, err := e.issues.GetByID(ctx, issueID)
	if err != nil {
		return nil, err
	}
	if issue == nil {
		return nil, apperror.ErrNotFound
	}

	allowed, err := e.workflows.GetAllowedTransitions(ctx, issue.ProjectID, issue.StatusID)
	if err != nil {
		return nil, err
	}

	var transition *models.WorkflowTransition
	for _, t := range allowed {
		if t.ToStatusID == req.TargetStatusID {
			transition = t
			break
		}
	}

	if transition == nil {
		allowedStatuses := make([]*models.WorkflowStatus, 0, len(allowed))
		for _, t := range allowed {
			allowedStatuses = append(allowedStatuses, t.ToStatus)
		}
		return nil, apperror.WithDetails(422, "transition not allowed", &models.TransitionError{
			Message:            fmt.Sprintf("cannot move from %q directly to the requested status", issue.Status.Name),
			AllowedTransitions: collectStatuses(allowedStatuses),
		})
	}

	// check validation rules before touching the DB
	if err := checkValidationRules(issue, transition.ValidationRules); err != nil {
		return nil, err
	}

	oldStatusName := issue.Status.Name
	newStatusName := ""
	if transition.ToStatus != nil {
		newStatusName = transition.ToStatus.Name
	}

	// status update + activity log in one transaction
	var rowsAffected int64
	if err := e.tx.WithTx(ctx, func(ctx context.Context, txStore repository.TxStore) error {
		rows, err := txStore.UpdateIssueStatus(ctx, issueID, req.TargetStatusID, issue.Version)
		if err != nil {
			return err
		}
		rowsAffected = rows

		if rows == 0 {
			// optimistic lock lost — caller returns 409
			return nil
		}

		return txStore.LogActivity(ctx, &models.ActivityLog{
			IssueID:   &issueID,
			ProjectID: issue.ProjectID,
			ActorID:   actorID,
			EventType: "status_changed",
			FieldName: strPtr("status"),
			OldValue:  strPtr(oldStatusName),
			NewValue:  strPtr(newStatusName),
		})
	}); err != nil {
		return nil, fmt.Errorf("transition transaction: %w", err)
	}

	if rowsAffected == 0 {
		return nil, apperror.ErrConflict
	}

	updated, err := e.issues.GetByID(ctx, issueID)
	if err != nil {
		return nil, err
	}

	// best-effort post-commit actions (e.g. auto-assign field)
	e.executeAutoActions(ctx, updated, transition, actorID)

	e.hub.Broadcast("project:"+issue.ProjectID.String(), &websocket.Event{
		Type:    "issue_updated",
		Payload: updated,
	})
	e.hub.Broadcast("issue:"+issueID.String(), &websocket.Event{
		Type:    "issue_updated",
		Payload: updated,
	})

	go e.notifyWatchers(
		context.Background(),
		updated,
		actorID,
		"status_changed",
		fmt.Sprintf("%s moved to %s", updated.IssueKey, newStatusName),
	)

	return updated, nil
}

// executeAutoActions runs transition auto_actions after commit.
// Failures are logged and swallowed — don't abort for this.
// Currently supports: assign_field → sets assignee_id.
func (e *WorkflowEngine) executeAutoActions(
	ctx context.Context,
	issue *models.Issue,
	transition *models.WorkflowTransition,
	actorID uuid.UUID,
) {
	for _, action := range transition.AutoActions {
		if action.Type != "assign_field" || action.Field != "assignee_id" || action.Value == "" {
			continue
		}
		assigneeID, err := uuid.Parse(action.Value)
		if err != nil {
			slog.WarnContext(ctx, "auto-action: invalid assignee_id", "value", action.Value, "error", err)
			continue
		}
		req := &models.UpdateIssueRequest{AssigneeID: &assigneeID}
		if _, err := e.issues.Update(ctx, issue.ID, req); err != nil {
			slog.WarnContext(ctx, "auto-action: update assignee failed",
				"issue_id", issue.ID,
				"assignee_id", assigneeID,
				"error", err,
			)
		}
	}
}

func (e *WorkflowEngine) notifyWatchers(
	ctx context.Context,
	issue *models.Issue,
	actorID uuid.UUID,
	notifType, title string,
) {
	watchers, err := e.issues.GetWatchers(ctx, issue.ID)
	if err != nil {
		slog.WarnContext(ctx, "get watchers failed for notification",
			"issue_id", issue.ID, "error", err)
		return
	}
	refType := "issue"
	for _, watcherID := range watchers {
		if watcherID == actorID {
			continue // skip if actor is watching — no point notifying yourself
		}
		n := &models.Notification{
			UserID:  watcherID,
			Type:    notifType,
			RefID:   &issue.ID,
			RefType: &refType,
			Title:   title,
		}
		if err := e.notifs.Create(ctx, n); err != nil {
			slog.WarnContext(ctx, "watcher notification failed",
				"user_id", watcherID, "issue_id", issue.ID, "error", err)
		}
	}
}

// checkValidationRules returns a 422 if any rule fails against the current issue state.
func checkValidationRules(issue *models.Issue, rules []models.ValidationRule) error {
	for _, r := range rules {
		msg := r.Message
		if msg == "" {
			msg = fmt.Sprintf("field %q must satisfy %q before this transition", r.Field, r.Operator)
		}

		var fails bool
		switch r.Field {
		case "assignee_id":
			switch r.Operator {
			case "not_empty":
				fails = issue.AssigneeID == nil
			case "is_empty":
				fails = issue.AssigneeID != nil
			}
		case "story_points":
			switch r.Operator {
			case "not_empty":
				fails = issue.StoryPoints == nil
			case "is_empty":
				fails = issue.StoryPoints != nil
			}
		case "description":
			switch r.Operator {
			case "not_empty":
				fails = issue.Description == ""
			case "is_empty":
				fails = issue.Description != ""
			}
		}

		if fails {
			return apperror.WithDetails(422, "transition blocked by validation rule", map[string]string{
				"field":   r.Field,
				"message": msg,
			})
		}
	}
	return nil
}

func collectStatuses(ss []*models.WorkflowStatus) []models.WorkflowStatus {
	out := make([]models.WorkflowStatus, 0, len(ss))
	for _, s := range ss {
		if s != nil {
			out = append(out, *s)
		}
	}
	return out
}

// strPtr is a small helper used across service files.
func strPtr(s string) *string { return &s }
