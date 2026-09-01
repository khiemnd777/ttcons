package core

import (
	"consflow/backend/internal/shared/response"
	"encoding/json"
	"fmt"
	fiber "github.com/gofiber/fiber/v2"
	"strconv"
	"strings"
)

func (h *Handler) ProjectBudget(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"accounting.budget_view"}); err != nil {
		return err
	}
	rows, err := h.service.ProjectBudget(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateBudget(c *fiber.Ctx) error {
	if err := h.requireProjectPermission(c, c.Params("id"), []string{"accounting.budget_approve"}); err != nil {
		return err
	}
	var req BudgetCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return validationError("Dữ liệu ngân sách không hợp lệ")
	}
	row, err := h.service.CreateBudget(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ApproveBudget(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "accounting_budgets", c.Params("id"), []string{"accounting.budget_approve"}); err != nil {
		return err
	}
	row, err := h.service.ApproveBudget(c.Context(), c.Params("id"), actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ListPayables(c *fiber.Ctx) error {
	permissions := []string{"accounting.view", "accounting.payable_view"}
	if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	scopeSQL, scopeArgs, err := h.projectAccessFilter(c, "p", 1)
	if err != nil {
		return err
	}
	projectID := cleanID(c.Query("project_id"))
	if projectID != "" {
		if !validActivityUUID(projectID) {
			return validationError("project_id không hợp lệ", Record{"project_id": projectID})
		}
		if err := h.requireProjectPermission(c, projectID, permissions); err != nil {
			return err
		}
		scopeSQL = fmt.Sprintf("(%s) AND pay.project_id=$%d::uuid", scopeSQL, len(scopeArgs)+1)
		scopeArgs = append(scopeArgs, projectID)
	}
	return h.list(c, fmt.Sprintf(`
		SELECT
			pay.*,
			p.name AS project_name,
			COALESCE(inv.invoice_count, 0) AS invoice_count,
			COALESCE(inv.invoice_total, 0) AS invoice_total,
			COALESCE(inv.verified_invoice_total, 0) AS verified_invoice_total,
			COALESCE(sch.schedule_count, 0) AS schedule_count,
			COALESCE(sch.scheduled_total, 0) AS scheduled_total,
			COALESCE(sch.scheduled_remaining, 0) AS scheduled_remaining,
			COALESCE(adv.advance_request_count, 0) AS advance_request_count,
			COALESCE(adv.advance_requested_total, 0) AS advance_requested_total,
			COALESCE(adv.advance_approved_total, 0) AS advance_approved_total,
			COALESCE(adv.advance_paid_total, 0) AS advance_paid_total,
			COALESCE(adv.advance_settled_total, 0) AS advance_settled_total,
			COALESCE(payments.approved_payment_total, 0) AS approved_payment_total,
			latest.latest_invoice_status,
			latest.latest_invoice_number,
			latest.latest_invoice_date,
			COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date) AS effective_due_date,
			CASE
				WHEN COALESCE(pay.remaining_amount,0) <= 0 THEN 0
				WHEN COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date) IS NULL THEN 0
				WHEN COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date) >= CURRENT_DATE THEN 0
				ELSE CURRENT_DATE - COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date)
			END AS overdue_days,
			CASE
				WHEN COALESCE(pay.remaining_amount,0) <= 0 THEN 'settled'
				WHEN COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date) IS NULL THEN 'no_due_date'
				WHEN COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date) >= CURRENT_DATE THEN 'not_due'
				WHEN CURRENT_DATE - COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date) <= 30 THEN '1_30'
				WHEN CURRENT_DATE - COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date) <= 60 THEN '31_60'
				WHEN CURRENT_DATE - COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date) <= 90 THEN '61_90'
				ELSE '90_plus'
			END AS aging_bucket,
			(
				CASE WHEN COALESCE(pay.remaining_amount,0) > 0 AND COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date) < CURRENT_DATE THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(inv.verified_invoice_total,0) > COALESCE(pay.gross_amount,0) + 1 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(pay.paid_amount,0) > COALESCE(inv.verified_invoice_total,0) + 1 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(pay.remaining_amount,0) > 0 AND COALESCE(sch.schedule_count,0) = 0 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(sch.scheduled_remaining,0) > COALESCE(pay.remaining_amount,0) + 1 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(adv.advance_approved_total,0) > COALESCE(adv.advance_paid_total,0) + 1 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(adv.advance_paid_total,0) > COALESCE(adv.advance_settled_total,0) + 1 THEN 1 ELSE 0 END
			) AS reconciliation_issue_count,
			CASE
				WHEN (
					CASE WHEN COALESCE(pay.remaining_amount,0) > 0 AND COALESCE(pay.due_date, sch.next_due_date, latest.latest_invoice_date) < CURRENT_DATE THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(inv.verified_invoice_total,0) > COALESCE(pay.gross_amount,0) + 1 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(pay.paid_amount,0) > COALESCE(inv.verified_invoice_total,0) + 1 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(pay.remaining_amount,0) > 0 AND COALESCE(sch.schedule_count,0) = 0 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(sch.scheduled_remaining,0) > COALESCE(pay.remaining_amount,0) + 1 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(adv.advance_approved_total,0) > COALESCE(adv.advance_paid_total,0) + 1 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(adv.advance_paid_total,0) > COALESCE(adv.advance_settled_total,0) + 1 THEN 1 ELSE 0 END
				) > 0 THEN 'needs_review'
				ELSE 'ok'
			END AS reconciliation_status
		FROM payables pay
		JOIN projects p ON p.id=pay.project_id
		LEFT JOIN (
			SELECT
				payable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('deleted','cancelled')) AS invoice_count,
				COALESCE(SUM(total_amount) FILTER (WHERE status NOT IN ('deleted','cancelled')), 0) AS invoice_total,
				COALESCE(SUM(total_amount) FILTER (WHERE status='verified'), 0) AS verified_invoice_total
			FROM input_invoices
			GROUP BY payable_id
		) inv ON inv.payable_id=pay.id
		LEFT JOIN (
			SELECT
				payable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('deleted','cancelled')) AS schedule_count,
				COALESCE(SUM(planned_amount) FILTER (WHERE status NOT IN ('deleted','cancelled')), 0) AS scheduled_total,
				COALESCE(SUM(remaining_amount) FILTER (WHERE status NOT IN ('deleted','cancelled')), 0) AS scheduled_remaining,
				MIN(due_date) FILTER (WHERE status IN ('approved','partially_paid','overdue') AND remaining_amount > 0) AS next_due_date
			FROM payable_payment_schedules
			GROUP BY payable_id
		) sch ON sch.payable_id=pay.id
		LEFT JOIN (
			SELECT
				payable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('deleted','cancelled')) AS advance_request_count,
				COALESCE(SUM(amount) FILTER (WHERE status NOT IN ('deleted','cancelled')), 0) AS advance_requested_total,
				COALESCE(SUM(amount) FILTER (WHERE status IN ('approved','partially_settled','settled')), 0) AS advance_approved_total,
				COALESCE(SUM(paid_amount) FILTER (WHERE status NOT IN ('deleted','cancelled')), 0) AS advance_paid_total,
				COALESCE(SUM(settled_amount) FILTER (WHERE status NOT IN ('deleted','cancelled')), 0) AS advance_settled_total
			FROM advance_requests
			WHERE payable_id IS NOT NULL
			GROUP BY payable_id
		) adv ON adv.payable_id=pay.id
		LEFT JOIN (
			SELECT
				pa.payable_id,
				COALESCE(SUM(pa.amount) FILTER (WHERE pmt.status='approved' AND pa.allocation_type='payable_payment'), 0) AS approved_payment_total
			FROM payment_allocations pa
			JOIN payments pmt ON pmt.id=pa.payment_id
			GROUP BY pa.payable_id
		) payments ON payments.payable_id=pay.id
		LEFT JOIN LATERAL (
			SELECT status AS latest_invoice_status, invoice_number AS latest_invoice_number, invoice_date AS latest_invoice_date
			FROM input_invoices ii
			WHERE ii.payable_id=pay.id AND ii.status NOT IN ('deleted','cancelled')
			ORDER BY ii.created_at DESC
			LIMIT 1
		) latest ON true
		WHERE pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		  AND %s
		ORDER BY pay.created_at DESC
	`, scopeSQL), scopeArgs...)
}

func (h *Handler) CreatePayable(c *fiber.Ctx) error {
	var req PayableCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return err
	}
	if err := h.requireProjectPermission(c, cleanID(req.ProjectID), []string{"accounting.payable_approve"}); err != nil {
		return err
	}
	row, err := h.service.CreatePayable(c.Context(), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ApprovePayable(c *fiber.Ctx) error {
	return h.payableAction(c, "approve")
}

func (h *Handler) PayPayable(c *fiber.Ctx) error {
	return h.payableAction(c, "pay")
}

func (h *Handler) ListPayablePaymentSchedules(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "payables", c.Params("id"), []string{"accounting.view", "accounting.payable_view"}); err != nil {
		return err
	}
	rows, err := h.service.PayablePaymentSchedules(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreatePayablePaymentSchedule(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "payables", c.Params("id"), []string{"accounting.payment_create", "accounting.payable_approve"}); err != nil {
		return err
	}
	var req PayablePaymentScheduleRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu kỳ thanh toán không hợp lệ")
	}
	row, err := h.service.CreatePayablePaymentSchedule(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ApprovePayablePaymentSchedule(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "payable_payment_schedules", c.Params("id"), []string{"accounting.payable_approve", "accounting.payment_approve"}); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.PayablePaymentScheduleAction(c.Context(), c.Params("id"), "approve", req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ListPayableAdvanceRequests(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "payables", c.Params("id"), []string{"accounting.view", "accounting.payable_view"}); err != nil {
		return err
	}
	rows, err := h.service.PayableAdvanceRequests(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreatePayableAdvanceRequest(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "payables", c.Params("id"), []string{"accounting.payment_create", "accounting.payable_approve"}); err != nil {
		return err
	}
	var req AdvanceRequestCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu ứng trước không hợp lệ")
	}
	row, err := h.service.CreatePayableAdvanceRequest(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ApprovePayableAdvanceRequest(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "advance_requests", c.Params("id"), []string{"accounting.payable_approve", "accounting.payment_approve"}); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.PayableAdvanceRequestAction(c.Context(), c.Params("id"), "approve", req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) SettlePayableAdvanceRequest(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "advance_requests", c.Params("id"), []string{"accounting.payable_approve", "accounting.payment_approve"}); err != nil {
		return err
	}
	var req AdvanceSettlementRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu quyết toán ứng trước không hợp lệ")
	}
	row, err := h.service.SettlePayableAdvanceRequest(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ListPayablePayments(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "payables", c.Params("id"), []string{"accounting.view", "accounting.payable_view", "accounting.payment_create", "accounting.payment_approve"}); err != nil {
		return err
	}
	rows, err := h.service.PayablePayments(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreatePayablePayment(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "payables", c.Params("id"), []string{"accounting.payment_create"}); err != nil {
		return err
	}
	var req PayablePaymentCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu thanh toán không hợp lệ")
	}
	row, err := h.service.CreatePayablePayment(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ListPayableInputInvoices(c *fiber.Ctx) error {
	rows, err := h.service.PayableInputInvoices(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) GetPayableReconciliation(c *fiber.Ctx) error {
	data, err := h.service.PayableReconciliation(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}

func (h *Handler) ProjectInputInvoices(c *fiber.Ctx) error {
	rows, err := h.service.ProjectInputInvoices(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) UploadPayableInputInvoice(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "File PDF hoá đơn là bắt buộc")
	}
	amountBeforeVAT, err := parseFormFloat(c.FormValue("amount_before_vat"))
	if err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Giá trị trước VAT không hợp lệ")
	}
	vatAmount, err := parseFormFloat(c.FormValue("vat_amount"))
	if err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Tiền VAT không hợp lệ")
	}
	totalAmount, err := parseFormFloat(c.FormValue("total_amount"))
	if err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Tổng tiền không hợp lệ")
	}
	req := InputInvoiceRequest{
		InvoiceNumber:        c.FormValue("invoice_number"),
		InvoiceSymbol:        c.FormValue("invoice_symbol"),
		InvoiceDate:          c.FormValue("invoice_date"),
		VendorPartnerID:      c.FormValue("vendor_partner_id"),
		VendorName:           c.FormValue("vendor_name"),
		VendorTaxCode:        c.FormValue("vendor_tax_code"),
		AmountBeforeVAT:      amountBeforeVAT,
		VATAmount:            vatAmount,
		TotalAmount:          totalAmount,
		ProcurementRequestID: c.FormValue("procurement_request_id"),
		DocumentID:           c.FormValue("document_id"),
		Notes:                c.FormValue("notes"),
	}
	record, err := h.service.CreateInputInvoice(c.Context(), c.Params("id"), req, fileHeader, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, record)
}

func (h *Handler) VerifyInputInvoice(c *fiber.Ctx) error {
	return h.inputInvoiceAction(c, "verify")
}

func (h *Handler) RejectInputInvoice(c *fiber.Ctx) error {
	return h.inputInvoiceAction(c, "reject")
}

func (h *Handler) inputInvoiceAction(c *fiber.Ctx, action string) error {
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.InputInvoiceAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) GetInputInvoiceFile(c *fiber.Ctx) error {
	file, err := h.service.InputInvoiceFile(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return sendProtectedFile(c, file, "input-invoice.pdf")
}

func parseFormFloat(value string) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	return strconv.ParseFloat(value, 64)
}

func (h *Handler) ListReceivables(c *fiber.Ctx) error {
	permissions := []string{"accounting.view", "accounting.receivable_view"}
	if err := h.requirePermission(c, permissions); err != nil {
		return err
	}
	scopeSQL, scopeArgs, err := h.projectAccessFilter(c, "p", 1)
	if err != nil {
		return err
	}
	projectID := cleanID(c.Query("project_id"))
	if projectID != "" {
		if !validActivityUUID(projectID) {
			return validationError("project_id không hợp lệ", Record{"project_id": projectID})
		}
		if err := h.requireProjectPermission(c, projectID, permissions); err != nil {
			return err
		}
		scopeSQL = fmt.Sprintf("(%s) AND rec.project_id=$%d::uuid", scopeSQL, len(scopeArgs)+1)
		scopeArgs = append(scopeArgs, projectID)
	}
	return h.list(c, fmt.Sprintf(`
		SELECT
			rec.*,
			p.name AS project_name,
			COALESCE(inv.invoice_count, 0) AS invoice_count,
			COALESCE(inv.invoice_total, 0) AS invoice_total,
			COALESCE(inv.issued_invoice_total, 0) AS issued_invoice_total,
			COALESCE(sched.schedule_count, 0) AS schedule_count,
			COALESCE(sched.scheduled_total, 0) AS scheduled_total,
			COALESCE(sched.scheduled_remaining, 0) AS scheduled_remaining,
			COALESCE(adv.advance_receipt_count, 0) AS advance_receipt_count,
			COALESCE(adv.advance_requested_total, 0) AS advance_requested_total,
			COALESCE(adv.advance_approved_total, 0) AS advance_approved_total,
			COALESCE(adv.advance_received_total, 0) AS advance_received_total,
			COALESCE(adv.advance_settled_total, 0) AS advance_settled_total,
			COALESCE(receipts.approved_receipt_total, 0) AS approved_receipt_total,
			COALESCE(receipts.approved_advance_receipt_total, 0) AS approved_advance_receipt_total,
			COALESCE(receipts.receipt_allocation_total, 0) AS receipt_allocation_total,
			COALESCE(receipts.pending_receipt_count, 0) AS pending_receipt_count,
			latest.latest_invoice_status,
			latest.latest_invoice_number,
			latest.latest_invoice_date,
			COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date) AS effective_due_date,
			CASE
				WHEN COALESCE(rec.remaining_amount,0) <= 0 THEN 0
				WHEN COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date) IS NULL THEN 0
				WHEN COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date) >= CURRENT_DATE THEN 0
				ELSE CURRENT_DATE - COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date)
			END AS overdue_days,
			CASE
				WHEN COALESCE(rec.remaining_amount,0) <= 0 THEN 'settled'
				WHEN COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date) IS NULL THEN 'no_due_date'
				WHEN COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date) >= CURRENT_DATE THEN 'not_due'
				WHEN CURRENT_DATE - COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date) <= 30 THEN '1_30'
				WHEN CURRENT_DATE - COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date) <= 60 THEN '31_60'
				WHEN CURRENT_DATE - COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date) <= 90 THEN '61_90'
				ELSE '90_plus'
			END AS aging_bucket,
			(
				CASE WHEN COALESCE(rec.remaining_amount,0) > 0 AND COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date) < CURRENT_DATE THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(inv.issued_invoice_total,0) > COALESCE(rec.gross_amount,0) + 1 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(rec.received_amount,0) > COALESCE(inv.issued_invoice_total,0) + 1 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(rec.remaining_amount,0) > 0 AND COALESCE(sched.schedule_count,0) = 0 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(sched.scheduled_remaining,0) > COALESCE(rec.remaining_amount,0) + 1 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(adv.advance_approved_total,0) > COALESCE(adv.advance_received_total,0) + 1 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(adv.advance_received_total,0) > COALESCE(adv.advance_settled_total,0) + 1 THEN 1 ELSE 0 END
				+ CASE WHEN COALESCE(receipts.pending_receipt_count,0) > 0 THEN 1 ELSE 0 END
			) AS reconciliation_issue_count,
			CASE
				WHEN (
					CASE WHEN COALESCE(rec.remaining_amount,0) > 0 AND COALESCE(rec.due_date, sched.next_due_date, latest.latest_invoice_date) < CURRENT_DATE THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(inv.issued_invoice_total,0) > COALESCE(rec.gross_amount,0) + 1 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(rec.received_amount,0) > COALESCE(inv.issued_invoice_total,0) + 1 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(rec.remaining_amount,0) > 0 AND COALESCE(sched.schedule_count,0) = 0 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(sched.scheduled_remaining,0) > COALESCE(rec.remaining_amount,0) + 1 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(adv.advance_approved_total,0) > COALESCE(adv.advance_received_total,0) + 1 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(adv.advance_received_total,0) > COALESCE(adv.advance_settled_total,0) + 1 THEN 1 ELSE 0 END
					+ CASE WHEN COALESCE(receipts.pending_receipt_count,0) > 0 THEN 1 ELSE 0 END
				) > 0 THEN 'needs_review'
				ELSE 'ok'
			END AS reconciliation_status
		FROM receivables rec
		JOIN projects p ON p.id=rec.project_id
		LEFT JOIN (
			SELECT
				receivable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('deleted','cancelled')) AS invoice_count,
				COALESCE(SUM(total_amount) FILTER (WHERE status NOT IN ('deleted','cancelled')), 0) AS invoice_total,
				COALESCE(SUM(total_amount) FILTER (WHERE status='issued'), 0) AS issued_invoice_total
			FROM output_invoices
			GROUP BY receivable_id
		) inv ON inv.receivable_id=rec.id
		LEFT JOIN LATERAL (
			SELECT status AS latest_invoice_status, invoice_number AS latest_invoice_number, invoice_date AS latest_invoice_date
			FROM output_invoices oi
			WHERE oi.receivable_id=rec.id AND oi.status NOT IN ('deleted','cancelled')
			ORDER BY oi.created_at DESC
			LIMIT 1
		) latest ON true
		LEFT JOIN (
			SELECT
				receivable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('cancelled','deleted')) AS schedule_count,
				COALESCE(SUM(planned_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS scheduled_total,
				COALESCE(SUM(remaining_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS scheduled_remaining,
				MIN(due_date) FILTER (WHERE status IN ('approved','partially_received','overdue') AND remaining_amount > 0) AS next_due_date
			FROM receivable_payment_schedules
			GROUP BY receivable_id
		) sched ON sched.receivable_id=rec.id
		LEFT JOIN (
			SELECT
				receivable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('cancelled','deleted')) AS advance_receipt_count,
				COALESCE(SUM(amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS advance_requested_total,
				COALESCE(SUM(amount) FILTER (WHERE status IN ('approved','partially_received','received','partially_settled','settled')), 0) AS advance_approved_total,
				COALESCE(SUM(received_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS advance_received_total,
				COALESCE(SUM(settled_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS advance_settled_total
			FROM client_advance_receipts
			GROUP BY receivable_id
		) adv ON adv.receivable_id=rec.id
		LEFT JOIN (
			SELECT
				ra.receivable_id,
				COALESCE(SUM(ra.amount) FILTER (WHERE pmt.status='approved' AND ra.allocation_type='receivable_receipt'), 0) AS approved_receipt_total,
				COALESCE(SUM(ra.amount) FILTER (WHERE pmt.status='approved' AND ra.allocation_type='advance_receipt'), 0) AS approved_advance_receipt_total,
				COALESCE(SUM(ra.amount) FILTER (WHERE pmt.status <> 'cancelled'), 0) AS receipt_allocation_total,
				COUNT(*) FILTER (WHERE pmt.status NOT IN ('approved','cancelled')) AS pending_receipt_count
			FROM receipt_allocations ra
			JOIN payments pmt ON pmt.id=ra.payment_id
			GROUP BY ra.receivable_id
		) receipts ON receipts.receivable_id=rec.id
		WHERE rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		  AND %s
		ORDER BY rec.created_at DESC
	`, scopeSQL), scopeArgs...)
}

func (h *Handler) CreateReceivable(c *fiber.Ctx) error {
	var req ReceivableCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return err
	}
	if err := h.requireProjectPermission(c, cleanID(req.ProjectID), []string{"accounting.receivable_approve"}); err != nil {
		return err
	}
	row, err := h.service.CreateReceivable(c.Context(), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ListReceivablePaymentSchedules(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "receivables", c.Params("id"), []string{"accounting.view", "accounting.receivable_view"}); err != nil {
		return err
	}
	rows, err := h.service.ReceivablePaymentSchedules(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateReceivablePaymentSchedule(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "receivables", c.Params("id"), []string{"accounting.payment_create", "accounting.receivable_approve"}); err != nil {
		return err
	}
	var req PayablePaymentScheduleRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu kỳ thu không hợp lệ")
	}
	row, err := h.service.CreateReceivablePaymentSchedule(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ApproveReceivablePaymentSchedule(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "receivable_payment_schedules", c.Params("id"), []string{"accounting.receivable_approve", "accounting.payment_approve"}); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.ReceivablePaymentScheduleAction(c.Context(), c.Params("id"), "approve", req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ListReceivableAdvanceReceipts(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "receivables", c.Params("id"), []string{"accounting.view", "accounting.receivable_view"}); err != nil {
		return err
	}
	rows, err := h.service.ReceivableAdvanceReceipts(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateReceivableAdvanceReceipt(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "receivables", c.Params("id"), []string{"accounting.payment_create", "accounting.receivable_approve"}); err != nil {
		return err
	}
	var req ReceivableAdvanceReceiptCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu ứng trước đầu ra không hợp lệ")
	}
	row, err := h.service.CreateReceivableAdvanceReceipt(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ApproveReceivableAdvanceReceipt(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "client_advance_receipts", c.Params("id"), []string{"accounting.receivable_approve", "accounting.payment_approve"}); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.ReceivableAdvanceReceiptAction(c.Context(), c.Params("id"), "approve", req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) SettleReceivableAdvanceReceipt(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "client_advance_receipts", c.Params("id"), []string{"accounting.receivable_approve", "accounting.payment_approve"}); err != nil {
		return err
	}
	var req ReceivableAdvanceSettlementRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu quyết toán ứng trước đầu ra không hợp lệ")
	}
	row, err := h.service.SettleReceivableAdvanceReceipt(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) ListReceivablePayments(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "receivables", c.Params("id"), []string{"accounting.view", "accounting.receivable_view", "accounting.payment_create", "accounting.payment_approve"}); err != nil {
		return err
	}
	rows, err := h.service.ReceivablePayments(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) CreateReceivablePayment(c *fiber.Ctx) error {
	if err := h.requireRecordProjectPermission(c, "receivables", c.Params("id"), []string{"accounting.payment_create"}); err != nil {
		return err
	}
	var req ReceivableReceiptCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Dữ liệu phiếu thu không hợp lệ")
	}
	row, err := h.service.CreateReceivablePayment(c.Context(), c.Params("id"), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ListReceivableOutputInvoices(c *fiber.Ctx) error {
	rows, err := h.service.ReceivableOutputInvoices(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) GetReceivableReconciliation(c *fiber.Ctx) error {
	data, err := h.service.ReceivableReconciliation(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, data)
}

func (h *Handler) ProjectOutputInvoices(c *fiber.Ctx) error {
	rows, err := h.service.ProjectOutputInvoices(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) UploadReceivableOutputInvoice(c *fiber.Ctx) error {
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "File PDF hoá đơn là bắt buộc")
	}
	amountBeforeVAT, err := parseFormFloat(c.FormValue("amount_before_vat"))
	if err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Giá trị trước VAT không hợp lệ")
	}
	vatAmount, err := parseFormFloat(c.FormValue("vat_amount"))
	if err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Tiền VAT không hợp lệ")
	}
	totalAmount, err := parseFormFloat(c.FormValue("total_amount"))
	if err != nil {
		return response.Fail(c, fiber.StatusBadRequest, "VALIDATION_ERROR", "Tổng tiền không hợp lệ")
	}
	req := OutputInvoiceRequest{
		InvoiceNumber:   c.FormValue("invoice_number"),
		InvoiceSymbol:   c.FormValue("invoice_symbol"),
		InvoiceDate:     c.FormValue("invoice_date"),
		ClientPartnerID: c.FormValue("client_partner_id"),
		ClientName:      c.FormValue("client_name"),
		ClientTaxCode:   c.FormValue("client_tax_code"),
		AmountBeforeVAT: amountBeforeVAT,
		VATAmount:       vatAmount,
		TotalAmount:     totalAmount,
		PaymentID:       c.FormValue("payment_id"),
		DocumentID:      c.FormValue("document_id"),
		Notes:           c.FormValue("notes"),
	}
	record, err := h.service.CreateOutputInvoice(c.Context(), c.Params("id"), req, fileHeader, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, record)
}

func (h *Handler) IssueOutputInvoice(c *fiber.Ctx) error {
	return h.outputInvoiceAction(c, "issue")
}

func (h *Handler) VoidOutputInvoice(c *fiber.Ctx) error {
	return h.outputInvoiceAction(c, "void")
}

func (h *Handler) outputInvoiceAction(c *fiber.Ctx, action string) error {
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.OutputInvoiceAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) GetOutputInvoiceFile(c *fiber.Ctx) error {
	file, err := h.service.OutputInvoiceFile(c.Context(), c.Params("id"), actorID(c))
	if err != nil {
		return err
	}
	return sendProtectedFile(c, file, "output-invoice.pdf")
}

func (h *Handler) ApproveReceivable(c *fiber.Ctx) error {
	return h.receivableAction(c, "approve")
}

func (h *Handler) ReceiveReceivable(c *fiber.Ctx) error {
	return h.receivableAction(c, "receive")
}

func (h *Handler) ListPayments(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"accounting.view", "accounting.payment_create", "accounting.payment_approve"}); err != nil {
		return err
	}
	scopeSQL, scopeArgs, err := h.projectAccessFilter(c, "p", 1)
	if err != nil {
		return err
	}
	return h.list(c, fmt.Sprintf(`SELECT pay.*, p.name AS project_name FROM payments pay JOIN projects p ON p.id=pay.project_id WHERE pay.status <> 'cancelled' AND p.status <> 'cancelled' AND %s ORDER BY pay.created_at DESC`, scopeSQL), scopeArgs...)
}

func (h *Handler) CreatePayment(c *fiber.Ctx) error {
	var req PaymentCreateRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return validationError("Dữ liệu phiếu thu/chi không hợp lệ")
	}
	if err := h.requireProjectPermission(c, cleanID(req.ProjectID), []string{"accounting.payment_create"}); err != nil {
		return err
	}
	row, err := h.service.CreatePayment(c.Context(), req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.Created(c, row)
}

func (h *Handler) ApprovePayment(c *fiber.Ctx) error {
	return h.paymentAction(c, "approve")
}

func (h *Handler) ReversePayment(c *fiber.Ctx) error {
	return h.paymentAction(c, "reverse")
}

func (h *Handler) payableAction(c *fiber.Ctx, action string) error {
	permissions := []string{"accounting.payable_approve"}
	if action == "pay" {
		permissions = []string{"accounting.payable_approve", "accounting.payment_approve"}
	}
	if err := h.requireRecordProjectPermission(c, "payables", c.Params("id"), permissions); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.PayableAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) receivableAction(c *fiber.Ctx, action string) error {
	permissions := []string{"accounting.receivable_approve"}
	if action == "receive" {
		permissions = []string{"accounting.receivable_approve", "accounting.payment_approve"}
	}
	if err := h.requireRecordProjectPermission(c, "receivables", c.Params("id"), permissions); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.ReceivableAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}

func (h *Handler) paymentAction(c *fiber.Ctx, action string) error {
	permissions := []string{"accounting.payment_approve"}
	if action == "reverse" {
		permissions = []string{"accounting.journal_reverse"}
	}
	if err := h.requireRecordProjectPermission(c, "payments", c.Params("id"), permissions); err != nil {
		return err
	}
	req, err := actionRequest(c)
	if err != nil {
		return err
	}
	row, err := h.service.PaymentAction(c.Context(), c.Params("id"), action, req, actorID(c), actorName(c))
	if err != nil {
		return err
	}
	return response.OK(c, row)
}
