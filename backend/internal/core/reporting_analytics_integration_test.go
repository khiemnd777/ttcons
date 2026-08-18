package core

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestNormalizeReportOptionsRequest(t *testing.T) {
	selectedID := "30000000-0000-0000-0000-000000000002"
	normalized, cursor, err := normalizeReportOptionsRequest(ReportOptionsRequest{
		Search:     "  Thủ Đức  ",
		SelectedID: selectedID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Search != "Thủ Đức" || normalized.Limit != 20 || normalized.SelectedID != selectedID || cursor != nil {
		t.Fatalf("normalized report options = %#v cursor=%#v", normalized, cursor)
	}

	if _, _, err := normalizeReportOptionsRequest(ReportOptionsRequest{Limit: 51}); err == nil {
		t.Fatal("expected limit validation error")
	}
	if _, _, err := normalizeReportOptionsRequest(ReportOptionsRequest{SelectedID: "not-a-uuid"}); err == nil {
		t.Fatal("expected selected_id validation error")
	}
}

func TestReportProjectOptionsCursorRoundTrip(t *testing.T) {
	want := reportProjectOptionsCursor{
		NameKey: "cải tạo nhà phố thủ đức",
		CodeKey: "ct-td-001",
		ID:      "30000000-0000-0000-0000-000000000002",
	}
	encoded := encodeReportProjectOptionsCursor(want)
	got, err := parseReportProjectOptionsCursor(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("cursor = %#v, want %#v", got, want)
	}
	if _, err := parseReportProjectOptionsCursor("invalid"); err == nil {
		t.Fatal("expected invalid cursor error")
	}
}

func TestNormalizeDashboardPeriodRequest(t *testing.T) {
	now := time.Date(2026, time.August, 18, 9, 30, 0, 0, time.FixedZone("ICT", 7*60*60))

	month, monthMeta, err := normalizeDashboardPeriodRequestAt(DashboardPeriodRequest{}, now)
	if err != nil {
		t.Fatal(err)
	}
	if month.PeriodType != "month" || month.Year != 2026 || month.PeriodNo != 8 || monthMeta.FromDate != "2026-08-01" || monthMeta.ToDate != "2026-08-18" || monthMeta.GroupBy != "day" || !monthMeta.IsCurrent || monthMeta.CanMoveForward {
		t.Fatalf("current month normalization = request %#v meta %#v", month, monthMeta)
	}

	quarter, quarterMeta, err := normalizeDashboardPeriodRequestAt(DashboardPeriodRequest{PeriodType: "quarter", Year: 2026, PeriodNo: 2}, now)
	if err != nil {
		t.Fatal(err)
	}
	if quarter.PeriodNo != 2 || quarterMeta.FromDate != "2026-04-01" || quarterMeta.ToDate != "2026-06-30" || quarterMeta.GroupBy != "month" || quarterMeta.IsCurrent || !quarterMeta.CanMoveForward {
		t.Fatalf("quarter normalization = request %#v meta %#v", quarter, quarterMeta)
	}

	_, leapMeta, err := normalizeDashboardPeriodRequestAt(DashboardPeriodRequest{PeriodType: "month", Year: 2024, PeriodNo: 2}, now)
	if err != nil {
		t.Fatal(err)
	}
	if leapMeta.ToDate != "2024-02-29" {
		t.Fatalf("leap month to = %s, want 2024-02-29", leapMeta.ToDate)
	}

	if _, _, err := normalizeDashboardPeriodRequestAt(DashboardPeriodRequest{PeriodType: "month", Year: 2026, PeriodNo: 9}, now); err == nil {
		t.Fatal("expected future dashboard period validation error")
	}
}

func TestDashboardProjectsCursorBindsFilters(t *testing.T) {
	request := DashboardProjectsRequest{
		DashboardPeriodRequest: DashboardPeriodRequest{PeriodType: "month", Year: 2026, PeriodNo: 8},
		Search:                 "  Nhà phố  ",
		Sort:                   "progress",
		Direction:              "desc",
		Limit:                  25,
	}
	meta := DashboardPeriodMeta{PeriodType: "month", Year: 2026, PeriodNo: 8, FromDate: "2026-08-01", ToDate: "2026-08-18"}
	normalized, offset, filterHash, err := normalizeDashboardProjectsRequest(request, meta, true)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Search != "Nhà phố" || offset != 0 || len(filterHash) != sha256.Size*2 {
		t.Fatalf("normalized dashboard projects = %#v offset=%d hash=%q", normalized, offset, filterHash)
	}
	normalized.Cursor = encodeDashboardProjectsCursor(dashboardProjectsCursor{Version: 1, Offset: 25, FilterHash: filterHash})
	_, offset, _, err = normalizeDashboardProjectsRequest(normalized, meta, true)
	if err != nil || offset != 25 {
		t.Fatalf("dashboard cursor offset=%d err=%v", offset, err)
	}
	normalized.Search = "Khác"
	if _, _, _, err := normalizeDashboardProjectsRequest(normalized, meta, true); err == nil {
		t.Fatal("expected dashboard cursor to fail after filters change")
	}
}

func TestCashflowCSVIncludesLedgerRows(t *testing.T) {
	payload, err := cashflowCSV(Record{"ledger": []Record{{
		"occurred_at":    "2026-06-03T10:00:00Z",
		"payment_code":   "PMT-CSV-001",
		"project_name":   "CSV Project",
		"category":       "Thu khách hàng",
		"direction":      "inflow",
		"payment_type":   "client_receipt",
		"payment_method": "bank_transfer",
		"status":         "approved",
		"inflow_amount":  100,
		"outflow_amount": 0,
		"net_amount":     100,
		"target_type":    "receivable",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if !strings.Contains(text, "payment_code") || !strings.Contains(text, "PMT-CSV-001") {
		t.Fatalf("csv payload missing header or row: %s", text)
	}
}

func TestTaxCSVIncludesLedgerRows(t *testing.T) {
	payload, err := taxCSV(Record{"ledger": []Record{{
		"invoice_date":      "2026-06-05",
		"invoice_number":    "VAT-CSV-001",
		"direction":         "output",
		"project_name":      "CSV Project",
		"counterparty_name": "CSV Client",
		"tax_code":          "0311111111",
		"amount_before_vat": 1000,
		"vat_amount":        100,
		"total_amount":      1100,
		"status":            "issued",
		"source_type":       "receivable",
		"source_code":       "REC-CSV-001",
		"document_name":     "VAT PDF",
		"warning_flags":     "",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if !strings.Contains(text, "invoice_number") || !strings.Contains(text, "VAT-CSV-001") {
		t.Fatalf("csv payload missing header or row: %s", text)
	}
}

func TestProfitCSVIncludesLedgerRows(t *testing.T) {
	payload, err := profitCSV(Record{"ledger": []Record{{
		"invoice_date":          "2026-06-06",
		"invoice_number":        "PNL-CSV-001",
		"direction":             "revenue",
		"cost_type":             "revenue",
		"project_name":          "CSV Project",
		"counterparty_name":     "CSV Client",
		"amount_before_vat":     1000,
		"revenue_amount":        1000,
		"direct_cost_amount":    0,
		"indirect_cost_amount":  0,
		"status":                "issued",
		"source_type":           "receivable",
		"source_code":           "REC-CSV-001",
		"classification_reason": "issued_output_invoice",
		"warning_flags":         "",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if !strings.Contains(text, "invoice_number") || !strings.Contains(text, "PNL-CSV-001") {
		t.Fatalf("csv payload missing header or row: %s", text)
	}
}

func TestOperationalReportCSVIncludesExactlyReturnedRows(t *testing.T) {
	payload, err := operationalReportCSV("projects", []Record{{
		"project_id":        "project-1",
		"name":              "CSV Project",
		"status":            "in_progress",
		"progress_percent":  42,
		"open_issues":       2,
		"pending_approvals": 1,
	}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if !strings.Contains(text, "project_id,name,status") || !strings.Contains(text, "project-1,CSV Project,in_progress,42,2,1") {
		t.Fatalf("operational csv payload missing header or row: %s", text)
	}
}

func TestCashflowReportSummarizesScopedPayments(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{"report.finance.view", "accounting.view"})
	mustExec(t, db, `
		INSERT INTO payments (id, project_id, payment_code, payment_type, target_type, target_id, amount, payment_method, status, paid_at, created_by)
		VALUES
			($1,$2,'PMT-CF-OPEN','client_receipt',NULL,NULL,20,'bank_transfer','approved','2026-05-20T10:00:00Z',$3),
			($4,$2,'PMT-CF-IN','client_receipt',NULL,NULL,100,'bank_transfer','approved','2026-06-03T10:00:00Z',$3),
			($5,$2,'PMT-CF-OUT','supplier_payment',NULL,NULL,40,'bank_transfer','approved','2026-06-03T11:00:00Z',$3),
			($6,$2,'PMT-CF-OTHER','other',NULL,NULL,10,'cash','approved','2026-06-04T09:00:00Z',$3),
			($7,$2,'PMT-CF-PENDING','client_receipt',NULL,NULL,30,'bank_transfer','pending','2026-06-05T09:00:00Z',$3),
			($8,$9,'PMT-CF-OTHER-PROJECT','client_receipt',NULL,NULL,999,'bank_transfer','approved','2026-06-03T10:00:00Z',$3)
	`, newID(), fixture.projectID, fixture.userID, newID(), newID(), newID(), newID(), newID(), fixture.otherProjectID)

	report, err := service.CashflowReport(ctx, CashflowReportRequest{
		ProjectID: fixture.projectID,
		FromDate:  "2026-06-01",
		ToDate:    "2026-06-30",
		GroupBy:   "day",
		Status:    "approved",
	}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	summary, _ := report["summary"].(Record)
	series, _ := report["series"].([]Record)
	categories, _ := report["categories"].([]Record)
	projects, _ := report["projects"].([]Record)
	ledger, _ := report["ledger"].([]Record)
	if number(summary["opening_balance"]) != 20 {
		t.Fatalf("opening_balance = %v, want 20", summary["opening_balance"])
	}
	if number(summary["total_inflow"]) != 100 || number(summary["total_outflow"]) != 40 || number(summary["net_flow"]) != 60 {
		t.Fatalf("cashflow summary = inflow %v outflow %v net %v, want 100/40/60", summary["total_inflow"], summary["total_outflow"], summary["net_flow"])
	}
	if number(summary["closing_balance"]) != 80 {
		t.Fatalf("closing_balance = %v, want 80", summary["closing_balance"])
	}
	if number(summary["payment_count"]) != 3 || number(summary["unclassified_count"]) != 1 {
		t.Fatalf("counts = payment %v unclassified %v, want 3/1", summary["payment_count"], summary["unclassified_count"])
	}
	if number(summary["pending_payment_count"]) != 1 || number(summary["pending_payment_total"]) != 30 {
		t.Fatalf("pending = count %v total %v, want 1/30", summary["pending_payment_count"], summary["pending_payment_total"])
	}
	if len(series) < 1 {
		t.Fatal("expected at least one cashflow series row")
	}
	if len(categories) < 3 {
		t.Fatalf("categories = %#v, want at least inflow/outflow/unclassified", categories)
	}
	if len(projects) != 1 || cleanID(projects[0]["project_id"]) != fixture.projectID || number(projects[0]["net_amount"]) != 60 {
		t.Fatalf("cashflow project breakdown = %#v, want scoped project net 60", projects)
	}
	if len(ledger) != 3 {
		t.Fatalf("ledger rows = %d, want 3 approved scoped rows", len(ledger))
	}
}

func TestDashboardRedactsFinanceWithoutFinancePermission(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{"report.view"})

	dashboard, err := service.Dashboard(ctx, DashboardPeriodRequest{PeriodType: "month", Year: 2026, PeriodNo: 6}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if dashboard.Capabilities.FinanceVisible || dashboard.Cashflow != nil {
		t.Fatalf("dashboard leaked finance capability/data: %#v", dashboard)
	}
	if dashboard.ProjectSnapshot.TotalProjects != 1 || dashboard.Meta.FromDate != "2026-06-01" || dashboard.Meta.ToDate != "2026-06-30" {
		t.Fatalf("dashboard snapshot/meta = %#v", dashboard)
	}

	page, err := service.DashboardProjects(ctx, DashboardProjectsRequest{
		DashboardPeriodRequest: DashboardPeriodRequest{PeriodType: "month", Year: 2026, PeriodNo: 6},
	}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].FinanceVisible || page.Items[0].InflowAmount != nil || page.Items[0].OutflowAmount != nil || page.Items[0].NetAmount != nil {
		t.Fatalf("dashboard project row leaked finance data: %#v", page.Items)
	}
}

func TestDashboardProjectsUsesScopedPaymentLedgerAndCursor(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{"report.view", "report.finance.view"})
	extraProjectID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM project_members WHERE project_id=$1`, extraProjectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, extraProjectID)
	})
	mustExec(t, db, `UPDATE projects SET name='Dashboard Alpha', progress_percent=42 WHERE id=$1`, fixture.projectID)
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status,progress_percent) VALUES ($1,'Dashboard Beta',$2,'Client','in_progress',75)`, extraProjectID, "DBB-"+extraProjectID[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id,staff_id,role_ids,active) VALUES ($1,$2,'["Member"]'::jsonb,true)`, extraProjectID, fixture.staffID)
	mustExec(t, db, `
		INSERT INTO payments (id,project_id,payment_code,payment_type,amount,payment_method,status,paid_at,created_by)
		VALUES ($1,$2,'PMT-DASH-IN','client_receipt',120,'bank_transfer','approved','2026-06-03T10:00:00Z',$3),
		       ($4,$2,'PMT-DASH-OUT','outbound',35,'bank_transfer','approved','2026-06-04T10:00:00Z',$3),
		       ($5,$6,'PMT-DASH-OTHER','client_receipt',999,'bank_transfer','approved','2026-06-03T10:00:00Z',$3)
	`, newID(), fixture.projectID, fixture.userID, newID(), newID(), fixture.otherProjectID)

	request := DashboardProjectsRequest{
		DashboardPeriodRequest: DashboardPeriodRequest{PeriodType: "month", Year: 2026, PeriodNo: 6},
		Search:                 "Dashboard",
		Sort:                   "name",
		Direction:              "asc",
		Limit:                  1,
	}
	first, err := service.DashboardProjects(ctx, request, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 1 || first.Items[0].ID != fixture.projectID || !first.Items[0].FinanceVisible || first.Items[0].NetAmount == nil || *first.Items[0].NetAmount != 85 || !first.HasMore || first.NextCursor == "" || first.Total != 2 {
		t.Fatalf("dashboard projects first page = %#v", first)
	}
	request.Cursor = first.NextCursor
	second, err := service.DashboardProjects(ctx, request, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || second.Items[0].ID != extraProjectID || second.HasMore || second.Total != 2 {
		t.Fatalf("dashboard projects second page = %#v", second)
	}
}

func TestTaxReportSummarizesScopedInvoices(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{"report.finance.view", "accounting.view"})
	payableID := newID()
	draftPayableID := newID()
	otherPayableID := newID()
	receivableID := newID()
	otherReceivableID := newID()
	inputInvoiceID := newID()
	outputInvoiceID := newID()
	mustExec(t, db, `
		INSERT INTO payables (id, project_id, payable_code, vendor_type, vendor_name, source_type, gross_amount, remaining_amount, status, created_by)
		VALUES
			($1,$2,$3,'supplier','Tax Vendor','manual',1100,1100,'approved',$4),
			($5,$2,$6,'supplier','Draft Vendor','manual',550,550,'approved',$4),
			($7,$8,$9,'supplier','Other Vendor','manual',999,999,'approved',$4)
	`, payableID, fixture.projectID, "PAY-TAX-"+payableID[:8], fixture.userID, draftPayableID, "PAY-TAX-"+draftPayableID[:8], otherPayableID, fixture.otherProjectID, "PAY-TAX-"+otherPayableID[:8])
	mustExec(t, db, `
		INSERT INTO receivables (id, project_id, receivable_code, client_name, source_type, gross_amount, received_amount, remaining_amount, status, created_by)
		VALUES
			($1,$2,$3,'Tax Client','manual',2200,0,2200,'approved',$4),
			($5,$6,$7,'Other Client','manual',999,0,999,'approved',$4)
	`, receivableID, fixture.projectID, "REC-TAX-"+receivableID[:8], fixture.userID, otherReceivableID, fixture.otherProjectID, "REC-TAX-"+otherReceivableID[:8])
	mustExec(t, db, `
		INSERT INTO input_invoices (
			id, project_id, payable_id, invoice_number, invoice_date, vendor_name, vendor_tax_code,
			amount_before_vat, vat_amount, total_amount, status,
			storage_provider, storage_key, mime_type, file_name, file_size, checksum_sha256,
			uploaded_by, verified_by, verified_at
		)
		VALUES
			($1,$2,$3,'IN-TAX-001','2026-06-03','Tax Vendor','0101111111',
				1000,100,1100,'draft',
				'local','input-invoices/tax-1.pdf','application/pdf','tax-1.pdf',128,$4,
				$5,NULL,NULL),
			($6,$2,$7,'IN-TAX-DRAFT','2026-06-05','Draft Vendor','0102222222',
				500,50,550,'draft',
				'local','input-invoices/tax-draft.pdf','application/pdf','tax-draft.pdf',128,$4,
				$5,NULL,NULL),
			($8,$9,$10,'IN-TAX-OTHER','2026-06-03','Other Vendor','0109999999',
				900,90,990,'verified',
				'local','input-invoices/tax-other.pdf','application/pdf','tax-other.pdf',128,$4,
				$5,$5,now())
	`, inputInvoiceID, fixture.projectID, payableID, strings.Repeat("c", 64), fixture.userID, newID(), draftPayableID, newID(), fixture.otherProjectID, otherPayableID)
	mustExec(t, db, `
		INSERT INTO output_invoices (
			id, project_id, receivable_id, invoice_number, invoice_date, client_name, client_tax_code,
			amount_before_vat, vat_amount, total_amount, status,
			storage_provider, storage_key, mime_type, file_name, file_size, checksum_sha256,
			uploaded_by, issued_by, issued_at
		)
		VALUES
			($1,$2,$3,'OUT-TAX-001','2026-06-04','Tax Client','',
				2000,200,2200,'draft',
				'local','output-invoices/tax-1.pdf','application/pdf','tax-1.pdf',128,$4,
				$5,NULL,NULL),
			($6,$7,$8,'OUT-TAX-OTHER','2026-06-04','Other Client','0319999999',
				900,90,990,'issued',
				'local','output-invoices/tax-other.pdf','application/pdf','tax-other.pdf',128,$4,
				$5,$5,now())
	`, outputInvoiceID, fixture.projectID, receivableID, strings.Repeat("d", 64), fixture.userID, newID(), fixture.otherProjectID, otherReceivableID)
	if _, err := repo.InputInvoiceAction(ctx, inputInvoiceID, "verify", ActionRequest{}, fixture.userID, "Kế toán báo cáo"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.OutputInvoiceAction(ctx, outputInvoiceID, "issue", ActionRequest{}, fixture.userID, "Kế toán báo cáo"); err != nil {
		t.Fatal(err)
	}

	report, err := service.TaxReport(ctx, TaxReportRequest{
		ProjectID: fixture.projectID,
		FromDate:  "2026-06-01",
		ToDate:    "2026-06-30",
		GroupBy:   "month",
		Source:    "all",
		Status:    "posted",
	}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	summary, _ := report["summary"].(Record)
	periods, _ := report["periods"].([]Record)
	counterparties, _ := report["counterparties"].([]Record)
	ledger, _ := report["ledger"].([]Record)
	warnings, _ := report["warnings"].([]Record)
	if number(summary["input_vat_amount"]) != 100 || number(summary["output_vat_amount"]) != 200 || number(summary["net_vat_amount"]) != 100 {
		t.Fatalf("tax summary = input %v output %v net %v, want 100/200/100", summary["input_vat_amount"], summary["output_vat_amount"], summary["net_vat_amount"])
	}
	if number(summary["input_invoice_count"]) != 1 || number(summary["output_invoice_count"]) != 1 {
		t.Fatalf("invoice counts = input %v output %v, want 1/1", summary["input_invoice_count"], summary["output_invoice_count"])
	}
	if summary["net_vat_status"] != "payable" {
		t.Fatalf("net_vat_status = %v, want payable", summary["net_vat_status"])
	}
	if number(summary["excluded_unposted_count"]) != 1 {
		t.Fatalf("excluded_unposted_count = %v, want 1", summary["excluded_unposted_count"])
	}
	if number(summary["missing_tax_code_count"]) != 1 || number(summary["vat_math_mismatch_count"]) != 0 {
		t.Fatalf("warning counts = missing %v mismatch %v, want 1/0", summary["missing_tax_code_count"], summary["vat_math_mismatch_count"])
	}
	if len(warnings) != 2 || number(summary["warning_count"]) != 2 {
		t.Fatalf("warnings = %d summary=%v, want 2", len(warnings), summary["warning_count"])
	}
	if len(periods) != 1 || number(periods[0]["net_vat_amount"]) != 100 {
		t.Fatalf("periods = %#v, want one net 100 row", periods)
	}
	if len(counterparties) != 2 {
		t.Fatalf("counterparties = %d, want 2 scoped posted counterparties", len(counterparties))
	}
	if len(ledger) != 2 {
		t.Fatalf("ledger rows = %d, want 2 scoped posted invoices", len(ledger))
	}
}

func TestProfitReportSummarizesScopedInvoices(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{"report.finance.view", "accounting.view"})
	directPayableID := newID()
	indirectPayableID := newID()
	draftPayableID := newID()
	otherPayableID := newID()
	receivableID := newID()
	draftReceivableID := newID()
	otherReceivableID := newID()
	mustExec(t, db, `
		INSERT INTO payables (id, project_id, payable_code, vendor_type, vendor_name, source_type, gross_amount, remaining_amount, status, created_by)
		VALUES
			($1,$2,$3,'supplier','Direct Vendor','procurement',330,330,'approved',$4),
			($5,$2,$6,'consultant','Indirect Vendor','manual',88,88,'approved',$4),
			($7,$2,$8,'supplier','Draft Vendor','procurement',55,55,'approved',$4),
			($9,$10,$11,'supplier','Other Vendor','procurement',999,999,'approved',$4)
	`, directPayableID, fixture.projectID, "PAY-PNL-"+directPayableID[:8], fixture.userID, indirectPayableID, "PAY-PNL-"+indirectPayableID[:8], draftPayableID, "PAY-PNL-"+draftPayableID[:8], otherPayableID, fixture.otherProjectID, "PAY-PNL-"+otherPayableID[:8])
	mustExec(t, db, `
		INSERT INTO receivables (id, project_id, receivable_code, client_name, source_type, gross_amount, received_amount, remaining_amount, status, created_by)
		VALUES
			($1,$2,$3,'P&L Client','manual',1100,0,1100,'approved',$4),
			($5,$2,$6,'Draft Client','manual',220,0,220,'approved',$4),
			($7,$8,$9,'Other Client','manual',999,0,999,'approved',$4)
	`, receivableID, fixture.projectID, "REC-PNL-"+receivableID[:8], fixture.userID, draftReceivableID, "REC-PNL-"+draftReceivableID[:8], otherReceivableID, fixture.otherProjectID, "REC-PNL-"+otherReceivableID[:8])
	mustExec(t, db, `
		INSERT INTO input_invoices (
			id, project_id, payable_id, invoice_number, invoice_date, vendor_name, vendor_tax_code,
			amount_before_vat, vat_amount, total_amount, status,
			storage_provider, storage_key, mime_type, file_name, file_size, checksum_sha256,
			uploaded_by, verified_by, verified_at
		)
		VALUES
			($1,$2,$3,'IN-PNL-DIRECT','2026-06-03','Direct Vendor','0101111111',
				300,30,330,'verified',
				'local','input-invoices/pnl-direct.pdf','application/pdf','pnl-direct.pdf',128,$4,
				$5,$5,now()),
			($6,$2,$7,'IN-PNL-INDIRECT','2026-06-04','Indirect Vendor','0102222222',
				80,8,88,'verified',
				'local','input-invoices/pnl-indirect.pdf','application/pdf','pnl-indirect.pdf',128,$4,
				$5,$5,now()),
			($8,$2,$9,'IN-PNL-DRAFT','2026-06-05','Draft Vendor','0103333333',
				50,5,55,'draft',
				'local','input-invoices/pnl-draft.pdf','application/pdf','pnl-draft.pdf',128,$4,
				$5,NULL,NULL),
			($10,$11,$12,'IN-PNL-OTHER','2026-06-04','Other Vendor','0109999999',
				900,90,990,'verified',
				'local','input-invoices/pnl-other.pdf','application/pdf','pnl-other.pdf',128,$4,
				$5,$5,now())
	`, newID(), fixture.projectID, directPayableID, strings.Repeat("e", 64), fixture.userID, newID(), indirectPayableID, newID(), draftPayableID, newID(), fixture.otherProjectID, otherPayableID)
	mustExec(t, db, `
		INSERT INTO output_invoices (
			id, project_id, receivable_id, invoice_number, invoice_date, client_name, client_tax_code,
			amount_before_vat, vat_amount, total_amount, status,
			storage_provider, storage_key, mime_type, file_name, file_size, checksum_sha256,
			uploaded_by, issued_by, issued_at
		)
		VALUES
			($1,$2,$3,'OUT-PNL-001','2026-06-02','P&L Client','0311111111',
				1000,100,1100,'issued',
				'local','output-invoices/pnl-1.pdf','application/pdf','pnl-1.pdf',128,$4,
				$5,$5,now()),
			($6,$2,$7,'OUT-PNL-DRAFT','2026-06-05','Draft Client','0312222222',
				200,20,220,'draft',
				'local','output-invoices/pnl-draft.pdf','application/pdf','pnl-draft.pdf',128,$4,
				$5,NULL,NULL),
			($8,$9,$10,'OUT-PNL-OTHER','2026-06-02','Other Client','0319999999',
				900,90,990,'issued',
				'local','output-invoices/pnl-other.pdf','application/pdf','pnl-other.pdf',128,$4,
				$5,$5,now())
	`, newID(), fixture.projectID, receivableID, strings.Repeat("f", 64), fixture.userID, newID(), draftReceivableID, newID(), fixture.otherProjectID, otherReceivableID)

	report, err := service.ProfitReport(ctx, ProfitReportRequest{
		ProjectID: fixture.projectID,
		FromDate:  "2026-06-01",
		ToDate:    "2026-06-30",
		GroupBy:   "month",
		Status:    "posted",
	}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	summary, _ := report["summary"].(Record)
	periods, _ := report["periods"].([]Record)
	projects, _ := report["projects"].([]Record)
	breakdown, _ := report["cost_breakdown"].([]Record)
	ledger, _ := report["ledger"].([]Record)
	warnings, _ := report["warnings"].([]Record)
	if number(summary["revenue_amount"]) != 1000 || number(summary["direct_cost_amount"]) != 300 || number(summary["indirect_cost_amount"]) != 80 {
		t.Fatalf("pnl summary = revenue %v direct %v indirect %v, want 1000/300/80", summary["revenue_amount"], summary["direct_cost_amount"], summary["indirect_cost_amount"])
	}
	if number(summary["gross_profit"]) != 700 || number(summary["net_profit"]) != 620 {
		t.Fatalf("profit = gross %v net %v, want 700/620", summary["gross_profit"], summary["net_profit"])
	}
	if number(summary["gross_margin_percent"]) != 70 || number(summary["net_margin_percent"]) != 62 {
		t.Fatalf("margins = gross %v net %v, want 70/62", summary["gross_margin_percent"], summary["net_margin_percent"])
	}
	if number(summary["excluded_unposted_count"]) != 2 || number(summary["excluded_unposted_total"]) != 250 {
		t.Fatalf("excluded = count %v total %v, want 2/250", summary["excluded_unposted_count"], summary["excluded_unposted_total"])
	}
	if number(summary["unclassified_cost_count"]) != 1 {
		t.Fatalf("unclassified_cost_count = %v, want 1", summary["unclassified_cost_count"])
	}
	if len(warnings) != 2 || number(summary["warning_count"]) != 2 {
		t.Fatalf("warnings = %d summary=%v, want 2", len(warnings), summary["warning_count"])
	}
	if len(periods) != 1 || number(periods[0]["net_profit"]) != 620 {
		t.Fatalf("periods = %#v, want one net 620 row", periods)
	}
	if len(projects) != 1 || number(projects[0]["net_profit"]) != 620 {
		t.Fatalf("projects = %#v, want one scoped project net 620", projects)
	}
	if len(breakdown) != 2 {
		t.Fatalf("cost breakdown rows = %d, want 2 direct/indirect groups", len(breakdown))
	}
	if len(ledger) != 3 {
		t.Fatalf("ledger rows = %d, want 3 scoped posted P&L rows", len(ledger))
	}
}

func TestOperationalReportsAggregateWithoutJoinMultiplicationAndRespectScope(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{"report.view", "report.finance.view", "accounting.view"})
	suffix := newID()
	materialID := newID()
	warehouseID := newID()
	budgetOneID := newID()
	budgetTwoID := newID()
	outsideStaffID := newID()
	outsideUserID := newID()

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM stock_movements WHERE material_id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM site_material_balances WHERE material_id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM materials WHERE id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM warehouses WHERE id=$1`, warehouseID)
		_, _ = db.ExecContext(ctx, `DELETE FROM project_members WHERE staff_id=$1`, outsideStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, outsideStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, outsideUserID)
	})

	mustExec(t, db, `
		INSERT INTO issues (id, project_id, title, status, reported_by)
		VALUES ($1,$2,'Issue one','open',$3),($4,$2,'Issue two','in_progress',$3)
	`, newID(), fixture.projectID, fixture.userID, newID())
	mustExec(t, db, `
		INSERT INTO approval_requests (id, target_type, project_id, requested_by, status, reason)
		VALUES ($1,'work_item',$2,$3,'pending','Approval one'),($4,'work_item',$2,$3,'pending','Approval two')
	`, newID(), fixture.projectID, fixture.userID, newID())

	projectRows, err := service.Report(ctx, "projects", OperationalReportRequest{}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	projectRow := recordByID(projectRows, "project_id", fixture.projectID)
	if projectRow == nil || number(projectRow["open_issues"]) != 2 || number(projectRow["pending_approvals"]) != 2 {
		t.Fatalf("project report row = %#v, want two issues and two approvals", projectRow)
	}

	mustExec(t, db, `
		INSERT INTO payables (id, project_id, payable_code, vendor_type, vendor_name, gross_amount, remaining_amount, status, created_by)
		VALUES ($1,$2,$3,'supplier','Vendor one',100,100,'approved',$4),($5,$2,$6,'supplier','Vendor two',100,100,'approved',$4)
	`, newID(), fixture.projectID, "PAY-RPT-1-"+suffix[:8], fixture.userID, newID(), "PAY-RPT-2-"+suffix[:8])
	mustExec(t, db, `
		INSERT INTO receivables (id, project_id, receivable_code, client_name, gross_amount, remaining_amount, status, created_by)
		VALUES ($1,$2,$3,'Client one',50,50,'approved',$4),($5,$2,$6,'Client two',50,50,'approved',$4)
	`, newID(), fixture.projectID, "REC-RPT-1-"+suffix[:8], fixture.userID, newID(), "REC-RPT-2-"+suffix[:8])
	mustExec(t, db, `
		INSERT INTO accounting_budgets (id, project_id, budget_code, name, used_amount, status)
		VALUES ($1,$2,$3,'Budget one',25,'approved'),($4,$2,$5,'Budget two',25,'approved')
	`, budgetOneID, fixture.projectID, "BUD-RPT-1-"+suffix[:8], budgetTwoID, "BUD-RPT-2-"+suffix[:8])
	mustExec(t, db, `
		INSERT INTO budget_movements (budget_id,project_id,movement_type,actual_delta,source_type,source_id,source_event,description,created_by)
		VALUES
			($1,$3,'actual',25,'test',$1,'report_actual','Chi phí thực tế kiểm thử báo cáo',$4),
			($2,$3,'actual',25,'test',$2,'report_actual','Chi phí thực tế kiểm thử báo cáo',$4)
	`, budgetOneID, budgetTwoID, fixture.projectID, fixture.userID)

	accountingRows, err := service.Report(ctx, "accounting", OperationalReportRequest{ProjectID: fixture.projectID}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(accountingRows) != 1 || number(accountingRows[0]["payable_remaining"]) != 200 || number(accountingRows[0]["receivable_remaining"]) != 100 || number(accountingRows[0]["used_budget"]) != 50 {
		t.Fatalf("accounting report = %#v, want payable/receivable/budget 200/100/50", accountingRows)
	}

	mustExec(t, db, `INSERT INTO materials (id, code, name, unit, category, active) VALUES ($1,$2,'Report Material','m','Test',true)`, materialID, "MAT-RPT-"+suffix[:8])
	mustExec(t, db, `INSERT INTO warehouses (id, name, location, type, active) VALUES ($1,'Report Warehouse','Test','central',true)`, warehouseID)
	mustExec(t, db, `
		INSERT INTO stock_movements (id, movement_code, project_id, warehouse_id, material_id, movement_type, quantity, unit, created_by)
		VALUES ($1,$2,$3,$4,$5,'receive',10,'m',$6),($7,$8,$3,$4,$5,'receive',20,'m',$6)
	`, newID(), "SM-RPT-1-"+suffix[:8], fixture.projectID, warehouseID, materialID, fixture.userID, newID(), "SM-RPT-2-"+suffix[:8])
	mustExec(t, db, `
		INSERT INTO site_material_balances (id, project_id, material_id, surplus_quantity)
		VALUES ($1,$2,$3,7)
	`, newID(), fixture.projectID, materialID)

	warehouseRows, err := service.Report(ctx, "warehouse", OperationalReportRequest{ProjectID: fixture.projectID}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	warehouseRow := recordByID(warehouseRows, "material_id", materialID)
	if warehouseRow == nil || number(warehouseRow["stock_quantity"]) != 30 || number(warehouseRow["surplus_quantity"]) != 7 {
		t.Fatalf("warehouse report row = %#v, want stock/surplus 30/7", warehouseRow)
	}

	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Outside Staff','active')`, outsideUserID, "outside-report-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id, user_id, full_name, email, department, position, status) VALUES ($1,$2,'Outside Staff',$3,'Other','Worker','active')`, outsideStaffID, outsideUserID, "outside-report-staff-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO project_members (project_id, staff_id, role_ids, active) VALUES ($1,$2,'["Worker"]'::jsonb,true)`, fixture.otherProjectID, outsideStaffID)

	staffRows, err := service.Report(ctx, "staff", OperationalReportRequest{}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if recordByID(staffRows, "staff_id", outsideStaffID) != nil || recordByID(staffRows, "staff_id", fixture.staffID) == nil {
		t.Fatalf("staff report leaked out-of-scope staff or omitted scoped staff: %#v", staffRows)
	}

	extraScopedProjectID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM project_members WHERE project_id=$1`, extraScopedProjectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, extraScopedProjectID)
	})
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'Batch4 Scoped Z',$2,'Client','planning')`, extraScopedProjectID, "B4SZ-"+extraScopedProjectID[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id, staff_id, role_ids, active) VALUES ($1,$2,'["Member"]'::jsonb,true)`, extraScopedProjectID, fixture.staffID)

	options, err := service.ReportOptions(ctx, ReportOptionsRequest{
		Search:     "batch4 scoped",
		Limit:      1,
		SelectedID: fixture.projectID,
	}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	projects := options.Projects.Items
	if len(projects) != 1 || projects[0].ID != fixture.projectID || projects[0].Code == "" || !options.Projects.HasMore || options.Projects.NextCursor == "" {
		t.Fatalf("report options scope/search = %#v", projects)
	}
	if options.Projects.Selected == nil || options.Projects.Selected.ID != fixture.projectID {
		t.Fatalf("report options selected = %#v", options.Projects.Selected)
	}
	nextOptions, err := service.ReportOptions(ctx, ReportOptionsRequest{
		Search: "batch4 scoped",
		Limit:  1,
		Cursor: options.Projects.NextCursor,
	}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(nextOptions.Projects.Items) != 1 || nextOptions.Projects.Items[0].ID != extraScopedProjectID || nextOptions.Projects.HasMore {
		t.Fatalf("report options next page = %#v", nextOptions.Projects)
	}

	outsideOptions, err := service.ReportOptions(ctx, ReportOptionsRequest{Search: "batch4 other"}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(outsideOptions.Projects.Items) != 0 {
		t.Fatalf("report options leaked out-of-scope project: %#v", outsideOptions.Projects.Items)
	}
}

func TestBatch4StockAggregateExcludesOutOfScopeProjectMovements(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	repo := NewRepository(db)
	handler := NewHandler(NewService(repo, "test-secret"))
	fixture := setupProjectScopeFixture(t, db, []string{"warehouse.view", "report.view"})
	materialID := newID()
	warehouseID := newID()
	mustExec(t, db, `INSERT INTO materials (id, code, name, unit, category, active) VALUES ($1,$2,'Batch4 Material','m','Test',true)`, materialID, "MAT-"+materialID[:8])
	mustExec(t, db, `INSERT INTO warehouses (id, name, location, type, active) VALUES ($1,'Batch4 Warehouse','Test','central',true)`, warehouseID)
	mustExec(t, db, `
		INSERT INTO stock_movements (movement_code, project_id, warehouse_id, material_id, movement_type, quantity, unit, reason, created_by, approved_by)
		VALUES ($1,NULL,$2,$3,'receive',100,'m','Central stock',$4,$4),
		       ($5,$6,$2,$3,'issue',10,'m','Scoped issue',$4,$4),
		       ($7,$8,$2,$3,'issue',70,'m','Out of scope issue',$4,$4)
	`, "SM-C-"+materialID[:6], warehouseID, materialID, fixture.userID, "SM-S-"+materialID[:6], fixture.projectID, "SM-O-"+materialID[:6], fixture.otherProjectID)
	app := batch4TestApp(fixture.userID, func(app *fiber.App, h *Handler) {
		app.Get("/stock", h.Stock)
	}, handler)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/stock", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var body struct {
		Success bool     `json:"success"`
		Data    []Record `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	var found Record
	for _, row := range body.Data {
		if row["material_id"] == materialID {
			found = row
			break
		}
	}
	if found == nil {
		t.Fatal("expected batch4 material in stock response")
	}
	if got := number(found["stock_quantity"]); got != 90 {
		t.Fatalf("stock_quantity = %v, want 90", got)
	}
}
