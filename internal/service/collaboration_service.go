package service

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/google/uuid"
	"github.com/prasanth-33460/Project-Management-Platform/internal/apperror"
	"github.com/prasanth-33460/Project-Management-Platform/internal/api/websocket"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
	"github.com/prasanth-33460/Project-Management-Platform/internal/repository"
)

var mentionRegex = regexp.MustCompile(`@([a-zA-Z0-9._-]+)`)

// CollaborationService handles threaded comments, @mention detection, and watcher auto-subscribe.
type CollaborationService struct {
	comments repository.CommentStore
	issues   repository.IssueStore
	users    repository.UserStore
	notifs   repository.NotificationStore
	hub      *websocket.Hub
}

func NewCollaborationService(
	comments repository.CommentStore,
	issues repository.IssueStore,
	users repository.UserStore,
	notifs repository.NotificationStore,
	hub *websocket.Hub,
) *CollaborationService {
	return &CollaborationService{
		comments: comments,
		issues:   issues,
		users:    users,
		notifs:   notifs,
		hub:      hub,
	}
}

func (s *CollaborationService) AddComment(
	ctx context.Context,
	issueID, authorID uuid.UUID,
	req *models.CreateCommentRequest,
) (*models.Comment, error) {
	issue, err := s.issues.GetByID(ctx, issueID)
	if err != nil {
		return nil, err
	}
	if issue == nil {
		return nil, apperror.ErrNotFound
	}

	if req.ParentID != nil {
		parent, err := s.comments.GetByID(ctx, *req.ParentID)
		if err != nil {
			return nil, err
		}
		if parent == nil || parent.IssueID != issueID {
			return nil, apperror.WithDetails(400, "parent comment not found on this issue", nil)
		}
	}

	comment, err := s.comments.Create(ctx, issueID, authorID, req.Body, req.ParentID)
	if err != nil {
		return nil, err
	}

	if err := s.issues.LogActivity(ctx, &models.ActivityLog{
		IssueID:   &issueID,
		ProjectID: issue.ProjectID,
		ActorID:   authorID,
		EventType: "comment_added",
		NewValue:  strPtr(comment.ID.String()),
	}); err != nil {
		slog.WarnContext(ctx, "activity log failed on comment", "issue_id", issueID, "error", err)
	}

	// auto-watch the commenter
	if err := s.issues.AddWatcher(ctx, issueID, authorID); err != nil {
		slog.WarnContext(ctx, "auto-watch commenter failed", "issue_id", issueID, "user_id", authorID, "error", err)
	}

	s.hub.Broadcast("issue:"+issueID.String(), &websocket.Event{
		Type:    "comment_added",
		Payload: comment,
	})
	s.hub.Broadcast("project:"+issue.ProjectID.String(), &websocket.Event{
		Type:    "comment_added",
		Payload: map[string]interface{}{"issue_id": issueID, "comment_id": comment.ID},
	})

	go s.notifyComment(context.Background(), issue, comment, authorID)

	return comment, nil
}

func (s *CollaborationService) notifyComment(
	ctx context.Context,
	issue *models.Issue,
	comment *models.Comment,
	authorID uuid.UUID,
) {
	watchers, err := s.issues.GetWatchers(ctx, issue.ID)
	if err != nil {
		slog.WarnContext(ctx, "get watchers failed for comment notification",
			"issue_id", issue.ID, "error", err)
		return
	}

	authorName := "Someone"
	if comment.Author != nil {
		authorName = comment.Author.DisplayName
	}

	notified := make(map[uuid.UUID]bool, len(watchers))
	refType := "comment"

	for _, wid := range watchers {
		if wid == authorID {
			continue // skip the author
		}
		n := &models.Notification{
			UserID:  wid,
			Type:    "comment_added",
			RefID:   &comment.ID,
			RefType: &refType,
			Title:   fmt.Sprintf("%s commented on %s", authorName, issue.IssueKey),
			Body:    strPtr(truncate(comment.Body, 100)),
		}
		if err := s.notifs.Create(ctx, n); err != nil {
			slog.WarnContext(ctx, "comment notification failed",
				"user_id", wid, "issue_id", issue.ID, "error", err)
		}
		notified[wid] = true
	}

	// resolve @mentions and notify those users (skip anyone already notified as a watcher)
	for _, m := range mentionRegex.FindAllStringSubmatch(comment.Body, -1) {
		if len(m) < 2 {
			continue
		}
		mentioned, err := s.users.GetByHandle(ctx, m[1])
		if err != nil || mentioned == nil {
			continue // unknown handle — silently skip
		}
		if mentioned.ID == authorID || notified[mentioned.ID] {
			continue // already got a watcher notification
		}
		n := &models.Notification{
			UserID:  mentioned.ID,
			Type:    "mentioned",
			RefID:   &comment.ID,
			RefType: &refType,
			Title:   fmt.Sprintf("%s mentioned you in %s", authorName, issue.IssueKey),
			Body:    strPtr(truncate(comment.Body, 100)),
		}
		if err := s.notifs.Create(ctx, n); err != nil {
			slog.WarnContext(ctx, "mention notification failed",
				"user_id", mentioned.ID, "issue_id", issue.ID, "error", err)
		}
		notified[mentioned.ID] = true
	}
}

func (s *CollaborationService) ListComments(ctx context.Context, issueID uuid.UUID) ([]*models.Comment, error) {
	return s.comments.ListByIssue(ctx, issueID)
}

func (s *CollaborationService) UpdateComment(ctx context.Context, id, authorID uuid.UUID, req *models.UpdateCommentRequest) (*models.Comment, error) {
	existing, err := s.comments.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, apperror.ErrNotFound
	}
	if existing.AuthorID != authorID {
		return nil, apperror.ErrForbidden
	}
	return s.comments.Update(ctx, id, req.Body)
}

func (s *CollaborationService) DeleteComment(ctx context.Context, id, authorID uuid.UUID) error {
	existing, err := s.comments.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return apperror.ErrNotFound
	}
	if existing.AuthorID != authorID {
		return apperror.ErrForbidden
	}
	return s.comments.Delete(ctx, id)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
