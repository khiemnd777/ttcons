package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Repository struct {
	db             *sql.DB
	systemSettings *SystemSettingsStore
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ConfigureSystemSettings(store *SystemSettingsStore) {
	r.systemSettings = store
}

func (r *Repository) Query(ctx context.Context, query string, args ...any) ([]Record, error) {
	return queryRecords(ctx, r.db, query, args...)
}

const readTransactionSettingsSQL = `SET LOCAL jit = off`

func (r *Repository) beginReadTransactionWithJITDisabled(ctx context.Context) (*sql.Tx, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	// Actor-scoped operational queues can have complex access predicates whose
	// JIT compilation costs more than their bounded execution. Keep the setting
	// local to this read transaction so connection and server defaults are intact.
	if _, err := tx.ExecContext(ctx, readTransactionSettingsSQL); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return tx, nil
}

func queryRecords(ctx context.Context, db queryContext, query string, args ...any) ([]Record, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

func (r *Repository) QueryOne(ctx context.Context, query string, args ...any) (Record, error) {
	return queryOneRecord(ctx, r.db, query, args...)
}

func queryOneRecord(ctx context.Context, db queryContext, query string, args ...any) (Record, error) {
	rows, err := queryRecords(ctx, db, query, args...)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	return rows[0], nil
}

func (r *Repository) Exec(ctx context.Context, query string, args ...any) error {
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

func scanRows(rows *sql.Rows) ([]Record, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := make([]Record, 0)
	for rows.Next() {
		values := make([]any, len(cols))
		refs := make([]any, len(cols))
		for i := range values {
			refs[i] = &values[i]
		}
		if err := rows.Scan(refs...); err != nil {
			return nil, err
		}
		row := Record{}
		for i, col := range cols {
			row[col] = normalizeValue(values[i])
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func normalizeValue(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		text := string(v)
		var parsed any
		if (strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[")) && json.Unmarshal(v, &parsed) == nil {
			return parsed
		}
		return text
	case time.Time:
		return v.Format(time.RFC3339)
	default:
		return v
	}
}

func (r *Repository) Insert(ctx context.Context, table string, specs []ColumnSpec, body Record) (Record, error) {
	return insertRecord(ctx, r.db, table, specs, body)
}

type queryContext interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type storedMutationError struct {
	err         error
	cleanupSafe bool
}

func (e *storedMutationError) Error() string { return e.err.Error() }
func (e *storedMutationError) Unwrap() error { return e.err }

type storedMutationTracker struct {
	commitAttempted bool
}

func (t *storedMutationTracker) wrap(err *error) {
	if err == nil || *err == nil {
		return
	}
	*err = &storedMutationError{err: *err, cleanupSafe: !t.commitAttempted}
}

func (t *storedMutationTracker) commit(commit func() error) error {
	t.commitAttempted = true
	return commit()
}

func storedMutationCleanupSafe(err error) bool {
	var mutationErr *storedMutationError
	return errors.As(err, &mutationErr) && mutationErr.cleanupSafe
}

func finalizeStoredMutation(tracker *storedMutationTracker, loadDetail func() (Record, error), commit func() error) (Record, error) {
	detail, err := loadDetail()
	if err != nil {
		return nil, err
	}
	if err := tracker.commit(commit); err != nil {
		return nil, err
	}
	return detail, nil
}

func insertRecord(ctx context.Context, db queryContext, table string, specs []ColumnSpec, body Record) (Record, error) {
	table = safeTable(table)
	if table == "" {
		return nil, errors.New("invalid table")
	}
	cols := make([]string, 0)
	holders := make([]string, 0)
	args := make([]any, 0)
	for _, spec := range specs {
		value, ok := body[spec.Name]
		if !ok || value == nil {
			continue
		}
		if text, ok := value.(string); ok && text == "" {
			continue
		}
		cols = append(cols, spec.Name)
		args = append(args, prepareValue(spec, value))
		holder := fmt.Sprintf("$%d", len(args))
		if spec.Cast != "" {
			holder += "::" + spec.Cast
		}
		holders = append(holders, holder)
	}
	if len(cols) == 0 {
		return nil, errors.New("no fields to insert")
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING *", table, strings.Join(cols, ","), strings.Join(holders, ","))
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records, err := scanRows(rows)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, sql.ErrNoRows
	}
	return records[0], nil
}

func (r *Repository) Update(ctx context.Context, table string, specs []ColumnSpec, id string, body Record) (Record, error) {
	return updateRecord(ctx, r.db, table, specs, id, body)
}

func updateRecord(ctx context.Context, db queryContext, table string, specs []ColumnSpec, id string, body Record) (Record, error) {
	table = safeTable(table)
	if table == "" {
		return nil, errors.New("invalid table")
	}
	sets := make([]string, 0)
	args := make([]any, 0)
	for _, spec := range specs {
		if spec.Name == "id" {
			continue
		}
		value, ok := body[spec.Name]
		if !ok {
			continue
		}
		args = append(args, prepareValue(spec, value))
		holder := fmt.Sprintf("$%d", len(args))
		if spec.Cast != "" {
			holder += "::" + spec.Cast
		}
		sets = append(sets, fmt.Sprintf("%s=%s", spec.Name, holder))
	}
	if len(sets) == 0 {
		rows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM %s WHERE id=$1", table), id)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		records, err := scanRows(rows)
		if err != nil {
			return nil, err
		}
		if len(records) == 0 {
			return nil, sql.ErrNoRows
		}
		return records[0], nil
	}
	if hasUpdatedAt(table) {
		sets = append(sets, "updated_at=now()")
	}
	args = append(args, id)
	query := fmt.Sprintf("UPDATE %s SET %s WHERE id=$%d RETURNING *", table, strings.Join(sets, ","), len(args))
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records, err := scanRows(rows)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, sql.ErrNoRows
	}
	return records[0], nil
}

func hasUpdatedAt(table string) bool {
	switch table {
	case "users", "staff", "projects", "pipeline_templates", "pipeline_template_nodes", "pipeline_run_nodes",
		"procurement_requests", "boq_items", "issues", "acceptance_records", "rework_requests", "replanning_requests",
		"construction_check_templates", "construction_checks", "construction_action_items",
		"handover_records", "accounting_budgets", "payables", "receivables", "payments", "advance_requests",
		"accounting_books", "accounting_accounts", "accounting_periods", "journal_entries", "accounting_posting_rules",
		"tax_codes", "tax_periods", "invoice_tax_lines",
		"system_settings", "work_items", "evidence_records", "workflow_form_templates", "workflow_form_versions",
		"workflow_form_instances", "workflow_form_signatures", "input_invoices", "output_invoices",
		"payable_payment_schedules", "receivable_payment_schedules", "client_advance_receipts", "partners":
		return true
	default:
		return false
	}
}

func prepareValue(spec ColumnSpec, value any) any {
	if spec.Cast == "jsonb" {
		raw, _ := json.Marshal(value)
		return string(raw)
	}
	return value
}

func (r *Repository) GetByID(ctx context.Context, table, id string) (Record, error) {
	table = safeTable(table)
	if table == "" {
		return nil, errors.New("invalid table")
	}
	return r.QueryOne(ctx, fmt.Sprintf("SELECT * FROM %s WHERE id=$1", table), id)
}

func (r *Repository) ListTable(ctx context.Context, table string, limit int) ([]Record, error) {
	table = safeTable(table)
	if table == "" {
		return nil, errors.New("invalid table")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return r.Query(ctx, fmt.Sprintf("SELECT * FROM %s ORDER BY created_at DESC LIMIT $1", table), limit)
}

func safeTable(table string) string {
	switch table {
	case "projects", "pipeline_templates", "pipeline_template_nodes", "pipeline_template_edges", "pipeline_template_steps",
		"pipeline_runs", "pipeline_run_nodes", "pipeline_run_edges", "pipeline_run_steps", "approval_requests", "approval_actions",
		"boq_items", "supplier_quotations", "procurement_requests", "procurement_request_items", "contracts", "warehouses", "materials", "stock_movements",
		"site_material_balances", "operation_daily_reports", "issues", "acceptance_records", "acceptance_checklist_items",
		"construction_check_templates", "construction_checks", "construction_action_items", "construction_action_events",
		"rework_requests", "replanning_requests", "handover_records", "accounting_budgets", "payables", "receivables",
		"payments", "advance_requests", "documents", "audit_logs", "system_settings", "staff", "roles", "permissions",
		"role_permissions", "user_roles", "users", "work_items", "work_item_assignments", "work_item_activities",
		"evidence_records", "workflow_form_templates", "workflow_form_versions", "workflow_form_fields",
		"workflow_form_instances", "workflow_form_answers", "workflow_form_events", "workflow_form_signatures",
		"workflow_form_pdf_exports", "boq_unit_price_events", "input_invoices", "output_invoices",
		"payable_payment_schedules", "payment_allocations", "receivable_payment_schedules", "client_advance_receipts",
		"receipt_allocations", "partners", "accounting_books", "accounting_accounts", "accounting_periods",
		"journal_entries", "journal_entry_lines", "accounting_posting_rules", "budget_movements", "tax_codes",
		"tax_periods", "invoice_tax_lines", "tax_adjustments", "accounting_backfill_issues",
		"project_material_costs", "project_cash_requests", "accounting_handoffs":
		return table
	default:
		return ""
	}
}

func newID() string {
	return uuid.NewString()
}

func projectColumns() []ColumnSpec {
	return []ColumnSpec{col("name"), col("code"), col("partner_id"), col("client_name"), col("address"), col("planned_start_date"), col("planned_end_date"), col("estimated_budget"), col("contract_amount"), col("status"), col("pipeline_template_id"), col("pm_staff_id"), col("progress_percent"), col("workflow_entry_mode"), col("created_by")}
}

func partnerColumns() []ColumnSpec {
	return []ColumnSpec{col("partner_code"), col("name"), col("legal_name"), jsonCol("partner_types"), col("tax_code"), col("phone"), col("email"), col("address"), col("contact_name"), col("payment_terms_days"), col("notes"), col("active")}
}

func templateColumns() []ColumnSpec {
	return []ColumnSpec{col("id"), col("name"), col("description"), col("project_type"), col("version"), col("status"), col("created_by"), col("template_family_id"), col("revision_no"), col("source_template_id"), col("engine_version"), col("definition_hash"), col("published_at"), col("published_by"), jsonCol("report_config_json")}
}

func templateNodeColumns() []ColumnSpec {
	return []ColumnSpec{col("template_id"), col("business_code"), col("name"), col("description"), col("type"), col("position_x"), col("position_y"), col("order_index"), jsonCol("responsible_role_ids"), col("required_permission"), jsonCol("permissions"), jsonCol("entry_conditions"), jsonCol("exit_conditions"), col("approval_rule"), jsonCol("approval_policy_json"), col("rework_enabled"), col("replanning_enabled"), col("fail_path"), col("status")}
}

func templateStepColumns() []ColumnSpec {
	return []ColumnSpec{col("node_id"), col("business_code"), col("name"), col("description"), col("step_type"), jsonCol("required_role_ids"), col("required_permission"), col("checklist_required"), col("document_required"), col("photo_required"), col("form_version_id"), col("form_required"), col("surface_kind"), col("domain_action_key"), col("order_index")}
}

func workItemColumns() []ColumnSpec {
	return []ColumnSpec{
		col("project_id"), col("pipeline_run_id"), col("run_node_id"), col("run_step_id"), col("source_type"), col("source_id"),
		col("title"), col("description"), col("work_type"), col("status"), col("priority"), col("department"), jsonCol("role_ids"),
		col("assignee_staff_id"), jsonCol("watcher_staff_ids"), col("planned_start_at"), col("due_at"), col("started_at"),
		col("completed_at"), col("blocked_at"), col("blocked_reason"), jsonCol("completion_payload_json"),
		jsonCol("evidence_requirement_json"), col("approval_request_id"), col("created_by"), col("updated_by"),
	}
}

func documentColumns() []ColumnSpec {
	return []ColumnSpec{
		col("project_id"), col("target_type"), col("target_id"), col("name"), col("file_url"),
		col("document_type"), col("status"), col("uploaded_by"), col("mime_type"), col("file_size"),
		col("storage_provider"), col("storage_key"), col("checksum_sha256"),
	}
}

func edgeColumns() []ColumnSpec {
	return []ColumnSpec{col("template_id"), col("from_node_id"), col("to_node_id"), col("condition_type"), jsonCol("condition_config"), col("label"), col("priority")}
}

func boqColumns() []ColumnSpec {
	return []ColumnSpec{
		col("project_id"), col("category"), col("work_item_name"), col("description"), col("unit"), col("quantity"),
		col("unit_price"), col("expected_unit_price"), col("material_id"), col("node_id"), col("status"), col("created_by"),
		col("approved_by"),
	}
}

func procurementColumns() []ColumnSpec {
	return []ColumnSpec{
		col("project_id"), col("request_code"), col("title"), col("requested_by"), col("status"),
		col("total_estimated_amount"), col("budget_status"), col("approval_request_id"), col("supplier_partner_id"),
		col("supplier_name"), col("order_reference"), col("expected_delivery_date"),
	}
}

func procurementItemColumns() []ColumnSpec {
	return []ColumnSpec{
		col("request_id"), col("material_id"), col("boq_item_id"), col("node_id"), col("quantity_planned"),
		col("quantity_requested"), col("unit"), col("estimated_unit_price"), col("supplier_partner_id"), col("supplier_name"), col("delivery_status"),
		col("approval_status"), col("boq_price_source"), col("boq_price_revision_no"), col("boq_price_event_id"),
		col("supplier_quotation_item_id"), col("quoted_unit_price"), col("price_variance_amount"),
	}
}

func materialColumns() []ColumnSpec {
	return []ColumnSpec{col("code"), col("name"), col("unit"), col("category"), col("default_supplier_partner_id"), col("default_supplier"), col("standard_price"), col("active")}
}

func warehouseColumns() []ColumnSpec {
	return []ColumnSpec{col("name"), col("location"), col("type"), col("active")}
}

func dailyReportColumns() []ColumnSpec {
	return []ColumnSpec{
		col("project_id"), col("report_date"), col("reported_by"), col("weather"), col("work_summary"),
		col("manpower_summary"), col("material_usage_summary"), col("issues_summary"), col("progress_percent"),
		jsonCol("photos"), col("work_item_id"), jsonCol("structured_json"), col("primary_evidence_id"),
	}
}

func issueColumns() []ColumnSpec {
	return []ColumnSpec{col("project_id"), col("node_id"), col("title"), col("description"), col("severity"), col("status"), col("reported_by"), col("assigned_to"), col("requires_rework"), col("requires_replanning")}
}

func acceptanceColumns() []ColumnSpec {
	return []ColumnSpec{
		col("project_id"), col("node_id"), col("work_item_id"), col("form_instance_id"), col("pdf_export_id"),
		col("type"), col("title"), col("requested_by"), col("inspected_by"), col("approved_by"),
		col("status"), col("result"), col("acceptance_date"), col("location"), col("client_representative"),
		col("contractor_representative"), col("scope_summary"), col("signature_status"), col("document_status"),
		col("notes"), jsonCol("summary_json"),
	}
}

func reworkColumns() []ColumnSpec {
	return []ColumnSpec{col("project_id"), col("issue_id"), col("acceptance_id"), col("node_id"), col("title"), col("description"), col("assigned_to"), col("status"), col("due_date"), col("created_by"), col("approved_by")}
}

func replanningColumns() []ColumnSpec {
	return []ColumnSpec{col("project_id"), col("issue_id"), col("node_id"), col("reason"), col("impact_scope"), col("budget_impact"), col("timeline_impact"), col("status"), col("created_by"), col("approved_by")}
}

func handoverColumns() []ColumnSpec {
	return []ColumnSpec{col("project_id"), col("handover_code"), col("status"), col("final_acceptance_id"), col("workflow_form_instance_id"), col("handover_date"), col("client_representative"), col("company_representative"), col("documents_ready"), col("warehouse_closed"), col("payables_recorded"), col("receivables_recorded"), jsonCol("readiness_snapshot_json"), col("notes"), col("created_by"), col("approved_by")}
}

func budgetColumns() []ColumnSpec {
	return []ColumnSpec{col("project_id"), col("accounting_book_id"), col("budget_code"), col("name"), col("planned_amount"), col("approved_amount"), col("used_amount"), col("remaining_amount"), col("status"), col("approved_by")}
}

func payableColumns() []ColumnSpec {
	return []ColumnSpec{col("project_id"), col("accounting_book_id"), col("budget_id"), col("cost_account_id"), col("payable_code"), col("vendor_type"), col("vendor_partner_id"), col("vendor_name"), col("source_type"), col("source_id"), col("gross_amount"), col("advance_amount"), col("retention_amount"), col("deduction_amount"), col("paid_amount"), col("remaining_amount"), col("due_date"), col("status"), col("created_by"), col("approved_by")}
}

func receivableColumns() []ColumnSpec {
	return []ColumnSpec{col("project_id"), col("accounting_book_id"), col("revenue_account_id"), col("receivable_code"), col("client_partner_id"), col("client_name"), col("source_type"), col("source_id"), col("contract_amount"), col("milestone_name"), col("gross_amount"), col("received_amount"), col("remaining_amount"), col("due_date"), col("status"), col("created_by"), col("approved_by")}
}

func paymentColumns() []ColumnSpec {
	return []ColumnSpec{col("project_id"), col("accounting_book_id"), col("budget_id"), col("cash_account_id"), col("payment_code"), col("payment_type"), col("target_type"), col("target_id"), col("tax_period_id"), col("amount"), col("payment_method"), col("status"), col("paid_at"), col("created_by"), col("approved_by")}
}

func staffColumns() []ColumnSpec {
	return []ColumnSpec{col("user_id"), col("full_name"), col("email"), col("phone"), col("department"), col("position"), col("status")}
}

func roleColumns() []ColumnSpec {
	return []ColumnSpec{col("name"), col("display_name"), col("description"), col("system_role")}
}
