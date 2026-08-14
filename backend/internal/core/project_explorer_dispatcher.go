package core

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

type projectExplorerInvalidation struct {
	ID        int64
	EventKind string
	ProjectID string
	ActorID   string
}

func (s *Service) StartProjectExplorerInvalidationDispatcher(parent context.Context) func() {
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		cleanupTicker := time.NewTicker(10 * time.Minute)
		defer cleanupTicker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.dispatchProjectExplorerInvalidations(ctx); err != nil && ctx.Err() == nil {
					log.Printf("event=project_explorer_invalidation_dispatch_failed error=%v", err)
				}
			case <-cleanupTicker.C:
				if err := s.cleanupProjectExplorerInvalidations(ctx); err != nil && ctx.Err() == nil {
					log.Printf("event=project_explorer_invalidation_cleanup_failed error=%v", err)
				}
			}
		}
	}()
	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}
}

func (s *Service) cleanupProjectExplorerInvalidations(ctx context.Context) error {
	_, err := s.repo.db.ExecContext(ctx, `
		DELETE FROM project_explorer_invalidation_outbox
		WHERE id IN (
			SELECT id
			FROM project_explorer_invalidation_outbox
			WHERE published_at < now() - INTERVAL '7 days'
			ORDER BY id
			LIMIT 1000
		)
	`)
	return err
}

func (s *Service) dispatchProjectExplorerInvalidations(ctx context.Context) error {
	tx, err := s.repo.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `
		SELECT id, event_kind, COALESCE(project_id::text, ''), COALESCE(actor_id::text, '')
		FROM project_explorer_invalidation_outbox
		WHERE published_at IS NULL AND next_attempt_at <= now()
		ORDER BY id
		LIMIT 50
		FOR UPDATE SKIP LOCKED
	`)
	if err != nil {
		return err
	}
	events := make([]projectExplorerInvalidation, 0, 50)
	for rows.Next() {
		var event projectExplorerInvalidation
		if err := rows.Scan(&event.ID, &event.EventKind, &event.ProjectID, &event.ActorID); err != nil {
			rows.Close()
			return err
		}
		events = append(events, event)
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, event := range events {
		streamEvent := ActivityStreamEvent{
			Type:      "project_explorer_changed",
			ScopeType: event.EventKind,
			ActorID:   event.ActorID,
			At:        time.Now().UTC().Format(time.RFC3339),
		}
		if event.EventKind == "project" {
			streamEvent.ProjectID = event.ProjectID
		}
		publishErr := s.streamHub.PublishDurable(ctx, streamEvent)
		if publishErr == nil {
			if _, err := tx.ExecContext(ctx, `
				UPDATE project_explorer_invalidation_outbox
				SET published_at=now(), last_error=NULL
				WHERE id=$1
			`, event.ID); err != nil {
				return err
			}
			continue
		}
		errorText := strings.TrimSpace(publishErr.Error())
		if len(errorText) > 500 {
			errorText = errorText[:500]
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE project_explorer_invalidation_outbox
			SET attempt_count=attempt_count+1,
				next_attempt_at=now() + LEAST(INTERVAL '60 seconds', INTERVAL '1 second' * power(2, LEAST(attempt_count, 6))),
				last_error=$2
			WHERE id=$1
		`, event.ID, errorText); err != nil {
			return fmt.Errorf("record project explorer invalidation failure: %w", err)
		}
	}
	return tx.Commit()
}
