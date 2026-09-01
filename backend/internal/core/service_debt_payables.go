package core

import (
	"context"
	"fmt"
	"mime/multipart"
	"strings"
)

func (s *Service) PayablePaymentSchedules(ctx context.Context, payableID, actorID string) ([]Record, error) {
	payable, err := s.requirePayableAccess(ctx, payableID, actorID, []string{"accounting.view", "accounting.payable_view"}, "Bạn không có quyền xem lịch thanh toán đầu vào")
	if err != nil {
		return nil, err
	}
	return s.repo.ListPayablePaymentSchedules(ctx, cleanID(payable["id"]))
}

func (s *Service) CreatePayablePaymentSchedule(ctx context.Context, payableID string, req PayablePaymentScheduleRequest, actorID, actorName string) (Record, error) {
	payable, err := s.requirePayableAccess(ctx, payableID, actorID, []string{"accounting.payment_create", "accounting.payable_approve"}, "Bạn không có quyền tạo kỳ thanh toán đầu vào")
	if err != nil {
		return nil, err
	}
	body, err := s.normalizePayablePaymentScheduleRequest(payable, req)
	if err != nil {
		return nil, err
	}
	return s.repo.CreatePayablePaymentSchedule(ctx, payable, body, actorID, actorName)
}

func (s *Service) PayablePaymentScheduleAction(ctx context.Context, scheduleID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	scheduleID = cleanID(scheduleID)
	if !validActivityUUID(scheduleID) {
		return nil, validationError("schedule_id không hợp lệ", Record{"schedule_id": scheduleID})
	}
	return s.repo.PayablePaymentScheduleAction(ctx, scheduleID, action, req, actorID, actorName)
}

func (s *Service) PayableAdvanceRequests(ctx context.Context, payableID, actorID string) ([]Record, error) {
	payable, err := s.requirePayableAccess(ctx, payableID, actorID, []string{"accounting.view", "accounting.payable_view"}, "Bạn không có quyền xem ứng trước đầu vào")
	if err != nil {
		return nil, err
	}
	return s.repo.ListPayableAdvanceRequests(ctx, cleanID(payable["id"]))
}

func (s *Service) CreatePayableAdvanceRequest(ctx context.Context, payableID string, req AdvanceRequestCreateRequest, actorID, actorName string) (Record, error) {
	payable, err := s.requirePayableAccess(ctx, payableID, actorID, []string{"accounting.payment_create", "accounting.payable_approve"}, "Bạn không có quyền tạo ứng trước đầu vào")
	if err != nil {
		return nil, err
	}
	body, err := s.normalizePayableAdvanceRequest(payable, req)
	if err != nil {
		return nil, err
	}
	return s.repo.CreatePayableAdvanceRequest(ctx, payable, body, actorID, actorName)
}

func (s *Service) PayableAdvanceRequestAction(ctx context.Context, advanceID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	advanceID = cleanID(advanceID)
	if !validActivityUUID(advanceID) {
		return nil, validationError("advance_request_id không hợp lệ", Record{"advance_request_id": advanceID})
	}
	return s.repo.PayableAdvanceRequestAction(ctx, advanceID, action, req, actorID, actorName)
}

func (s *Service) SettlePayableAdvanceRequest(ctx context.Context, advanceID string, req AdvanceSettlementRequest, actorID, actorName string) (Record, error) {
	advanceID = cleanID(advanceID)
	if !validActivityUUID(advanceID) {
		return nil, validationError("advance_request_id không hợp lệ", Record{"advance_request_id": advanceID})
	}
	advance, err := s.repo.GetPayableAdvanceRequestDetail(ctx, advanceID)
	if err != nil {
		return nil, err
	}
	body, err := s.normalizeAdvanceSettlementRequest(ctx, advance, req)
	if err != nil {
		return nil, err
	}
	return s.repo.SettlePayableAdvanceRequest(ctx, advanceID, body, actorID, actorName)
}

func (s *Service) PayablePayments(ctx context.Context, payableID, actorID string) ([]Record, error) {
	payable, err := s.requirePayableAccess(ctx, payableID, actorID, []string{"accounting.view", "accounting.payable_view", "accounting.payment_create", "accounting.payment_approve"}, "Bạn không có quyền xem thanh toán đầu vào")
	if err != nil {
		return nil, err
	}
	return s.repo.ListPayablePayments(ctx, cleanID(payable["id"]))
}

