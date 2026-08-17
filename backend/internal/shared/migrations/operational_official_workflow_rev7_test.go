package migrations

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestOperationalOfficialWorkflowRev7PreservesHistoryAndReducesCapture(t *testing.T) {
	dsn := requireMigrationTestDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	schema, cleanup := createMigrationTestSchema(t, ctx, dsn)
	defer cleanup()
	database := openMigrationTestDatabase(t, ctx, dsn, schema)
	defer database.Close()

	migrationsDir := filepath.Join("..", "..", "..", "migrations")
	if _, err := Run(ctx, database, migrationsDir); err != nil {
		t.Fatal(err)
	}

	var rev6Steps, rev7Nodes, rev7Steps, rev7FieldOccurrences int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pipeline_template_steps step
		JOIN pipeline_template_nodes node ON node.id=step.node_id
		WHERE node.template_id='40000000-0000-0000-0000-000000000025'
	`).Scan(&rev6Steps); err != nil {
		t.Fatal(err)
	}
	if rev6Steps != 51 {
		t.Fatalf("historical Rev6 step count=%d, want 51", rev6Steps)
	}

	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM pipeline_template_nodes WHERE template_id='40000000-0000-0000-0000-000000000026'`).Scan(&rev7Nodes); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pipeline_template_steps step
		JOIN pipeline_template_nodes node ON node.id=step.node_id
		WHERE node.template_id='40000000-0000-0000-0000-000000000026'
	`).Scan(&rev7Steps); err != nil {
		t.Fatal(err)
	}
	if rev7Nodes != 15 || rev7Steps != 50 {
		t.Fatalf("Rev7 topology=%d nodes/%d steps, want 15/50", rev7Nodes, rev7Steps)
	}

	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM pipeline_template_nodes node
		JOIN pipeline_template_steps step ON step.node_id=node.id
		JOIN workflow_form_fields field ON field.form_version_id=step.form_version_id
		WHERE node.template_id='40000000-0000-0000-0000-000000000026'
	`).Scan(&rev7FieldOccurrences); err != nil {
		t.Fatal(err)
	}
	if rev7FieldOccurrences != 87 {
		t.Fatalf("Rev7 field occurrences=%d, want 87", rev7FieldOccurrences)
	}

	var duplicateAssignment, emptyRequiredForms, removedFields, optionalFields, defaultedFields int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pipeline_template_steps step
		JOIN pipeline_template_nodes node ON node.id=step.node_id
		WHERE node.template_id='40000000-0000-0000-0000-000000000026'
		  AND step.business_code='planning.assignment_meeting'
	`).Scan(&duplicateAssignment); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
			SELECT step.id
			FROM pipeline_template_steps step
			JOIN pipeline_template_nodes node ON node.id=step.node_id
			LEFT JOIN workflow_form_fields field ON field.form_version_id=step.form_version_id
			WHERE node.template_id='40000000-0000-0000-0000-000000000026' AND step.form_required
			GROUP BY step.id HAVING COUNT(field.id)=0
		) empty_forms
	`).Scan(&emptyRequiredForms); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM pipeline_template_nodes node
		JOIN pipeline_template_steps step ON step.node_id=node.id
		JOIN workflow_form_fields field ON field.form_version_id=step.form_version_id
		WHERE node.template_id='40000000-0000-0000-0000-000000000026'
		  AND field.field_key IN ('confirmed','progress_percent','site_photo_evidence_id','output_invoice_no','boq_version','quote_version','invoice_status')
	`).Scan(&removedFields); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT field.id)
		FROM workflow_form_versions version
		JOIN workflow_form_fields field ON field.form_version_id=version.id
		WHERE version.schema_json->>'variant'='operational_capture_v3'
		  AND field.validation_json->>'capture_mode'='optional'
	`).Scan(&optionalFields); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT field.id)
		FROM workflow_form_versions version
		JOIN workflow_form_fields field ON field.form_version_id=version.id
		WHERE version.schema_json->>'variant'='operational_capture_v3'
		  AND field.validation_json ? 'default_source'
	`).Scan(&defaultedFields); err != nil {
		t.Fatal(err)
	}
	if duplicateAssignment != 0 || emptyRequiredForms != 0 || removedFields != 0 {
		t.Fatalf("Rev7 invariant failed: duplicate=%d empty_forms=%d removed_fields=%d", duplicateAssignment, emptyRequiredForms, removedFields)
	}
	if optionalFields == 0 || defaultedFields == 0 {
		t.Fatalf("Rev7 metadata missing: optional=%d defaulted=%d", optionalFields, defaultedFields)
	}
}
