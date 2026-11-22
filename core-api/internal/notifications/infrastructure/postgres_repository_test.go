package infrastructure_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/notifications/application"
	"github.com/swiftdrop/core-api/internal/notifications/domain"
	"github.com/swiftdrop/core-api/internal/notifications/infrastructure"
	"github.com/swiftdrop/core-api/internal/platform/db"
)

func TestPostgresRepository_InterfaceConformance(t *testing.T) {
	repo := infrastructure.NewPostgresRepository(nil)
	var _ application.NotificationRepository = repo
}

func TestPostgresRepository_NilDB_ReturnsError(t *testing.T) {
	repo := infrastructure.NewPostgresRepository(nil)
	ctx := context.Background()

	_, _, err := repo.ListByUserID(ctx, application.ListFilter{UserID: "u1"})
	if !errors.Is(err, db.ErrNilDB) {
		t.Errorf("expected ErrNilDB on ListByUserID, got %v", err)
	}

	_, err = repo.GetUnreadCount(ctx, "u1", false)
	if !errors.Is(err, db.ErrNilDB) {
		t.Errorf("expected ErrNilDB on GetUnreadCount, got %v", err)
	}

	_, err = repo.FindByID(ctx, "id1")
	if !errors.Is(err, db.ErrNilDB) {
		t.Errorf("expected ErrNilDB on FindByID, got %v", err)
	}

	_, err = repo.MarkAsRead(ctx, "id1", "u1", false, time.Now())
	if !errors.Is(err, db.ErrNilDB) {
		t.Errorf("expected ErrNilDB on MarkAsRead, got %v", err)
	}

	_, err = repo.MarkAllAsRead(ctx, "u1", false, time.Now())
	if !errors.Is(err, db.ErrNilDB) {
		t.Errorf("expected ErrNilDB on MarkAllAsRead, got %v", err)
	}

	err = repo.Insert(ctx, &domain.Notification{})
	if !errors.Is(err, db.ErrNilDB) {
		t.Errorf("expected ErrNilDB on Insert, got %v", err)
	}
}
