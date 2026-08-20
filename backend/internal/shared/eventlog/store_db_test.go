package eventlog

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestStorePersistsSanitizedEventsAndDeletesExpiredRows(t *testing.T) {
	dsn := os.Getenv("CONSFLOW_TEST_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("CI") == "true" {
			t.Fatal("CONSFLOW_TEST_DATABASE_URL is required in CI; event log persistence must not skip")
		}
		t.Skip("set CONSFLOW_TEST_DATABASE_URL to run DB-backed event log tests")
	}
	database, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	database.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	schema := "eventlog_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := database.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = database.ExecContext(context.Background(), `SET search_path TO public`)
		_, _ = database.ExecContext(context.Background(), `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	})
	if _, err := database.ExecContext(ctx, `SET search_path TO `+schema); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `
		CREATE TABLE event_logs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			category TEXT NOT NULL,
			http_status INTEGER NOT NULL,
			error_code TEXT NOT NULL,
			method TEXT NOT NULL,
			route_template TEXT NOT NULL,
			request_class TEXT NOT NULL,
			request_id TEXT NOT NULL,
			actor_id UUID,
			actor_email TEXT,
			project_id UUID,
			duration_ms BIGINT NOT NULL,
			error_type TEXT,
			error_fingerprint TEXT NOT NULL,
			internal_message TEXT,
			stack_trace TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		t.Fatal(err)
	}

	store := NewStore(database)
	expiredRequestID := "event-log-expired-" + uuid.NewString()
	freshRequestID := "event-log-fresh-" + uuid.NewString()
	for _, requestID := range []string{expiredRequestID, freshRequestID} {
		if err := store.Record(ctx, Entry{
			Category: CategoryUnexpected5xx, HTTPStatus: 500, ErrorCode: "INTERNAL_ERROR",
			Method: "POST", RouteTemplate: "/api/projects/:id/example", RequestClass: "general",
			RequestID: requestID, ErrorType: "error",
			InternalMessage: "password=private-value email=user@example.com",
			StackTrace:      "Authorization: Bearer private-token",
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := database.ExecContext(ctx, `UPDATE event_logs SET created_at=now()-INTERVAL '91 days' WHERE request_id=$1`, expiredRequestID); err != nil {
		t.Fatal(err)
	}

	var message, stack string
	if err := database.QueryRowContext(ctx, `SELECT internal_message,stack_trace FROM event_logs WHERE request_id=$1`, freshRequestID).Scan(&message, &stack); err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"private-value", "user@example.com", "private-token"} {
		if strings.Contains(message, forbidden) || strings.Contains(stack, forbidden) {
			t.Fatalf("persisted event contains forbidden value %q: message=%q stack=%q", forbidden, message, stack)
		}
	}

	deleted, err := store.DeleteExpired(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if deleted < 1 {
		t.Fatalf("deleted=%d, want at least the expired fixture", deleted)
	}
	var expiredCount, freshCount int
	if err := database.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE request_id=$1),
			COUNT(*) FILTER (WHERE request_id=$2)
		FROM event_logs
	`, expiredRequestID, freshRequestID).Scan(&expiredCount, &freshCount); err != nil {
		t.Fatal(err)
	}
	if expiredCount != 0 || freshCount != 1 {
		t.Fatalf("retention result expired=%d fresh=%d", expiredCount, freshCount)
	}
}
