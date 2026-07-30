package core

import (
	"context"
	"math"
	"strings"
	"time"
)

func (s *Service) ProjectFinanceWorkspace(ctx context.Context, projectID string, includeManagementReport bool) (Record, error) {
	summary, err := s.repo.ProjectFinanceSummary(ctx, projectID)
	if err != nil {
		return nil, err
	}
	tasks, err := s.repo.ProjectFinanceTasks(ctx, projectID)
	if err != nil {
		return nil, err
	}
	costs, err := s.repo.ProjectMaterialCosts(ctx, projectID)
	if err != nil {
		return nil, err
	}
	requests, err := s.repo.ProjectCashRequests(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if !includeManagementReport {
		for _, field := range []string{"contract_value", "eligible_revenue", "committed_material_cost", "confirmed_material_cost", "provisional_profit", "unconfirmed_cost_amount", "open_inbound_amount", "open_outbound_amount"} {
			summary[field] = nil
		}
	}
	return Record{"summary": summary, "tasks": tasks, "material_costs": costs, "cash_requests": requests, "can_view_report": includeManagementReport}, nil
}

func (s *Service) ProjectFinanceOptions(ctx context.Context, projectID string) (Record, error) {
	return s.repo.ProjectFinanceOptions(ctx, projectID)
}

func (s *Service) CreateProjectMaterialCost(ctx context.Context, projectID string, req ProjectMaterialCostRequest, actorID, actorName string) (Record, error) {
	req.VendorName = strings.TrimSpace(req.VendorName)
	req.InvoiceNumber = strings.TrimSpace(req.InvoiceNumber)
	req.InvoiceDate = strings.TrimSpace(req.InvoiceDate)
	req.Description = strings.TrimSpace(req.Description)
	req.Notes = strings.TrimSpace(req.Notes)
	if req.VendorName == "" || req.Description == "" {
		return nil, validationError("Cần nhập nhà cung cấp và nội dung chi phí", Record{"fields": []string{"vendor_name", "description"}})
	}
	if req.ProcurementRequestID == "" && req.IncomingAcceptanceID == "" && req.StockDocumentID == "" && req.DocumentID == "" {
		return nil, validationError("Cần liên kết ít nhất một hồ sơ nguồn của công trình")
	}
	if req.InvoiceDate != "" {
		if _, err := time.Parse("2006-01-02", req.InvoiceDate); err != nil {
			return nil, validationError("Ngày hóa đơn không hợp lệ", Record{"field": "invoice_date"})
		}
	}
	if req.AmountBeforeVAT < 0 || req.VATAmount < 0 || req.TotalAmount <= 0 {
		return nil, validationError("Giá trị chi phí phải lớn hơn 0 và không được âm")
	}
	if math.Abs((req.AmountBeforeVAT+req.VATAmount)-req.TotalAmount) > 0.009 {
		return nil, validationError("Tổng thanh toán phải bằng tiền trước VAT cộng VAT")
	}
	return s.repo.CreateProjectMaterialCost(ctx, projectID, req, actorID, actorName)
}

func (s *Service) CreateProjectCashRequest(ctx context.Context, projectID string, req ProjectCashRequestRequest, actorID, actorName string) (Record, error) {
	req.Direction = strings.TrimSpace(req.Direction)
	req.Title = strings.TrimSpace(req.Title)
	req.CounterpartyName = strings.TrimSpace(req.CounterpartyName)
	req.DueDate = strings.TrimSpace(req.DueDate)
	req.SourceType = strings.TrimSpace(req.SourceType)
	req.SourceID = strings.TrimSpace(req.SourceID)
	req.Notes = strings.TrimSpace(req.Notes)
	if req.Direction != "inbound" && req.Direction != "outbound" {
		return nil, validationError("Loại đề nghị phải là thu hoặc chi", Record{"field": "direction"})
	}
	if req.Title == "" || req.CounterpartyName == "" {
		return nil, validationError("Cần nhập nội dung và đối tượng thu/chi", Record{"fields": []string{"title", "counterparty_name"}})
	}
	if req.Amount <= 0 {
		return nil, validationError("Số tiền đề nghị phải lớn hơn 0", Record{"field": "amount"})
	}
	if (req.SourceType == "") != (req.SourceID == "") {
		return nil, validationError("Loại hồ sơ nguồn và mã hồ sơ nguồn phải được chọn cùng nhau", Record{"fields": []string{"source_type", "source_id"}})
	}
	if req.DueDate != "" {
		if _, err := time.Parse("2006-01-02", req.DueDate); err != nil {
			return nil, validationError("Hạn xử lý không hợp lệ", Record{"field": "due_date"})
		}
	}
	return s.repo.CreateProjectCashRequest(ctx, projectID, req, actorID, actorName)
}

func (s *Service) ProjectMaterialCostAction(ctx context.Context, id, action string, req ProjectFinanceActionRequest, actorID, actorName string) (Record, error) {
	req.Comment = strings.TrimSpace(req.Comment)
	return s.repo.ProjectMaterialCostAction(ctx, id, action, req, actorID, actorName)
}

func (s *Service) ProjectCashRequestAction(ctx context.Context, id, action string, req ProjectFinanceActionRequest, actorID, actorName string) (Record, error) {
	req.Comment = strings.TrimSpace(req.Comment)
	return s.repo.ProjectCashRequestAction(ctx, id, action, req, actorID, actorName)
}
