package core

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
)

func (h *Handler) AccountingAccounts(c *fiber.Ctx) error {
	projectID := cleanID(c.Query("project_id"))
	if err := h.requireProjectPermission(c, projectID, []string{"accounting.journal_view"}); err != nil {
		return err
	}
	rows, err := h.service.AccountingAccounts(c.Context(), projectID)
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) AccountingPeriods(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"accounting.journal_view"}); err != nil {
		return err
	}
	rows, err := h.service.AccountingPeriods(c.Context())
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) AccountingTaxCodes(c *fiber.Ctx) error {
	projectID := cleanID(c.Query("project_id"))
	if err := h.requireProjectPermission(c, projectID, []string{"accounting.tax_view"}); err != nil {
		return err
	}
	rows, err := h.service.AccountingTaxCodes(c.Context(), projectID)
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) AccountingTaxPeriods(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"accounting.tax_view"}); err != nil {
		return err
	}
	rows, err := h.service.AccountingTaxPeriods(c.Context())
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) AccountingTaxPeriod(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"accounting.tax_view"}); err != nil {
		return err
	}
	row, err := h.service.AccountingTaxPeriod(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) PrepareAccountingTaxPeriod(c *fiber.Ctx) error {
	return h.accountingTaxPeriodAction(c, "prepare")
}

func (h *Handler) FileAccountingTaxPeriod(c *fiber.Ctx) error {
	return h.accountingTaxPeriodAction(c, "file")
}

func (h *Handler) LockAccountingTaxPeriod(c *fiber.Ctx) error {
	return h.accountingTaxPeriodAction(c, "lock")
}

func (h *Handler) StartAccountingTaxPeriodAmendment(c *fiber.Ctx) error {
	return h.accountingTaxPeriodAction(c, "start_amendment")
}

