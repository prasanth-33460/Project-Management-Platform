package service

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/prasanth-33460/Project-Management-Platform/internal/models"
	"github.com/prasanth-33460/Project-Management-Platform/internal/repository"
)

type NotificationService struct {
	notifs repository.NotificationStore
}

func NewNotificationService(notifs repository.NotificationStore) *NotificationService {
	return &NotificationService{notifs: notifs}
}

func (s *NotificationService) List(ctx context.Context, userID uuid.UUID, cursor string, limit int) (*models.NotificationListResponse, error) {
	items, _, err := s.notifs.List(ctx, userID, cursor, limit)
	if err != nil {
		return nil, err
	}

	var nextCursor string
	if len(items) > 0 && len(items) == limit {
		last := items[len(items)-1]
		nextCursor = models.EncodeCursor(last.CreatedAt, last.ID)
	}

	unread, err := s.notifs.UnreadCount(ctx, userID)
	if err != nil {
		slog.WarnContext(ctx, "unread count failed", "user_id", userID, "error", err)
	}

	return &models.NotificationListResponse{
		Items:       items,
		NextCursor:  nextCursor,
		UnreadCount: unread,
	}, nil
}

func (s *NotificationService) MarkRead(ctx context.Context, id, userID uuid.UUID) error {
	return s.notifs.MarkRead(ctx, id, userID)
}

func (s *NotificationService) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return s.notifs.MarkAllRead(ctx, userID)
}
