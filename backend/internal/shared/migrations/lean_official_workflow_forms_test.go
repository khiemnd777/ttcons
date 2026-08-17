package migrations

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLeanOfficialWorkflowFormsPreserveHistoryAndRemapRev6(t *testing.T) {
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

	var leanForms, leanUniversalFields, rev5LegacySteps, rev6LeanSteps int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM workflow_form_versions
		WHERE version_no=2 AND schema_json->>'variant'='lean_capture_v2'
	`).Scan(&leanForms); err != nil {
		t.Fatal(err)
	}
	if leanForms != 45 {
		t.Fatalf("lean form count=%d, want 45", leanForms)
	}

	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM workflow_form_fields field
		JOIN workflow_form_versions version ON version.id=field.form_version_id
		WHERE version.schema_json->>'variant'='lean_capture_v2'
		  AND field.field_key IN ('scope_summary','work_result','linked_evidence_id','next_action','confirmed')
	`).Scan(&leanUniversalFields); err != nil {
		t.Fatal(err)
	}
	if leanUniversalFields != 0 {
		t.Fatalf("lean universal field count=%d, want 0", leanUniversalFields)
	}

	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM pipeline_template_steps step
		JOIN pipeline_template_nodes node ON node.id=step.node_id
		JOIN workflow_form_versions version ON version.id=step.form_version_id
		WHERE node.template_id='40000000-0000-0000-0000-000000000024'
		  AND version.version_no=1
		  AND version.validation_json->>'template_id'='40000000-0000-0000-0000-000000000020'
	`).Scan(&rev5LegacySteps); err != nil {
		t.Fatal(err)
	}
	if rev5LegacySteps != 43 {
		t.Fatalf("Rev5 legacy business-form step count=%d, want 43", rev5LegacySteps)
	}

	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM pipeline_template_steps step
		JOIN pipeline_template_nodes node ON node.id=step.node_id
		JOIN workflow_form_versions version ON version.id=step.form_version_id
		WHERE node.template_id='40000000-0000-0000-0000-000000000025'
		  AND version.schema_json->>'variant'='lean_capture_v2'
	`).Scan(&rev6LeanSteps); err != nil {
		t.Fatal(err)
	}
	if rev6LeanSteps != 43 {
		t.Fatalf("Rev6 lean business-form step count=%d, want 43", rev6LeanSteps)
	}

	rows, err := database.QueryContext(ctx, `
		SELECT template.code,field.field_key,field.field_type,field.required,
		       field.validation_json->'required_when'->>'value',
		       field.visibility_condition_json->>'value'
		FROM workflow_form_templates template
		JOIN workflow_form_versions version ON version.template_id=template.id
		JOIN workflow_form_fields field ON field.form_version_id=version.id
		WHERE version.version_no=2
		  AND version.schema_json->>'variant'='lean_capture_v2'
		  AND template.code IN ('client_output_invoice_issue','client_input_invoice_closure')
		ORDER BY template.code,field.order_index
	`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	type invoiceField struct {
		code       string
		key        string
		fieldType  string
		required   bool
		requiredOn *string
		visibleOn  *string
	}
	var invoiceFields []invoiceField
	for rows.Next() {
		var field invoiceField
		if err := rows.Scan(&field.code, &field.key, &field.fieldType, &field.required, &field.requiredOn, &field.visibleOn); err != nil {
			t.Fatal(err)
		}
		invoiceFields = append(invoiceFields, field)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(invoiceFields) != 6 {
		t.Fatalf("invoice lean field count=%d, want 6", len(invoiceFields))
	}
	for _, field := range invoiceFields {
		if field.key == "invoice_status" {
			if !field.required || field.fieldType != "select" {
				t.Fatalf("%s status field=%+v, want required select", field.code, field)
			}
			continue
		}
		wantState := "issued"
		if field.code == "client_input_invoice_closure" {
			wantState = "received"
		}
		if field.required || field.requiredOn == nil || *field.requiredOn != wantState || field.visibleOn == nil || *field.visibleOn != wantState {
			t.Fatalf("conditional invoice field=%+v, want visible/required on %q", field, wantState)
		}
		if field.key == "invoice_amount" && field.fieldType != "currency" {
			t.Fatalf("%s invoice amount type=%q, want currency", field.code, field.fieldType)
		}
	}

	migrationSQL, err := os.ReadFile(filepath.Join(migrationsDir, "V102__lean_official_workflow_forms.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, string(migrationSQL)); err != nil {
		t.Fatalf("reapply lean workflow reference migration: %v", err)
	}
	var leanFormsAfterReapply int
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM workflow_form_versions
		WHERE version_no=2 AND schema_json->>'variant'='lean_capture_v2'
	`).Scan(&leanFormsAfterReapply); err != nil {
		t.Fatal(err)
	}
	if leanFormsAfterReapply != leanForms {
		t.Fatalf("lean forms after reapply=%d, want %d", leanFormsAfterReapply, leanForms)
	}
}
