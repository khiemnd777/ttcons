package core

import (
	"context"
	"fmt"
	"strings"
)

func (s *Service) ReceivablePaymentSchedules(ctx context.Context, receivableID, actorID string) ([]Record, error) {
	receivable, err := s.requireReceivableAccess(ctx, receivableID, actorID, []string{"accounting.view", "accounting.receivable_view"}, "Bạn không có quyền xem lịch thu đầu ra")
	if err != nil {
		return nil, err
	}
	return s.repo.ListReceivablePaymentSchedules(ctx, cleanID(receivable["id"]))
}

func (s *Service) CreateReceivablePaymentSchedule(ctx context.Context, receivableID string, req PayablePaymentScheduleRequest, actorID, actorName string) (Record, error) {
	receivable, err := s.requireReceivableAccess(ctx, receivableID, actorID, []string{"accounting.payment_create", "accounting.receivable_approve"}, "Bạn không có quyền tạo kỳ thu đầu ra")
	if err != nil {
		return nil, err
	}
	body, err := s.normalizeReceivablePaymentScheduleRequest(receivable, req)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateReceivablePaymentSchedule(ctx, receivable, body, actorID, actorName)
}

func (s *Service) ReceivablePaymentScheduleAction(ctx context.Context, scheduleID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	scheduleID = cleanID(scheduleID)
	if !validActivityUUID(scheduleID) {
		return nil, validationError("schedule_id không hợp lệ", Record{"schedule_id": scheduleID})
	}
	return s.repo.ReceivablePaymentScheduleAction(ctx, scheduleID, action, req, actorID, actorName)
}

func (s *Service) ReceivableAdvanceReceipts(ctx context.Context, receivableID, actorID string) ([]Record, error) {
	receivable, err := s.requireReceivableAccess(ctx, receivableID, actorID, []string{"accounting.view", "accounting.receivable_view"}, "Bạn không có quyền xem ứng trước đầu ra")
	if err != nil {
		return nil, err
	}
	return s.repo.ListReceivableAdvanceReceipts(ctx, cleanID(receivable["id"]))
}

func (s *Service) CreateReceivableAdvanceReceipt(ctx context.Context, receivableID string, req ReceivableAdvanceReceiptCreateRequest, actorID, actorName string) (Record, error) {
	receivable, err := s.requireReceivableAccess(ctx, receivableID, actorID, []string{"accounting.payment_create", "accounting.receivable_approve"}, "Bạn không có quyền tạo ứng trước đầu ra")
	if err != nil {
		return nil, err
	}
	body, err := s.normalizeReceivableAdvanceReceiptRequest(receivable, req)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateReceivableAdvanceReceipt(ctx, receivable, body, actorID, actorName)
}

func (s *Service) ReceivableAdvanceReceiptAction(ctx context.Context, advanceID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	advanceID = cleanID(advanceID)
	if !validActivityUUID(advanceID) {
		return nil, validationError("advance_receipt_id không hợp lệ", Record{"advance_receipt_id": advanceID})
	}
	return s.repo.ReceivableAdvanceReceiptAction(ctx, advanceID, action, req, actorID, actorName)
}

func (s *Service) SettleReceivableAdvanceReceipt(ctx context.Context, advanceID string, req ReceivableAdvanceSettlementRequest, actorID, actorName string) (Record, error) {
	advanceID = cleanID(advanceID)
	if !validActivityUUID(advanceID) {
		return nil, validationError("advance_receipt_id không hợp lệ", Record{"advance_receipt_id": advanceID})
	}
	advance, err := s.repo.GetReceivableAdvanceReceiptDetail(ctx, advanceID)
	if err != nil {
		return nil, err
	}
	body, err := s.normalizeReceivableAdvanceSettlementRequest(ctx, advance, req)
	if err != nil {
		return nil, err
	}
	return s.repo.SettleReceivableAdvanceReceipt(ctx, advanceID, body, actorID, actorName)
}

func (s *Service) ReceivablePayments(ctx context.Context, receivableID, actorID string) ([]Record, error) {
	receivable, err := s.requireReceivableAccess(ctx, receivableID, actorID, []string{"accounting.view", "accounting.receivable_view", "accounting.payment_create", "accounting.payment_approve"}, "Bạn không có quyền xem phiếu thu đầu ra")
	if err != nil {
		return nil, err
	}
	return s.repo.ListReceivablePayments(ctx, cleanID(receivable["id"]))
}