func (s *Service) CreatePayablePayment(ctx context.Context, payableID string, req PayablePaymentCreateRequest, actorID, actorName string) (Record, error) {
	payable, err := s.requirePayableAccess(ctx, payableID, actorID, []string{"accounting.payment_create"}, "Bạn không có quyền tạo thanh toán đầu vào")
	if err != nil {
		return nil, err
	}
	body, err := s.normalizePayablePaymentRequest(ctx, payable, req)
	if err != nil {
		return nil, err
	}
	return s.repo.CreatePayablePayment(ctx, payable, body, actorID, actorName)
}

func (s *Service) PayableInputInvoices(ctx context.Context, payableID, actorID string) ([]Record, error) {
	payable, err := s.requirePayableAccess(ctx, payableID, actorID, []string{"accounting.view", "accounting.payable_view"}, "Bạn không có quyền xem hoá đơn đầu vào")
	if err != nil {
		return nil, err
	}
	return s.repo.ListInputInvoicesForPayable(ctx, cleanID(payable["id"]))
}

func (s *Service) PayableReconciliation(ctx context.Context, payableID, actorID string) (Record, error) {
	payable, err := s.requirePayableAccess(ctx, payableID, actorID, []string{"accounting.view", "accounting.payable_view"}, "Bạn không có quyền xem đối soát công nợ phải trả")
	if err != nil {
		return nil, err
	}
	return s.repo.PayableReconciliation(ctx, cleanID(payable["id"]))
}

func (s *Service) ProjectInputInvoices(ctx context.Context, projectID, actorID string) ([]Record, error) {
	projectID = cleanID(projectID)
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"accounting.view", "accounting.payable_view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem hoá đơn đầu vào", Record{"permissions": []string{"accounting.view", "accounting.payable_view"}})
	}
	return s.repo.ListInputInvoicesForProject(ctx, projectID)
}

func (s *Service) CreateInputInvoice(ctx context.Context, payableID string, req InputInvoiceRequest, fileHeader *multipart.FileHeader, actorID, actorName string) (Record, error) {
	payable, err := s.requirePayableAccess(ctx, payableID, actorID, []string{"accounting.payable_approve"}, "Bạn không có quyền upload hoá đơn đầu vào")
	if err != nil {
		return nil, err
	}
	body, err := s.normalizeInputInvoiceRequest(ctx, payable, req)
	if err != nil {
		return nil, err
	}
	fileMeta, storedObject, originalName, err := s.storeInputInvoicePDF(ctx, cleanID(payable["project_id"]), cleanID(payable["id"]), fileHeader)
	if err != nil {
		return nil, err
	}
	body["file_name"] = originalName
	record, err := s.repo.CreateInputInvoice(ctx, payable, body, fileMeta, actorID, actorName)
	if err != nil {
		return nil, s.cleanupStoredObjectAfterError(storedObject, err)
	}
	return record, nil
}

