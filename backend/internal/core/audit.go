package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
)

type auditExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type auditEvent struct {
	ProjectID   string
	ActorID     string
	ActorName   string
	EventType   string
	TargetType  string
	TargetID    string
	Description string
	Metadata    any
}

func writeAudit(ctx context.Context, exec auditExecer, event auditEvent) error {
	raw, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("marshal audit metadata for %s: %w", event.EventType, err)
	}
	var projectID any
	var targetID any
	if event.ProjectID != "" {
		projectID = event.ProjectID
	}
	if event.TargetID != "" {
		targetID = event.TargetID
	}
	if _, err := exec.ExecContext(ctx, `
		INSERT INTO audit_logs (project_id, actor_id, actor_name, event_type, target_type, target_id, description, metadata_json)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4, $5, $6, $7, $8::jsonb)
	`, projectID, event.ActorID, event.ActorName, event.EventType, event.TargetType, targetID, event.Description, string(raw)); err != nil {
		return fmt.Errorf("write audit event %s: %w", event.EventType, err)
	}
	return nil
}

func (r *Repository) AddAudit(ctx context.Context, projectID, actorID, actorName, eventType, targetType, targetID, description string, metadata any) error {
	return writeAudit(ctx, r.db, auditEvent{
		ProjectID: projectID, ActorID: actorID, ActorName: actorName, EventType: eventType,
		TargetType: targetType, TargetID: targetID, Description: description, Metadata: metadata,
	})
}

func (r *Repository) AddAuditBestEffort(ctx context.Context, projectID, actorID, actorName, eventType, targetType, targetID, description string, metadata any) {
	writeAuditBestEffort(ctx, r.db, auditEvent{
		ProjectID: projectID, ActorID: actorID, ActorName: actorName, EventType: eventType,
		TargetType: targetType, TargetID: targetID, Description: description, Metadata: metadata,
	})
}

func writeAuditBestEffort(ctx context.Context, exec auditExecer, event auditEvent) {
	if err := writeAudit(ctx, exec, event); err != nil {
		log.Printf("level=error component=audit policy=best_effort event_type=%q target_type=%q target_id=%q project_id=%q actor_id=%q error=%q", event.EventType, event.TargetType, event.TargetID, event.ProjectID, event.ActorID, err)
	}
}

func addAuditTx(ctx context.Context, tx *sql.Tx, projectID, actorID, actorName, eventType, targetType, targetID, description string, metadata any) error {
	return writeAudit(ctx, tx, auditEvent{
		ProjectID: projectID, ActorID: actorID, ActorName: actorName, EventType: eventType,
		TargetType: targetType, TargetID: targetID, Description: description, Metadata: metadata,
	})
}

func (r *Repository) withAuditTransaction(
	ctx context.Context,
	mutation func(*sql.Tx) (Record, error),
	eventFor func(Record) auditEvent,
) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	row, err := mutation(tx)
	if err != nil {
		return nil, err
	}
	event := eventFor(row)
	if event.TargetID == "" {
		event.TargetID = cleanID(row["id"])
	}
	if err := writeAudit(ctx, tx, event); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return row, nil
}

func (r *Repository) insertWithAudit(ctx context.Context, table string, specs []ColumnSpec, body Record, eventFor func(Record) auditEvent) (Record, error) {
	return r.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
		return insertRecord(ctx, tx, table, specs, body)
	}, eventFor)
}

func (r *Repository) updateWithAudit(ctx context.Context, table string, specs []ColumnSpec, id string, body Record, eventFor func(Record) auditEvent) (Record, error) {
	return r.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
		return updateRecord(ctx, tx, table, specs, id, body)
	}, eventFor)
}