func (s *Service) CreateReceivablePayment(ctx context.Context, receivableID string, req ReceivableReceiptCreateRequest, actorID, actorName string) (Record, error) {
	receivable, err := s.requireReceivableAccess(ctx, receivableID, actorID, []string{"accounting.payment_create"}, "Bạn không có quyền tạo phiếu thu đầu ra")
	if err != nil {
		return nil, err
	}
	body, err := s.normalizeReceivablePaymentRequest(ctx, receivable, req)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateReceivablePayment(ctx, receivable, body, actorID, actorName)
}

func (s *Service) normalizeReceivablePaymentScheduleRequest(receivable Record, req PayablePaymentScheduleRequest) (Record, error) {
	scheduleCode := strings.TrimSpace(req.ScheduleCode)
	if scheduleCode == "" {
		return nil, validationError("Mã kỳ thu là bắt buộc")
	}
	if len([]rune(scheduleCode)) > 80 {
		return nil, validationError("Mã kỳ thu không được vượt quá 80 ký tự")
	}
	milestoneName := strings.TrimSpace(req.MilestoneName)
	if milestoneName == "" {
		return nil, validationError("Tên kỳ thu là bắt buộc")
	}
	if len([]rune(milestoneName)) > 180 {
		return nil, validationError("Tên kỳ thu không được vượt quá 180 ký tự")
	}
	scheduleType := strings.TrimSpace(req.ScheduleType)
	if scheduleType == "" {
		scheduleType = "progress"
	}
	if !statusIn(scheduleType, "advance", "progress", "final", "retention", "other") {
		return nil, validationError("Loại kỳ thu không hợp lệ", Record{"schedule_type": scheduleType})
	}
	if req.PlannedAmount <= 0 {
		return nil, validationError("Số tiền kế hoạch phải lớn hơn 0")
	}
	if number(receivable["gross_amount"]) > 0 && req.PlannedAmount > number(receivable["gross_amount"]) {
		return nil, validationError("Số tiền kỳ thu vượt giá trị phải thu", Record{"planned_amount": req.PlannedAmount})
	}
	dueDate, err := normalizeOptionalDate(req.DueDate, "due_date")
	if err != nil {
		return nil, err
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, validationError("Ghi chú kỳ thu không được vượt quá 2000 ký tự")
	}
	return Record{
		"project_id":              cleanID(receivable["project_id"]),
		"receivable_id":           cleanID(receivable["id"]),
		"schedule_code":           scheduleCode,
		"milestone_name":          milestoneName,
		"schedule_type":           scheduleType,
		"planned_amount":          req.PlannedAmount,
		"received_amount":         0,
		"remaining_amount":        req.PlannedAmount,
		"due_date":                dueDate,
		"status":                  "draft",
		"notes":                   notes,
		"receivable_remaining":    number(receivable["remaining_amount"]),
		"receivable_gross_amount": number(receivable["gross_amount"]),
	}, nil
}

func (s *Service) normalizeReceivableAdvanceReceiptRequest(receivable Record, req ReceivableAdvanceReceiptCreateRequest) (Record, error) {
	receiptCode := strings.TrimSpace(req.ReceiptCode)
	if receiptCode == "" {
		return nil, validationError("Mã ứng trước là bắt buộc")
	}
	if len([]rune(receiptCode)) > 80 {
		return nil, validationError("Mã ứng trước không được vượt quá 80 ký tự")
	}
	if req.Amount <= 0 {
		return nil, validationError("Số tiền ứng trước phải lớn hơn 0")
	}
	remaining := number(receivable["remaining_amount"])
	if remaining > 0 && req.Amount > remaining {
		return nil, validationError("Số tiền ứng trước vượt số còn phải thu", Record{"amount": req.Amount, "remaining_amount": remaining})
	}
	dueDate, err := normalizeOptionalDate(req.DueDate, "due_date")
	if err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(req.Reason)
	if len([]rune(reason)) > 1000 {
		return nil, validationError("Lý do ứng trước không được vượt quá 1000 ký tự")
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, validationError("Ghi chú ứng trước không được vượt quá 2000 ký tự")
	}
	return Record{
		"project_id":    cleanID(receivable["project_id"]),
		"receivable_id": cleanID(receivable["id"]),
		"receipt_code":  receiptCode,
		"client_name":   cleanOptionalText(receivable["client_name"]),
		"amount":        req.Amount,
		"due_date":      dueDate,
		"reason":        reason,
		"notes":         notes,
		"status":        "draft",
	}, nil
}