func (s *Service) InputInvoiceAction(ctx context.Context, invoiceID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	invoice, err := s.repo.GetInputInvoiceDetail(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(invoice["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"accounting.payable_approve"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền cập nhật hoá đơn đầu vào", Record{"permissions": []string{"accounting.payable_approve"}})
	}
	return s.repo.InputInvoiceAction(ctx, cleanID(invoice["id"]), action, req, actorID, actorName)
}

func (s *Service) InputInvoiceFile(ctx context.Context, invoiceID, actorID string) (EvidenceFileResult, error) {
	invoiceID = cleanID(invoiceID)
	if !validActivityUUID(invoiceID) {
		return EvidenceFileResult{}, validationError("invoice_id không hợp lệ", Record{"invoice_id": invoiceID})
	}
	invoice, err := s.repo.GetInputInvoiceDetail(ctx, invoiceID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	projectID := cleanID(invoice["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"accounting.view", "accounting.payable_view"})
	if err != nil {
		return EvidenceFileResult{}, err
	}
	if !allowed {
		return EvidenceFileResult{}, forbiddenError("Bạn không có quyền xem file hoá đơn đầu vào")
	}
	if fmt.Sprint(invoice["status"]) == "deleted" {
		return EvidenceFileResult{}, validationError("Hoá đơn đầu vào đã bị xoá", Record{"invoice_id": invoiceID})
	}
	return s.storedFileResult(ctx, invoice, "storage_provider", "storage_key", "mime_type", fmt.Sprint(invoice["file_name"]))
}

func (s *Service) requirePayableAccess(ctx context.Context, payableID, actorID string, permissions []string, message string) (Record, error) {
	payableID = cleanID(payableID)
	if !validActivityUUID(payableID) {
		return nil, validationError("payable_id không hợp lệ", Record{"payable_id": payableID})
	}
	payable, err := s.repo.GetPayableDetail(ctx, payableID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(payable["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, permissions)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError(message, Record{"permissions": permissions})
	}
	return payable, nil
}

func (s *Service) normalizePayablePaymentScheduleRequest(payable Record, req PayablePaymentScheduleRequest) (Record, error) {
	scheduleCode := strings.TrimSpace(req.ScheduleCode)
	if scheduleCode == "" {
		return nil, validationError("Mã kỳ thanh toán là bắt buộc")
	}
	if len([]rune(scheduleCode)) > 80 {
		return nil, validationError("Mã kỳ thanh toán không được vượt quá 80 ký tự")
	}
	milestoneName := strings.TrimSpace(req.MilestoneName)
	if milestoneName == "" {
		return nil, validationError("Tên kỳ thanh toán là bắt buộc")
	}
	if len([]rune(milestoneName)) > 180 {
		return nil, validationError("Tên kỳ thanh toán không được vượt quá 180 ký tự")
	}
	scheduleType := strings.TrimSpace(req.ScheduleType)
	if scheduleType == "" {
		scheduleType = "progress"
	}
	if !statusIn(scheduleType, "advance", "progress", "final", "retention", "other") {
		return nil, validationError("Loại kỳ thanh toán không hợp lệ", Record{"schedule_type": scheduleType})
	}
	if req.PlannedAmount <= 0 {
		return nil, validationError("Số tiền kế hoạch phải lớn hơn 0")
	}
	if number(payable["gross_amount"]) > 0 && req.PlannedAmount > number(payable["gross_amount"]) {
		return nil, validationError("Số tiền kỳ thanh toán vượt giá trị phải trả", Record{"planned_amount": req.PlannedAmount})
	}
	dueDate, err := normalizeOptionalDate(req.DueDate, "due_date")
	if err != nil {
		return nil, err
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, validationError("Ghi chú kỳ thanh toán không được vượt quá 2000 ký tự")
	}
	return Record{
		"project_id":        cleanID(payable["project_id"]),
		"payable_id":        cleanID(payable["id"]),
		"schedule_code":     scheduleCode,
		"milestone_name":    milestoneName,
		"schedule_type":     scheduleType,
		"planned_amount":    req.PlannedAmount,
		"paid_amount":       0,
		"remaining_amount":  req.PlannedAmount,
		"due_date":          dueDate,
		"status":            "draft",
		"notes":             notes,
		"payable_remaining": number(payable["remaining_amount"]),
	}, nil
}

func (s *Service) normalizePayableAdvanceRequest(payable Record, req AdvanceRequestCreateRequest) (Record, error) {
	requestCode := strings.TrimSpace(req.RequestCode)
	if requestCode == "" {
		return nil, validationError("Mã ứng trước là bắt buộc")
	}
	if len([]rune(requestCode)) > 80 {
		return nil, validationError("Mã ứng trước không được vượt quá 80 ký tự")
	}
	if req.Amount <= 0 {
		return nil, validationError("Số tiền ứng trước phải lớn hơn 0")
	}
	remaining := number(payable["remaining_amount"])
	if remaining > 0 && req.Amount > remaining {
		return nil, validationError("Số tiền ứng trước vượt số còn phải trả", Record{"amount": req.Amount, "remaining_amount": remaining})
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
		"project_id":   cleanID(payable["project_id"]),
		"payable_id":   cleanID(payable["id"]),
		"request_code": requestCode,
		"vendor_name":  cleanOptionalText(payable["vendor_name"]),
		"amount":       req.Amount,
		"due_date":     dueDate,
		"reason":       reason,
		"notes":        notes,
		"status":       "draft",
	}, nil
}

func (s *Service) normalizePayablePaymentRequest(ctx context.Context, payable Record, req PayablePaymentCreateRequest) (Record, error) {
	if !statusIn(fmt.Sprint(payable["status"]), "approved", "partially_paid", "overdue") {
		return nil, validationError("Phải trả cần được duyệt trước khi tạo thanh toán", Record{"status": payable["status"]})
	}
	paymentCode := strings.TrimSpace(req.PaymentCode)
	if paymentCode == "" {
		return nil, validationError("Mã thanh toán là bắt buộc")
	}
	if len([]rune(paymentCode)) > 80 {
		return nil, validationError("Mã thanh toán không được vượt quá 80 ký tự")
	}
	if req.Amount <= 0 {
		return nil, validationError("Số tiền thanh toán phải lớn hơn 0")
	}
	paymentMethod := strings.TrimSpace(req.PaymentMethod)
	if paymentMethod == "" {
		paymentMethod = "bank_transfer"
	}
	if !statusIn(paymentMethod, "bank_transfer", "cash") {
		return nil, validationError("Phương thức thanh toán không hợp lệ", Record{"payment_method": paymentMethod})
	}
	defaultCashAccount := "1121"
	if paymentMethod == "cash" {
		defaultCashAccount = "1111"
	}
	accountingBookID, cashAccountID, err := s.resolveAccountingAccount(ctx, cleanID(payable["project_id"]), req.CashAccountID, defaultCashAccount)
	if err != nil {
		return nil, err
	}
	advanceID := cleanID(req.AdvanceRequestID)
	scheduleID := cleanID(req.ScheduleID)
	if advanceID != "" && scheduleID != "" {
		return nil, validationError("Chỉ được chọn một kỳ thanh toán hoặc một ứng trước cho phiếu thanh toán")
	}
	paymentType := strings.TrimSpace(req.PaymentType)
	if paymentType == "" {
		if advanceID != "" {
			paymentType = "advance"
		} else if statusIn(fmt.Sprint(payable["vendor_type"]), "subcontractor", "labor_team") {
			paymentType = "subcontractor_payment"
		} else {
			paymentType = "supplier_payment"
		}
	}
	if !statusIn(paymentType, "supplier_payment", "subcontractor_payment", "advance", "advance_settlement", "other") {
		return nil, validationError("Loại thanh toán đầu vào không hợp lệ", Record{"payment_type": paymentType})
	}
	if advanceID != "" && !validActivityUUID(advanceID) {
		return nil, validationError("advance_request_id không hợp lệ", Record{"advance_request_id": advanceID})
	}
	if scheduleID != "" && !validActivityUUID(scheduleID) {
		return nil, validationError("schedule_id không hợp lệ", Record{"schedule_id": scheduleID})
	}
	inputInvoiceID := cleanID(req.InputInvoiceID)
	if inputInvoiceID != "" {
		if !validActivityUUID(inputInvoiceID) {
			return nil, validationError("input_invoice_id không hợp lệ", Record{"input_invoice_id": inputInvoiceID})
		}
		invoice, err := s.repo.GetInputInvoiceDetail(ctx, inputInvoiceID)
		if err != nil {
			return nil, validationError("Hoá đơn đầu vào liên kết không tồn tại", Record{"input_invoice_id": inputInvoiceID})
		}
		if cleanID(invoice["payable_id"]) != cleanID(payable["id"]) || fmt.Sprint(invoice["status"]) != "verified" {
			return nil, validationError("Hoá đơn đầu vào phải thuộc phải trả và đã xác thực", Record{"input_invoice_id": inputInvoiceID})
		}
	}
	if scheduleID != "" {
		schedule, err := s.repo.GetPayablePaymentScheduleDetail(ctx, scheduleID)
		if err != nil {
			return nil, validationError("Kỳ thanh toán không tồn tại", Record{"schedule_id": scheduleID})
		}
		if cleanID(schedule["payable_id"]) != cleanID(payable["id"]) || !statusIn(fmt.Sprint(schedule["status"]), "approved", "partially_paid", "overdue") {
			return nil, validationError("Kỳ thanh toán không thuộc phải trả hoặc chưa được duyệt", Record{"schedule_id": scheduleID})
		}
		if req.Amount > number(schedule["remaining_amount"]) {
			return nil, validationError("Số tiền thanh toán vượt số còn lại của kỳ", Record{"amount": req.Amount, "schedule_remaining": schedule["remaining_amount"]})
		}
	}
	if advanceID != "" {
		advance, err := s.repo.GetPayableAdvanceRequestDetail(ctx, advanceID)
		if err != nil {
			return nil, validationError("Ứng trước không tồn tại", Record{"advance_request_id": advanceID})
		}
		if cleanID(advance["payable_id"]) != cleanID(payable["id"]) || !statusIn(fmt.Sprint(advance["status"]), "approved", "partially_settled") {
			return nil, validationError("Ứng trước không thuộc phải trả hoặc chưa được duyệt", Record{"advance_request_id": advanceID})
		}
		advanceRemaining := number(advance["amount"]) - number(advance["paid_amount"])
		if req.Amount > advanceRemaining {
			return nil, validationError("Số tiền thanh toán vượt số ứng trước chưa chi", Record{"amount": req.Amount, "advance_remaining": advanceRemaining})
		}
	}
	if scheduleID == "" && advanceID == "" && req.Amount > number(payable["remaining_amount"]) {
		return nil, validationError("Số tiền thanh toán vượt số còn phải trả", Record{"amount": req.Amount, "remaining_amount": payable["remaining_amount"]})
	}
	paidAt, err := normalizeOptionalDateTime(req.PaidAt, "paid_at")
	if err != nil {
		return nil, err
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, validationError("Ghi chú thanh toán không được vượt quá 2000 ký tự")
	}
	allocationType := "payable_payment"
	if advanceID != "" {
		allocationType = "advance_payment"
	}
	return Record{
		"project_id":          cleanID(payable["project_id"]),
		"accounting_book_id":  accountingBookID,
		"budget_id":           cleanID(payable["budget_id"]),
		"cash_account_id":     cashAccountID,
		"payable_id":          cleanID(payable["id"]),
		"payment_code":        paymentCode,
		"payment_type":        paymentType,
		"payment_method":      paymentMethod,
		"amount":              req.Amount,
		"schedule_id":         scheduleID,
		"advance_request_id":  advanceID,
		"input_invoice_id":    inputInvoiceID,
		"paid_at":             paidAt,
		"notes":               notes,
		"allocation_type":     allocationType,
		"target_type":         "payable",
		"target_id":           cleanID(payable["id"]),
		"status":              "draft",
		"payable_vendor_type": fmt.Sprint(payable["vendor_type"]),
	}, nil
}

func (s *Service) normalizeAdvanceSettlementRequest(ctx context.Context, advance Record, req AdvanceSettlementRequest) (Record, error) {
	if !statusIn(fmt.Sprint(advance["status"]), "approved", "partially_settled") {
		return nil, validationError("Ứng trước chưa sẵn sàng quyết toán", Record{"status": advance["status"]})
	}
	if req.Amount <= 0 {
		return nil, validationError("Số tiền quyết toán phải lớn hơn 0")
	}
	remaining := number(advance["amount"]) - number(advance["settled_amount"])
	if req.Amount > remaining {
		return nil, validationError("Số tiền quyết toán vượt số ứng trước còn lại", Record{"amount": req.Amount, "remaining": remaining})
	}
	inputInvoiceID := cleanID(req.InputInvoiceID)
	if inputInvoiceID != "" {
		if !validActivityUUID(inputInvoiceID) {
			return nil, validationError("input_invoice_id không hợp lệ", Record{"input_invoice_id": inputInvoiceID})
		}
		invoice, err := s.repo.GetInputInvoiceDetail(ctx, inputInvoiceID)
		if err != nil {
			return nil, validationError("Hoá đơn quyết toán không tồn tại", Record{"input_invoice_id": inputInvoiceID})
		}
		if cleanID(invoice["payable_id"]) != cleanID(advance["payable_id"]) || fmt.Sprint(invoice["status"]) != "verified" {
			return nil, validationError("Hoá đơn quyết toán phải thuộc phải trả và đã xác thực", Record{"input_invoice_id": inputInvoiceID})
		}
	}
	scheduleID := cleanID(req.ScheduleID)
	if scheduleID != "" {
		if !validActivityUUID(scheduleID) {
			return nil, validationError("schedule_id không hợp lệ", Record{"schedule_id": scheduleID})
		}
		schedule, err := s.repo.GetPayablePaymentScheduleDetail(ctx, scheduleID)
		if err != nil {
			return nil, validationError("Kỳ thanh toán quyết toán không tồn tại", Record{"schedule_id": scheduleID})
		}
		if cleanID(schedule["payable_id"]) != cleanID(advance["payable_id"]) {
			return nil, validationError("Kỳ thanh toán quyết toán không thuộc phải trả", Record{"schedule_id": scheduleID})
		}
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 2000 {
		return nil, validationError("Ghi chú quyết toán không được vượt quá 2000 ký tự")
	}
	return Record{
		"amount":           req.Amount,
		"input_invoice_id": inputInvoiceID,
		"schedule_id":      scheduleID,
		"comment":          comment,
	}, nil
}
