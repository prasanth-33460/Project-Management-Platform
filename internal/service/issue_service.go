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

// IssueService handles issue CRUD, watch management, and activity feed queries.
type IssueService struct {
	issues    repository.IssueStore
	projects  repository.ProjectStore
	workflows repository.WorkflowStore
	notifs    repository.NotificationStore
	hub       *websocket.Hub
}

func NewIssueService(
	issues repository.IssueStore,
	projects repository.ProjectStore,
	workflows repository.WorkflowStore,
	notifs repository.NotificationStore,
	hub *websocket.Hub,
) *IssueService {
	return &IssueService{
		issues:    issues,
		projects:  projects,
		workflows: workflows,
		notifs:    notifs,
		hub:       hub,
	}
}

func (s *IssueService) Create(ctx context.Context, projectID uuid.UUID, req *models.CreateIssueRequest, reporterID uuid.UUID) (*models.Issue, error) {
	if req.ParentID != nil {
		parent, err := s.issues.GetByID(ctx, *req.ParentID)
		if err != nil {
			return nil, err
		}
		if parent == nil {
			return nil, apperror.WithDetails(400, "parent issue not found", nil)
		}
		if err := validateParentChild(parent.Type, req.Type); err != nil {
			return nil, err
		}
	}

	var statusID uuid.UUID
	if req.StatusID != nil {
		statusID = *req.StatusID
	} else {
		defaultStatus, err := s.workflows.GetDefaultStatus(ctx, projectID)
		if err != nil {
			return nil, err
		}
		if defaultStatus == nil {
			return nil, apperror.WithDetails(400, "project has no default workflow status; create statuses first", nil)
		}
		statusID = defaultStatus.ID
	}

	issueKey, err := s.projects.NextIssueKey(ctx, projectID)
	if err != nil {
		return nil, err
	}

	if req.Priority == "" {
		req.Priority = models.PriorityMedium
	}

	issue := &models.Issue{
		ProjectID:   projectID,
		IssueKey:    issueKey,
		SprintID:    req.SprintID,
		ParentID:    req.ParentID,
		Type:        req.Type,
		Title:       req.Title,
		Description: req.Description,
		StatusID:    statusID,
		Priority:    req.Priority,
		AssigneeID:  req.AssigneeID,
		ReporterID:  reporterID,
		StoryPoints: req.StoryPoints,
		Labels:      req.Labels,
	}

	created, err := s.issues.Create(ctx, issue)
	if err != nil {
		return nil, err
	}

	if err := s.issues.AddWatcher(ctx, created.ID, reporterID); err != nil {
		slog.WarnContext(ctx, "auto-watch reporter failed", "issue_id", created.ID, "user_id", reporterID, "error", err)
	}
	// assignee gets auto-watched too, but only if they're someone other than the reporter
	if req.AssigneeID != nil && *req.AssigneeID != reporterID {
		if err := s.issues.AddWatcher(ctx, created.ID, *req.AssigneeID); err != nil {
			slog.WarnContext(ctx, "auto-watch assignee failed", "issue_id", created.ID, "user_id", req.AssigneeID, "error", err)
		}
	}

	if err := s.issues.LogActivity(ctx, &models.ActivityLog{
		IssueID:   &created.ID,
		ProjectID: projectID,
		ActorID:   reporterID,
		EventType: "issue_created",
		NewValue:  strPtr(created.IssueKey),
	}); err != nil {
		slog.WarnContext(ctx, "activity log failed on create", "issue_id", created.ID, "error", err)
	}

	if req.AssigneeID != nil && *req.AssigneeID != reporterID {
		refType := "issue"
		n := &models.Notification{
			UserID:  *req.AssigneeID,
			Type:    "assigned",
			RefID:   &created.ID,
			RefType: &refType,
			Title:   fmt.Sprintf("You were assigned to %s: %s", created.IssueKey, created.Title),
		}
		if err := s.notifs.Create(ctx, n); err != nil {
			slog.WarnContext(ctx, "assignment notification failed", "user_id", req.AssigneeID, "error", err)
		}
	}

	s.hub.Broadcast("project:"+projectID.String(), &websocket.Event{
		Type:    "issue_created",
		Payload: created,
	})

	return created, nil
}

func (s *IssueService) GetByID(ctx context.Context, id uuid.UUID) (*models.Issue, error) {
	issue, err := s.issues.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if issue == nil {
		return nil, apperror.ErrNotFound
	}
	watchers, err := s.issues.GetWatchers(ctx, id)
	if err != nil {
		slog.WarnContext(ctx, "get watchers failed", "issue_id", id, "error", err)
	}
	issue.Watchers = watchers
	return issue, nil
}