func (s *Service) normalizeReceivablePaymentRequest(ctx context.Context, receivable Record, req ReceivableReceiptCreateRequest) (Record, error) {
	if !statusIn(fmt.Sprint(receivable["status"]), "approved", "partially_received", "overdue") {
		return nil, validationError("Phải thu cần được duyệt trước khi tạo phiếu thu", Record{"status": receivable["status"]})
	}
	paymentCode := strings.TrimSpace(req.PaymentCode)
	if paymentCode == "" {
		return nil, validationError("Mã phiếu thu là bắt buộc")
	}
	if len([]rune(paymentCode)) > 80 {
		return nil, validationError("Mã phiếu thu không được vượt quá 80 ký tự")
	}
	if req.Amount <= 0 {
		return nil, validationError("Số tiền thu phải lớn hơn 0")
	}
	paymentMethod := strings.TrimSpace(req.PaymentMethod)
	if paymentMethod == "" {
		paymentMethod = "bank_transfer"
	}
	if !statusIn(paymentMethod, "bank_transfer", "cash") {
		return nil, validationError("Phương thức thu không hợp lệ", Record{"payment_method": paymentMethod})
	}
	defaultCashAccount := "1121"
	if paymentMethod == "cash" {
		defaultCashAccount = "1111"
	}
	accountingBookID, cashAccountID, err := s.resolveAccountingAccount(ctx, cleanID(receivable["project_id"]), req.CashAccountID, defaultCashAccount)
	if err != nil {
		return nil, err
	}
	advanceID := cleanID(req.AdvanceReceiptID)
	scheduleID := cleanID(req.ScheduleID)
	if advanceID != "" && scheduleID != "" {
		return nil, validationError("Chỉ được chọn một kỳ thu hoặc một ứng trước cho phiếu thu")
	}
	paymentType := strings.TrimSpace(req.PaymentType)
	if paymentType == "" {
		paymentType = "client_receipt"
	}
	if !statusIn(paymentType, "client_receipt", "advance", "advance_settlement", "inbound", "other") {
		return nil, validationError("Loại phiếu thu đầu ra không hợp lệ", Record{"payment_type": paymentType})
	}
	if advanceID != "" && !validActivityUUID(advanceID) {
		return nil, validationError("advance_receipt_id không hợp lệ", Record{"advance_receipt_id": advanceID})
	}
	if scheduleID != "" && !validActivityUUID(scheduleID) {
		return nil, validationError("schedule_id không hợp lệ", Record{"schedule_id": scheduleID})
	}
	outputInvoiceID := cleanID(req.OutputInvoiceID)
	if outputInvoiceID != "" {
		if !validActivityUUID(outputInvoiceID) {
			return nil, validationError("output_invoice_id không hợp lệ", Record{"output_invoice_id": outputInvoiceID})
		}
		invoice, err := s.repo.GetOutputInvoiceDetail(ctx, outputInvoiceID)
		if err != nil {
			return nil, validationError("Hoá đơn đầu ra liên kết không tồn tại", Record{"output_invoice_id": outputInvoiceID})
		}
		if cleanID(invoice["receivable_id"]) != cleanID(receivable["id"]) || fmt.Sprint(invoice["status"]) != "issued" {
			return nil, validationError("Hoá đơn đầu ra phải thuộc phải thu và đã phát hành", Record{"output_invoice_id": outputInvoiceID})
		}
	}
	if scheduleID != "" {
		schedule, err := s.repo.GetReceivablePaymentScheduleDetail(ctx, scheduleID)
		if err != nil {
			return nil, validationError("Kỳ thu không tồn tại", Record{"schedule_id": scheduleID})
		}
		if cleanID(schedule["receivable_id"]) != cleanID(receivable["id"]) || !statusIn(fmt.Sprint(schedule["status"]), "approved", "partially_received", "overdue") {
			return nil, validationError("Kỳ thu không thuộc phải thu hoặc chưa được duyệt", Record{"schedule_id": scheduleID})
		}
		if req.Amount > number(schedule["remaining_amount"]) {
			return nil, validationError("Số tiền thu vượt số còn lại của kỳ", Record{"amount": req.Amount, "schedule_remaining": schedule["remaining_amount"]})
		}
	}
	if advanceID != "" {
		advance, err := s.repo.GetReceivableAdvanceReceiptDetail(ctx, advanceID)
		if err != nil {
			return nil, validationError("Ứng trước đầu ra không tồn tại", Record{"advance_receipt_id": advanceID})
		}
		if cleanID(advance["receivable_id"]) != cleanID(receivable["id"]) || !statusIn(fmt.Sprint(advance["status"]), "approved", "partially_received", "received", "partially_settled") {
			return nil, validationError("Ứng trước không thuộc phải thu hoặc chưa được duyệt", Record{"advance_receipt_id": advanceID})
		}
		advanceRemaining := number(advance["amount"]) - number(advance["received_amount"])
		if req.Amount > advanceRemaining {
			return nil, validationError("Số tiền thu vượt số ứng trước chưa ghi nhận tiền", Record{"amount": req.Amount, "advance_remaining": advanceRemaining})
		}
	}
	if scheduleID == "" && advanceID == "" && req.Amount > number(receivable["remaining_amount"]) {
		return nil, validationError("Số tiền thu vượt số còn phải thu", Record{"amount": req.Amount, "remaining_amount": receivable["remaining_amount"]})
	}
	paidAt, err := normalizeOptionalDateTime(req.PaidAt, "paid_at")
	if err != nil {
		return nil, err
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, validationError("Ghi chú phiếu thu không được vượt quá 2000 ký tự")
	}
	allocationType := "receivable_receipt"
	if advanceID != "" {
		allocationType = "advance_receipt"
	}
	return Record{
		"project_id":         cleanID(receivable["project_id"]),
		"accounting_book_id": accountingBookID,
		"cash_account_id":    cashAccountID,
		"receivable_id":      cleanID(receivable["id"]),
		"payment_code":       paymentCode,
		"payment_type":       paymentType,
		"payment_method":     paymentMethod,
		"amount":             req.Amount,
		"schedule_id":        scheduleID,
		"advance_receipt_id": advanceID,
		"output_invoice_id":  outputInvoiceID,
		"paid_at":            paidAt,
		"notes":              notes,
		"allocation_type":    allocationType,
		"target_type":        "receivable",
		"target_id":          cleanID(receivable["id"]),
		"status":             "draft",
	}, nil
}

