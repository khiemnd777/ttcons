package core

import (
	"context"
	"strings"
	"time"
)

func (s *Service) ProjectBudget(ctx context.Context, projectID string) ([]Record, error) {
	return s.repo.ListProjectBudget(ctx, cleanID(projectID))
}

func (s *Service) CreateBudget(ctx context.Context, projectID string, req BudgetCreateRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	budgetCode := strings.TrimSpace(req.BudgetCode)
	name := strings.TrimSpace(req.Name)
	if !validActivityUUID(projectID) || budgetCode == "" || name == "" || req.PlannedAmount <= 0 {
		return nil, validationError("Ngân sách cần mã, tên và giá trị kế hoạch lớn hơn 0", Record{"project_id": projectID, "budget_code": budgetCode, "planned_amount": req.PlannedAmount})
	}
	if len([]rune(budgetCode)) > 100 || len([]rune(name)) > 255 {
		return nil, validationError("Mã hoặc tên ngân sách vượt quá độ dài cho phép")
	}
	body := Record{
		"project_id": projectID, "budget_code": budgetCode, "name": name,
		"planned_amount": req.PlannedAmount, "approved_amount": 0, "used_amount": 0,
		"remaining_amount": 0, "status": "draft",
	}
	row, err := s.repo.CreateBudget(ctx, body, actorID, actorName)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *Service) ApproveBudget(ctx context.Context, id string, actorID, actorName string) (Record, error) {
	row, err := s.repo.ApproveBudget(ctx, cleanID(id), actorID, actorName)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *Service) CreatePayable(ctx context.Context, req PayableCreateRequest, actorID, actorName string) (Record, error) {
	projectID := cleanID(req.ProjectID)
	code := strings.TrimSpace(req.PayableCode)
	if !validActivityUUID(projectID) || code == "" || req.GrossAmount <= 0 {
		return nil, validationError("Phải trả cần công trình, mã và giá trị lớn hơn 0", Record{"project_id": projectID, "payable_code": code, "gross_amount": req.GrossAmount})
	}
	if req.AdvanceAmount < 0 || req.RetentionAmount < 0 || req.DeductionAmount < 0 || req.AdvanceAmount+req.DeductionAmount > req.GrossAmount {
		return nil, validationError("Giá trị tạm ứng/giữ lại/khấu trừ không hợp lệ")
	}
	dueDate, err := normalizeOptionalDate(req.DueDate, "due_date")
	if err != nil {
		return nil, err
	}
	vendorType := strings.TrimSpace(req.VendorType)
	if vendorType == "" {
		vendorType = "supplier"
	}
	if !statusIn(vendorType, vendorPartnerTypes()...) {
		return nil, validationError("Loại vendor không hợp lệ", Record{"vendor_type": vendorType})
	}
	accountingBookID, costAccountID, err := s.resolveAccountingAccount(ctx, projectID, req.CostAccountID, "154")
	if err != nil {
		return nil, err
	}
	budgetID, err := s.validateAccountingBudget(ctx, projectID, req.BudgetID)
	if err != nil {
		return nil, err
	}
	body := Record{
		"project_id": projectID, "accounting_book_id": accountingBookID, "budget_id": budgetID, "cost_account_id": costAccountID,
		"payable_code": code, "vendor_type": vendorType,
		"vendor_partner_id": cleanID(req.VendorPartnerID), "vendor_name": strings.TrimSpace(req.VendorName),
		"source_type": "manual", "gross_amount": req.GrossAmount, "advance_amount": req.AdvanceAmount,
		"retention_amount": req.RetentionAmount, "deduction_amount": req.DeductionAmount,
		"paid_amount": 0, "remaining_amount": req.GrossAmount - req.AdvanceAmount - req.DeductionAmount,
		"due_date": dueDate, "status": "draft", "created_by": actorID,
	}
	if err := s.applyPartnerSnapshot(ctx, body, "vendor_partner_id", "vendor_name", "", vendorPartnerTypes()); err != nil {
		return nil, err
	}
	row, err := s.repo.insertWithAudit(ctx, "payables", payableColumns(), body, func(row Record) auditEvent {
		return auditEvent{
			ProjectID: projectID, ActorID: actorID, ActorName: actorName,
			EventType: "payable_created", TargetType: "payable", Description: "Tạo phải trả ở trạng thái nháp",
			Metadata: Record{"payable_code": code, "gross_amount": req.GrossAmount, "remaining_amount": row["remaining_amount"]},
		}
	})
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *Service) CreateReceivable(ctx context.Context, req ReceivableCreateRequest, actorID, actorName string) (Record, error) {
	projectID := cleanID(req.ProjectID)
	code := strings.TrimSpace(req.ReceivableCode)
	if !validActivityUUID(projectID) || code == "" || req.GrossAmount <= 0 {
		return nil, validationError("Phải thu cần công trình, mã và giá trị lớn hơn 0", Record{"project_id": projectID, "receivable_code": code, "gross_amount": req.GrossAmount})
	}
	dueDate, err := normalizeOptionalDate(req.DueDate, "due_date")
	if err != nil {
		return nil, err
	}
	accountingBookID, revenueAccountID, err := s.resolveAccountingAccount(ctx, projectID, req.RevenueAccountID, "5111")
	if err != nil {
		return nil, err
	}
	body := Record{
		"project_id": projectID, "accounting_book_id": accountingBookID, "revenue_account_id": revenueAccountID, "receivable_code": code,
		"client_partner_id": cleanID(req.ClientPartnerID), "client_name": strings.TrimSpace(req.ClientName),
		"source_type": "manual", "contract_amount": req.ContractAmount,
		"milestone_name": strings.TrimSpace(req.MilestoneName), "gross_amount": req.GrossAmount,
		"received_amount": 0, "remaining_amount": req.GrossAmount, "due_date": dueDate,
		"status": "draft", "created_by": actorID,
	}
	if err := s.applyPartnerSnapshot(ctx, body, "client_partner_id", "client_name", "", []string{"client"}); err != nil {
		return nil, err
	}
	row, err := s.repo.insertWithAudit(ctx, "receivables", receivableColumns(), body, func(row Record) auditEvent {
		return auditEvent{
			ProjectID: projectID, ActorID: actorID, ActorName: actorName,
			EventType: "receivable_created", TargetType: "receivable", Description: "Tạo phải thu ở trạng thái nháp",
			Metadata: Record{"receivable_code": code, "gross_amount": req.GrossAmount, "remaining_amount": row["remaining_amount"]},
		}
	})
	if err != nil {
		return nil, err
	}
	return row, nil
}