func (s *IssueService) Update(ctx context.Context, id uuid.UUID, req *models.UpdateIssueRequest, actorID uuid.UUID) (*models.Issue, error) {
	existing, err := s.issues.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, apperror.ErrNotFound
	}

	updated, err := s.issues.Update(ctx, id, req)
	if err != nil {
		return nil, err
	}

	if req.AssigneeID != nil {
		oldAssignee := ""
		if existing.AssigneeID != nil {
			oldAssignee = existing.AssigneeID.String()
		}
		if err := s.issues.LogActivity(ctx, &models.ActivityLog{
			IssueID:   &id,
			ProjectID: existing.ProjectID,
			ActorID:   actorID,
			EventType: "field_updated",
			FieldName: strPtr("assignee_id"),
			OldValue:  strPtr(oldAssignee),
			NewValue:  strPtr(req.AssigneeID.String()),
		}); err != nil {
			slog.WarnContext(ctx, "activity log failed on update", "issue_id", id, "error", err)
		}

		if *req.AssigneeID != actorID {
			refType := "issue"
			n := &models.Notification{
				UserID:  *req.AssigneeID,
				Type:    "assigned",
				RefID:   &id,
				RefType: &refType,
				Title:   fmt.Sprintf("You were assigned to %s: %s", existing.IssueKey, existing.Title),
			}
			if err := s.notifs.Create(ctx, n); err != nil {
				slog.WarnContext(ctx, "assignment notification failed on update", "user_id", req.AssigneeID, "error", err)
			}
		}
	}

	projectRoom := "project:" + existing.ProjectID.String()
	s.hub.Broadcast(projectRoom, &websocket.Event{Type: "issue_updated", Payload: updated})
	s.hub.Broadcast("issue:"+id.String(), &websocket.Event{Type: "issue_updated", Payload: updated})

	return updated, nil
}

func (s *IssueService) Delete(ctx context.Context, id uuid.UUID, actorID uuid.UUID) error {
	issue, err := s.issues.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if issue == nil {
		return apperror.ErrNotFound
	}
	if err := s.issues.Delete(ctx, id); err != nil {
		return err
	}
	s.hub.Broadcast("project:"+issue.ProjectID.String(), &websocket.Event{
		Type:    "issue_deleted",
		Payload: map[string]string{"issue_id": id.String()},
	})
	return nil
}

func (s *IssueService) List(ctx context.Context, filter models.IssueFilter) (*models.PagedResult[*models.Issue], error) {
	issues, total, err := s.issues.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	var nextCursor string
	if len(issues) == filter.Limit && len(issues) > 0 {
		last := issues[len(issues)-1]
		nextCursor = models.EncodeCursor(last.CreatedAt, last.ID)
	}

	return &models.PagedResult[*models.Issue]{
		Items:      issues,
		NextCursor: nextCursor,
		Total:      total,
	}, nil
}

func (s *IssueService) GetBacklog(ctx context.Context, projectID uuid.UUID) ([]*models.Issue, error) {
	return s.issues.GetBacklog(ctx, projectID)
}

func (s *IssueService) Watch(ctx context.Context, issueID, userID uuid.UUID) error {
	return s.issues.AddWatcher(ctx, issueID, userID)
}

func (s *IssueService) Unwatch(ctx context.Context, issueID, userID uuid.UUID) error {
	return s.issues.RemoveWatcher(ctx, issueID, userID)
}

func (s *IssueService) GetActivityFeed(ctx context.Context, projectID uuid.UUID, cursor string, limit int) (*models.ActivityFeedResponse, error) {
	logs, total, err := s.issues.GetActivityFeed(ctx, projectID, cursor, limit)
	if err != nil {
		return nil, err
	}

	var nextCursor string
	if len(logs) > 0 && len(logs) == limit {
		last := logs[len(logs)-1]
		nextCursor = models.EncodeCursor(last.CreatedAt, last.ID)
	}

	return &models.ActivityFeedResponse{
		Items:      logs,
		NextCursor: nextCursor,
		Total:      total,
	}, nil
}

// validateParentChild enforces the issue hierarchy:
// epics can contain stories/tasks/bugs; those can contain subtasks; nothing else.
func validateParentChild(parentType, childType models.IssueType) error {
	allowed := map[models.IssueType][]models.IssueType{
		models.IssueTypeEpic:  {models.IssueTypeStory, models.IssueTypeTask, models.IssueTypeBug},
		models.IssueTypeStory: {models.IssueTypeSubtask},
		models.IssueTypeTask:  {models.IssueTypeSubtask},
		models.IssueTypeBug:   {models.IssueTypeSubtask},
	}
	for _, a := range allowed[parentType] {
		if a == childType {
			return nil
		}
	}
	return apperror.WithDetails(422, fmt.Sprintf("a %s cannot be a child of a %s", childType, parentType), nil)
}
