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

// SprintService manages sprint lifecycle and issue carry-over.
type SprintService struct {
	sprints  repository.SprintStore
	issues   repository.IssueStore
	projects repository.ProjectStore
	tx       repository.Transactor
	hub      *websocket.Hub
}

func NewSprintService(
	sprints repository.SprintStore,
	issues repository.IssueStore,
	projects repository.ProjectStore,
	tx repository.Transactor,
	hub *websocket.Hub,
) *SprintService {
	return &SprintService{
		sprints:  sprints,
		issues:   issues,
		projects: projects,
		tx:       tx,
		hub:      hub,
	}
}

func (s *SprintService) Create(ctx context.Context, projectID uuid.UUID, req *models.CreateSprintRequest) (*models.Sprint, error) {
	sprint, err := s.sprints.Create(ctx, projectID, req)
	if err != nil {
		return nil, err
	}
	s.broadcastSprintUpdate(sprint)
	return sprint, nil
}

func (s *SprintService) GetByID(ctx context.Context, id uuid.UUID) (*models.Sprint, error) {
	sprint, err := s.sprints.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if sprint == nil {
		return nil, apperror.ErrNotFound
	}
	return sprint, nil
}

func (s *SprintService) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*models.Sprint, error) {
	return s.sprints.ListByProject(ctx, projectID)
}

func (s *SprintService) Update(ctx context.Context, id uuid.UUID, req *models.UpdateSprintRequest) (*models.Sprint, error) {
	sprint, err := s.sprints.Update(ctx, id, req)
	if err != nil {
		return nil, err
	}
	if sprint == nil {
		return nil, apperror.ErrNotFound
	}
	s.broadcastSprintUpdate(sprint)
	return sprint, nil
}

func (s *SprintService) Start(ctx context.Context, id uuid.UUID) (*models.Sprint, error) {
	existing, err := s.sprints.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, apperror.ErrNotFound
	}

	active, err := s.sprints.GetActiveSprint(ctx, existing.ProjectID)
	if err != nil {
		return nil, err
	}
	if active != nil && active.ID != id {
		return nil, apperror.WithDetails(409, "project already has an active sprint", map[string]string{
			"active_sprint_id": active.ID.String(),
		})
	}

	sprint, err := s.sprints.Start(ctx, id)
	if err != nil {
		return nil, err
	}
	if sprint == nil {
		return nil, apperror.WithDetails(409, "sprint cannot be started (already active or completed)", nil)
	}
	s.broadcastSprintUpdate(sprint)
	return sprint, nil
}

// Complete closes the sprint. It totals up completed story points as velocity,
// then atomically moves any incomplete issues to either the next sprint or backlog.
func (s *SprintService) Complete(
	ctx context.Context,
	id uuid.UUID,
	req *models.CompleteSprintRequest,
	actorID uuid.UUID,
) (*models.SprintCompleteResult, error) {
	sprint, err := s.sprints.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if sprint == nil {
		return nil, apperror.ErrNotFound
	}
	if sprint.Status != models.SprintStatusActive {
		return nil, apperror.WithDetails(409, "only active sprints can be completed", nil)
	}

	velocity, err := s.issues.SumCompletedPoints(ctx, id)
	if err != nil {
		return nil, err
	}

	incomplete, err := s.issues.GetIncompleteBySprintID(ctx, id)
	if err != nil {
		return nil, err
	}

	carrySet := make(map[uuid.UUID]bool, len(req.CarryOverIssueIDs))
	for _, cid := range req.CarryOverIssueIDs {
		carrySet[cid] = true
	}

	if err := s.tx.WithTx(ctx, func(ctx context.Context, txStore repository.TxStore) error {
		for _, issue := range incomplete {
			var target *uuid.UUID
			if carrySet[issue.ID] {
				target = req.NextSprintID // nil → backlog
			}
			// issues not explicitly listed for carry-over always go to backlog

			if err := txStore.UpdateIssueSprint(ctx, issue.ID, target); err != nil {
				return fmt.Errorf("move issue %s: %w", issue.ID, err)
			}

			newVal := "backlog"
			if target != nil {
				newVal = target.String()
			}
			if err := txStore.LogActivity(ctx, &models.ActivityLog{
				IssueID:   &issue.ID,
				ProjectID: sprint.ProjectID,
				ActorID:   actorID,
				EventType: "sprint_carry_over",
				FieldName: strPtr("sprint_id"),
				OldValue:  strPtr(id.String()),
				NewValue:  strPtr(newVal),
			}); err != nil {
				// non-fatal — a failed audit entry shouldn't roll back the carry-over
				slog.WarnContext(ctx, "carry-over activity log failed",
					"issue_id", issue.ID, "error", err)
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	completed, err := s.sprints.Complete(ctx, id, velocity)
	if err != nil {
		return nil, err
	}

	s.broadcastSprintUpdate(completed)

	return &models.SprintCompleteResult{
		Sprint:           completed,
		CompletedPoints:  velocity,
		IncompleteIssues: incomplete,
	}, nil
}

func (s *SprintService) Delete(ctx context.Context, id uuid.UUID) error {
	sprint, err := s.sprints.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if sprint == nil {
		return apperror.ErrNotFound
	}
	if sprint.Status == models.SprintStatusActive {
		return apperror.WithDetails(409, "cannot delete an active sprint", nil)
	}
	return s.sprints.Delete(ctx, id)
}

func (s *SprintService) MoveIssue(ctx context.Context, req *models.MoveIssueRequest, actorID uuid.UUID) error {
	issue, err := s.issues.GetByID(ctx, req.IssueID)
	if err != nil {
		return err
	}
	if issue == nil {
		return apperror.ErrNotFound
	}

	oldVal := "backlog"
	if issue.SprintID != nil {
		oldVal = issue.SprintID.String()
	}
	newVal := "backlog"
	if req.SprintID != nil {
		newVal = req.SprintID.String()
	}

	if err := s.tx.WithTx(ctx, func(ctx context.Context, txStore repository.TxStore) error {
		if err := txStore.UpdateIssueSprint(ctx, req.IssueID, req.SprintID); err != nil {
			return err
		}
		return txStore.LogActivity(ctx, &models.ActivityLog{
			IssueID:   &req.IssueID,
			ProjectID: issue.ProjectID,
			ActorID:   actorID,
			EventType: "issue_moved",
			FieldName: strPtr("sprint_id"),
			OldValue:  strPtr(oldVal),
			NewValue:  strPtr(newVal),
		})
	}); err != nil {
		return err
	}

	s.hub.Broadcast("project:"+issue.ProjectID.String(), &websocket.Event{
		Type:    "issue_moved",
		Payload: map[string]interface{}{"issue_id": req.IssueID, "sprint_id": req.SprintID},
	})
	return nil
}

func (s *SprintService) broadcastSprintUpdate(sprint *models.Sprint) {
	s.hub.Broadcast("project:"+sprint.ProjectID.String(), &websocket.Event{
		Type:    "sprint_updated",
		Payload: sprint,
	})
}
