package application

import (
	"context"
	"strings"
	"time"

	"github.com/swiftdrop/core-api/internal/notifications/domain"
)

// Service provides use cases for managing in-app notifications.
type Service struct {
	repo NotificationRepository
}

// NewService constructs a notifications application service.
func NewService(repo NotificationRepository) *Service {
	return &Service{repo: repo}
}

// ListNotificationsQuery encapsulates query arguments for listing notifications.
type ListNotificationsQuery struct {
	UserID       string
	IsDispatcher bool
	Cursor       *PaginationCursor
	Limit        int
}

// ListNotifications retrieves a page of notifications for the authenticated user.
func (s *Service) ListNotifications(ctx context.Context, query ListNotificationsQuery) ([]*domain.Notification, *PaginationCursor, error) {
	if strings.TrimSpace(query.UserID) == "" {
		return nil, nil, domain.ErrEmptyUserID
	}
	if query.Limit <= 0 {
		query.Limit = 20
	}
	if query.Limit > 100 {
		query.Limit = 100
	}

	filter := ListFilter{
		UserID:       strings.TrimSpace(query.UserID),
		IsDispatcher: query.IsDispatcher,
		Cursor:       query.Cursor,
		Limit:        query.Limit,
	}

	return s.repo.ListByUserID(ctx, filter)
}

// GetUnreadCount returns the number of unread notifications for the user.
func (s *Service) GetUnreadCount(ctx context.Context, userID string, isDispatcher bool) (int, error) {
	if strings.TrimSpace(userID) == "" {
		return 0, domain.ErrEmptyUserID
	}
	return s.repo.GetUnreadCount(ctx, strings.TrimSpace(userID), isDispatcher)
}

// FindByID retrieves a single notification, verifying user authorization.
func (s *Service) FindByID(ctx context.Context, id, userID string, isDispatcher bool) (*domain.Notification, error) {
	if strings.TrimSpace(id) == "" {
		return nil, domain.ErrInvalidNotificationID
	}
	if strings.TrimSpace(userID) == "" {
		return nil, domain.ErrEmptyUserID
	}

	notif, err := s.repo.FindByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	if notif == nil {
		return nil, domain.ErrNotificationNotFound
	}
	if !notif.IsForUser(userID, isDispatcher) {
		return nil, domain.ErrUnauthorizedAccess
	}
	return notif, nil
}

// MarkAsRead marks a specific notification as read, enforcing domain invariants and ownership.
func (s *Service) MarkAsRead(ctx context.Context, id, userID string, isDispatcher bool) (*domain.Notification, error) {
	if strings.TrimSpace(id) == "" {
		return nil, domain.ErrInvalidNotificationID
	}
	if strings.TrimSpace(userID) == "" {
		return nil, domain.ErrEmptyUserID
	}

	notif, err := s.repo.FindByID(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, err
	}
	if notif == nil {
		return nil, domain.ErrNotificationNotFound
	}
	if !notif.IsForUser(userID, isDispatcher) {
		return nil, domain.ErrUnauthorizedAccess
	}

	now := time.Now().UTC()
	if err := notif.MarkRead(now); err != nil {
		return nil, err
	}

	return s.repo.MarkAsRead(ctx, strings.TrimSpace(id), strings.TrimSpace(userID), isDispatcher, now)
}

// MarkAllAsRead marks all unread notifications for the user as read.
func (s *Service) MarkAllAsRead(ctx context.Context, userID string, isDispatcher bool) (int64, error) {
	if strings.TrimSpace(userID) == "" {
		return 0, domain.ErrEmptyUserID
	}
	now := time.Now().UTC()
	return s.repo.MarkAllAsRead(ctx, strings.TrimSpace(userID), isDispatcher, now)
}