func (s *Service) normalizeReceivableAdvanceSettlementRequest(ctx context.Context, advance Record, req ReceivableAdvanceSettlementRequest) (Record, error) {
	if !statusIn(fmt.Sprint(advance["status"]), "approved", "partially_received", "received", "partially_settled") {
		return nil, validationError("Ứng trước đầu ra chưa sẵn sàng quyết toán", Record{"status": advance["status"]})
	}
	if req.Amount <= 0 {
		return nil, validationError("Số tiền quyết toán phải lớn hơn 0")
	}
	remaining := number(advance["amount"]) - number(advance["settled_amount"])
	if req.Amount > remaining {
		return nil, validationError("Số tiền quyết toán vượt số ứng trước còn lại", Record{"amount": req.Amount, "remaining": remaining})
	}
	outputInvoiceID := cleanID(req.OutputInvoiceID)
	if outputInvoiceID != "" {
		if !validActivityUUID(outputInvoiceID) {
			return nil, validationError("output_invoice_id không hợp lệ", Record{"output_invoice_id": outputInvoiceID})
		}
		invoice, err := s.repo.GetOutputInvoiceDetail(ctx, outputInvoiceID)
		if err != nil {
			return nil, validationError("Hoá đơn quyết toán không tồn tại", Record{"output_invoice_id": outputInvoiceID})
		}
		if cleanID(invoice["receivable_id"]) != cleanID(advance["receivable_id"]) || fmt.Sprint(invoice["status"]) != "issued" {
			return nil, validationError("Hoá đơn quyết toán phải thuộc phải thu và đã phát hành", Record{"output_invoice_id": outputInvoiceID})
		}
	}
	scheduleID := cleanID(req.ScheduleID)
	if scheduleID != "" {
		if !validActivityUUID(scheduleID) {
			return nil, validationError("schedule_id không hợp lệ", Record{"schedule_id": scheduleID})
		}
		schedule, err := s.repo.GetReceivablePaymentScheduleDetail(ctx, scheduleID)
		if err != nil {
			return nil, validationError("Kỳ thu quyết toán không tồn tại", Record{"schedule_id": scheduleID})
		}
		if cleanID(schedule["receivable_id"]) != cleanID(advance["receivable_id"]) {
			return nil, validationError("Kỳ thu quyết toán không thuộc phải thu", Record{"schedule_id": scheduleID})
		}
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 2000 {
		return nil, validationError("Ghi chú quyết toán không được vượt quá 2000 ký tự")
	}
	return Record{
		"amount":            req.Amount,
		"output_invoice_id": outputInvoiceID,
		"schedule_id":       scheduleID,
		"comment":           comment,
	}, nil
}
