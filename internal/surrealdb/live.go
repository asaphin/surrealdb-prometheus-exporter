package surrealdb

import (
	"context"
	"errors"
	"log/slog"
	"time"

	sdk "github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

type LiveQuery struct {
	db *sdk.DB
}

func NewLiveQuery(db *sdk.DB) (*LiveQuery, error) {
	if db == nil {
		return nil, errors.New("db argument cannot be nil")
	}

	return &LiveQuery{db: db}, nil
}

func (q *LiveQuery) Run(tableName string) {
	ctx := context.Background()

	slog.Info("Starting live query", "table", tableName)

	live, err := sdk.Live(ctx, q.db, models.Table(tableName), false)
	if err != nil {
		slog.Error("error running live query", "error", err)
		return
	}

	slog.Info("Live query registered", "uuid", live.String())

	notifications, err := q.db.LiveNotifications(live.String())
	if err != nil {
		slog.Error("error getting live notifications", "error", err)
		return // CRITICAL: Must return on error
	}

	if notifications == nil {
		slog.Error("notifications channel is nil")
		return
	}

	slog.Info("live notifications channel obtained, entering loop")

	// Add periodic heartbeat to prove the loop is running
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case notification, ok := <-notifications:
			if !ok {
				slog.Warn("notifications channel closed")
				return
			}
			slog.Info("NOTIFICATION RECEIVED!",
				slog.String("action", string(notification.Action)),
				slog.Any("id", notification.ID),
				slog.Any("result", notification.Result))

		case <-ticker.C:
			slog.Info("Live query still active, waiting for notifications...")
		}
	}
}