func normalizeOptionalDate(value, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return "", validationError("Ngày không hợp lệ", Record{field: value})
	}
	return value, nil
}

func normalizeOptionalDateTime(value, field string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return value, nil
	}
	if _, err := time.Parse("2006-01-02", value); err == nil {
		return value, nil
	}
	return "", validationError("Thời điểm không hợp lệ", Record{field: value})
}

func (s *Service) PayableAction(ctx context.Context, id, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.PayableAction(ctx, id, action, req, actorID, actorName)
}

func (s *Service) ReceivableAction(ctx context.Context, id, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.ReceivableAction(ctx, id, action, req, actorID, actorName)
}

func (s *Service) CreatePayment(ctx context.Context, req PaymentCreateRequest, actorID, actorName string) (Record, error) {
	projectID := cleanID(req.ProjectID)
	paymentCode := strings.TrimSpace(req.PaymentCode)
	paymentType := strings.TrimSpace(req.PaymentType)
	paymentMethod := strings.TrimSpace(req.PaymentMethod)
	amount := req.Amount
	if !validActivityUUID(projectID) || paymentCode == "" || amount <= 0 {
		return nil, validationError("Phiếu thu/chi cần công trình, mã và số tiền lớn hơn 0", Record{"project_id": projectID, "payment_code": paymentCode, "amount": amount})
	}
	if !statusIn(paymentType, "outbound", "inbound") {
		return nil, validationError("Chỉ được lập Phiếu chi khác hoặc Phiếu thu khác tại đây; chứng từ gắn công nợ phải lập tại Phải trả/Phải thu", Record{"payment_type": paymentType, "allowed_types": []string{"outbound", "inbound"}})
	}
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
	accountingBookID, cashAccountID, err := s.resolveAccountingAccount(ctx, projectID, req.CashAccountID, defaultCashAccount)
	if err != nil {
		return nil, err
	}
	budgetID, err := s.validateAccountingBudget(ctx, projectID, req.BudgetID)
	if err != nil {
		return nil, err
	}
	if paymentType == "outbound" && budgetID == "" {
		return nil, validationError("Phiếu chi cần chọn ngân sách để ghi nhận thực chi")
	}
	targetType := strings.TrimSpace(req.TargetType)
	targetID := cleanID(req.TargetID)
	if (targetType == "") != (targetID == "") || (targetID != "" && !validActivityUUID(targetID)) {
		return nil, validationError("Nguồn liên kết phiếu thu/chi không hợp lệ", Record{"target_type": targetType, "target_id": targetID})
	}
	normalized := Record{
		"project_id": projectID, "accounting_book_id": accountingBookID, "budget_id": budgetID, "cash_account_id": cashAccountID,
		"payment_code": paymentCode, "payment_type": paymentType,
		"target_type": targetType, "target_id": targetID,
		"amount": amount, "payment_method": paymentMethod, "status": "draft", "created_by": actorID,
	}
	row, err := s.repo.CreatePayment(ctx, normalized, actorID, actorName)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *Service) PaymentAction(ctx context.Context, id, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.PaymentAction(ctx, id, action, req, actorID, actorName)
}