func (h *Handler) accountingTaxPeriodAction(c *fiber.Ctx, action string) error {
	if err := h.requirePermission(c, []string{"accounting.tax_manage"}); err != nil {
		return err
	}
	var req AccountingTaxPeriodActionRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return validationError("Dữ liệu xử lý kỳ thuế GTGT không hợp lệ")
	}
	row, err := h.service.AccountingTaxPeriodAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) CreateAccountingTaxAdjustment(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"accounting.tax_manage"}); err != nil {
		return err
	}
	var req AccountingTaxAdjustmentCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return validationError("Dữ liệu điều chỉnh thuế GTGT không hợp lệ")
	}
	row, err := h.service.CreateAccountingTaxAdjustment(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ReverseAccountingTaxAdjustment(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"accounting.tax_manage"}); err != nil {
		return err
	}
	var req AccountingTaxAdjustmentReverseRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return validationError("Dữ liệu đảo điều chỉnh thuế GTGT không hợp lệ")
	}
	row, err := h.service.ReverseAccountingTaxAdjustment(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) CreateAccountingTaxPayment(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"accounting.tax_manage"}); err != nil {
		return err
	}
	var req AccountingTaxPaymentCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return validationError("Dữ liệu Phiếu nộp thuế GTGT không hợp lệ")
	}
	row, err := h.service.CreateAccountingTaxPayment(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) AccountingTaxPeriodExport(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"accounting.tax_view"}); err != nil {
		return err
	}
	kind := strings.TrimSpace(c.Query("kind"))
	if !statusIn(kind, "purchases", "sales", "declaration") {
		return validationError("Loại sổ thuế xuất ra không hợp lệ", Record{"kind": kind})
	}
	period, err := h.service.AccountingTaxPeriod(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	payload, err := accountingTaxPeriodCSV(period, kind)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="vat-%s-%v-%v.csv"`, kind, period["fiscal_year"], period["period_no"]))
	return c.Send(payload)
}

func accountingTaxPeriodCSV(period Record, kind string) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if kind == "declaration" {
		if err := writer.Write([]string{"Chỉ tiêu", "Giá trị"}); err != nil {
			return nil, err
		}
		latest, _ := period["latest_return"].(Record)
		if latest == nil {
			latest, _ = period["summary"].(Record)
		}
		for _, item := range []struct{ label, key string }{
			{"Thuế GTGT đầu vào", "input_vat_amount"}, {"Thuế GTGT đầu vào được khấu trừ", "deductible_input_vat_amount"},
			{"Thuế GTGT đầu ra", "output_vat_amount"}, {"Điều chỉnh thuần", "adjustment_vat_amount"},
			{"Thuế còn được khấu trừ kỳ trước", "credit_brought_forward"}, {"Thuế GTGT phải nộp", "vat_payable"},
			{"Thuế còn được khấu trừ chuyển kỳ sau", "credit_carried_forward"},
		} {
			if err := writer.Write([]string{item.label, fmt.Sprint(latest[item.key])}); err != nil {
				return nil, err
			}
		}
	} else {
		if err := writer.Write([]string{"Ngày hóa đơn", "Số hóa đơn", "Công trình", "Đối tượng", "Mã số thuế đối tượng", "Mã thuế GTGT", "Giá trị chưa thuế", "Tiền thuế", "Được khấu trừ", "Số CTGS"}); err != nil {
			return nil, err
		}
		rows, _ := period[kind].([]Record)
		for _, row := range rows {
			if err := writer.Write([]string{
				fmt.Sprint(row["invoice_date"]), fmt.Sprint(row["invoice_number"]), fmt.Sprint(row["project_name"]),
				fmt.Sprint(row["counterparty_name"]), fmt.Sprint(row["counterparty_tax_code"]), fmt.Sprint(row["tax_code"]),
				fmt.Sprint(row["taxable_amount"]), fmt.Sprint(row["vat_amount"]), fmt.Sprint(row["deductible_amount"]), fmt.Sprint(row["journal_entry_no"]),
			}); err != nil {
				return nil, err
			}
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (h *Handler) AccountingJournalEntries(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"accounting.journal_view"}); err != nil {
		return err
	}
	filter := AccountingJournalFilter{
		ProjectID: c.Query("project_id"),
		DateFrom:  c.Query("date_from"),
		DateTo:    c.Query("date_to"),
		AccountID: c.Query("account_id"),
	}
	if filter.ProjectID != "" {
		if err := h.requireProjectPermission(c, cleanID(filter.ProjectID), []string{"accounting.journal_view"}); err != nil {
			return err
		}
	}
	rows, err := h.service.AccountingJournalEntries(c.Context(), actorID(c), filter)
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) AccountingJournalEntry(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "journal_entries", c.Params("id"), []string{"accounting.journal_view"}); err != nil {
		return err
	}
	row, err := h.service.AccountingJournalEntry(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) CreateAccountingJournal(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"accounting.journal_post"}); err != nil {
		return err
	}
	var req AccountingJournalCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return validationError("Dữ liệu chứng từ ghi sổ không hợp lệ")
	}
	row, err := h.service.CreateAccountingJournal(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ReverseAccountingJournal(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "journal_entries", c.Params("id"), []string{"accounting.journal_reverse"}); err != nil {
		return err
	}
	var req AccountingJournalReverseRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return validationError("Dữ liệu bút toán đảo không hợp lệ")
	}
	row, err := h.service.ReverseAccountingJournal(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) CloseAccountingPeriod(c *fiber.Ctx) error {
	return h.accountingPeriodAction(c, "close", "accounting.period_close")
}

func (h *Handler) ReopenAccountingPeriod(c *fiber.Ctx) error {
	return h.accountingPeriodAction(c, "reopen", "accounting.period_reopen")
}

func (h *Handler) accountingPeriodAction(c *fiber.Ctx, action, permission string) error {
	if err := h.requirePermission(c, []string{permission}); err != nil {
		return err
	}
	var req AccountingPeriodActionRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return validationError("Dữ liệu kỳ kế toán không hợp lệ")
	}
	row, err := h.service.AccountingPeriodAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) AccountingTrialBalance(c *fiber.Ctx) error {
	return h.accountingReport(c, "trial_balance")
}

func (h *Handler) AccountingGeneralLedger(c *fiber.Ctx) error {
	return h.accountingReport(c, "general_ledger")
}

func (h *Handler) AccountingProfitLoss(c *fiber.Ctx) error {
	return h.accountingReport(c, "profit_loss")
}

func (h *Handler) AccountingVATLedger(c *fiber.Ctx) error {
	return h.accountingReport(c, "vat")
}

func (h *Handler) AccountingBackfillIssues(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"accounting.journal_view"}); err != nil {
		return err
	}
	projectID := cleanID(c.Query("project_id"))
	if projectID != "" {
		if err := h.requireProjectPermission(c, projectID, []string{"accounting.journal_view"}); err != nil {
			return err
		}
	}
	rows, err := h.service.AccountingBackfillIssues(c.Context(), actorID(c), projectID)
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) ResolveAccountingBackfillIssue(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "accounting_backfill_issues", c.Params("id"), []string{"accounting.journal_post"}); err != nil {
		return err
	}
	var req AccountingReconciliationResolveRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return validationError("Dữ liệu xử lý đối chiếu không hợp lệ")
	}
	row, err := h.service.ResolveAccountingBackfillIssue(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) accountingReport(c *fiber.Ctx, kind string) error {
	permission := "accounting.journal_view"
	if kind == "vat" {
		permission = "accounting.tax_view"
	}
	if err := h.requirePermission(c, []string{permission}); err != nil {
		return err
	}
	filter := AccountingReportFilter{ProjectID: c.Query("project_id"), DateFrom: c.Query("date_from"), DateTo: c.Query("date_to")}
	if filter.ProjectID != "" {
		if err := h.requireProjectPermission(c, cleanID(filter.ProjectID), []string{permission}); err != nil {
			return err
		}
	}
	var result Record
	var err error
	switch kind {
	case "trial_balance":
		result, err = h.service.AccountingTrialBalance(c.Context(), actorID(c), filter)
	case "general_ledger":
		result, err = h.service.AccountingGeneralLedger(c.Context(), actorID(c), filter)
	case "profit_loss":
		result, err = h.service.AccountingProfitLoss(c.Context(), actorID(c), filter)
	case "vat":
		result, err = h.service.AccountingVATLedger(c.Context(), actorID(c), filter)
	}
	if err != nil {
		return err
	}
	return response.OK(c, result)
}
