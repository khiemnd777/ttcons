package core

import (
	"context"
	"fmt"
	"mime/multipart"
	"strings"
	"time"
)

func (s *Service) normalizeInputInvoiceRequest(ctx context.Context, payable Record, req InputInvoiceRequest) (Record, error) {
	projectID := cleanID(payable["project_id"])
	invoiceNumber := strings.TrimSpace(req.InvoiceNumber)
	if invoiceNumber == "" {
		return nil, validationError("Số hoá đơn là bắt buộc")
	}
	if len([]rune(invoiceNumber)) > 120 {
		return nil, validationError("Số hoá đơn không được vượt quá 120 ký tự")
	}
	invoiceSymbol := strings.TrimSpace(req.InvoiceSymbol)
	if len([]rune(invoiceSymbol)) > 80 {
		return nil, validationError("Ký hiệu hoá đơn không được vượt quá 80 ký tự")
	}
	invoiceDate := strings.TrimSpace(req.InvoiceDate)
	if invoiceDate == "" {
		return nil, validationError("Ngày hoá đơn là bắt buộc")
	}
	if _, err := time.Parse("2006-01-02", invoiceDate); err != nil {
		return nil, validationError("Ngày hoá đơn không hợp lệ", Record{"invoice_date": invoiceDate})
	}
	vendorPartnerID := cleanID(req.VendorPartnerID)
	if vendorPartnerID == "" {
		vendorPartnerID = cleanID(payable["vendor_partner_id"])
	}
	vendorName := strings.TrimSpace(req.VendorName)
	vendorTaxCode := strings.TrimSpace(req.VendorTaxCode)
	if vendorPartnerID != "" {
		partner, err := s.requireActivePartner(ctx, vendorPartnerID, vendorPartnerTypes())
		if err != nil {
			return nil, err
		}
		vendorName = cleanOptionalText(partner["name"])
		vendorTaxCode = cleanOptionalText(partner["tax_code"])
	}
	if vendorName == "" {
		vendorName = cleanOptionalText(payable["vendor_name"])
	}
	if vendorName == "" {
		return nil, validationError("Tên nhà cung cấp trên hoá đơn là bắt buộc")
	}
	if len([]rune(vendorName)) > 240 {
		return nil, validationError("Tên nhà cung cấp trên hoá đơn không được vượt quá 240 ký tự")
	}
	if len([]rune(vendorTaxCode)) > 80 {
		return nil, validationError("Mã số thuế nhà cung cấp không được vượt quá 80 ký tự")
	}
	if req.AmountBeforeVAT < 0 || req.VATAmount < 0 || req.TotalAmount < 0 {
		return nil, validationError("Số tiền hoá đơn không được âm")
	}
	totalAmount := req.TotalAmount
	if totalAmount == 0 && req.AmountBeforeVAT+req.VATAmount > 0 {
		totalAmount = req.AmountBeforeVAT + req.VATAmount
	}
	if totalAmount <= 0 {
		return nil, validationError("Tổng tiền hoá đơn phải lớn hơn 0")
	}
	accountingBookID, expenseAccountID, err := s.resolveAccountingAccount(ctx, projectID, firstNonEmpty(req.ExpenseAccountID, cleanID(payable["cost_account_id"])), "154")
	if err != nil {
		return nil, err
	}
	taxCodeID, err := s.resolveInvoiceTaxCode(ctx, projectID, req.TaxCodeID, "input", req.AmountBeforeVAT, req.VATAmount)
	if err != nil {
		return nil, err
	}
	if req.AmountBeforeVAT+req.VATAmount > 0 && totalAmount+1 < req.AmountBeforeVAT+req.VATAmount {
		return nil, validationError("Tổng tiền hoá đơn nhỏ hơn trước VAT cộng VAT", Record{
			"amount_before_vat": req.AmountBeforeVAT,
			"vat_amount":        req.VATAmount,
			"total_amount":      totalAmount,
		})
	}
	procurementRequestID := cleanID(req.ProcurementRequestID)
	if procurementRequestID != "" {
		if !validActivityUUID(procurementRequestID) {
			return nil, validationError("procurement_request_id không hợp lệ", Record{"procurement_request_id": procurementRequestID})
		}
		procurement, err := s.repo.GetByID(ctx, "procurement_requests", procurementRequestID)
		if err != nil {
			return nil, validationError("Yêu cầu mua liên kết hoá đơn không tồn tại", Record{"procurement_request_id": procurementRequestID})
		}
		if cleanID(procurement["project_id"]) != projectID || fmt.Sprint(procurement["status"]) == "cancelled" {
			return nil, validationError("Yêu cầu mua liên kết hoá đơn không thuộc công trình", Record{"procurement_request_id": procurementRequestID})
		}
	}
	documentID := cleanID(req.DocumentID)
	if documentID != "" {
		if !validActivityUUID(documentID) {
			return nil, validationError("document_id không hợp lệ", Record{"document_id": documentID})
		}
		document, err := s.repo.GetByID(ctx, "documents", documentID)
		if err != nil {
			return nil, validationError("Tài liệu liên kết hoá đơn không tồn tại", Record{"document_id": documentID})
		}
		if cleanID(document["project_id"]) != projectID || fmt.Sprint(document["status"]) != "active" {
			return nil, validationError("Tài liệu liên kết hoá đơn không thuộc công trình", Record{"document_id": documentID})
		}
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, validationError("Ghi chú hoá đơn không được vượt quá 2000 ký tự")
	}
	return Record{
		"project_id":             projectID,
		"accounting_book_id":     accountingBookID,
		"tax_code_id":            taxCodeID,
		"expense_account_id":     expenseAccountID,
		"payable_id":             cleanID(payable["id"]),
		"procurement_request_id": procurementRequestID,
		"document_id":            documentID,
		"invoice_number":         invoiceNumber,
		"invoice_symbol":         invoiceSymbol,
		"invoice_date":           invoiceDate,
		"vendor_partner_id":      vendorPartnerID,
		"vendor_name":            vendorName,
		"vendor_tax_code":        vendorTaxCode,
		"amount_before_vat":      req.AmountBeforeVAT,
		"vat_amount":             req.VATAmount,
		"total_amount":           totalAmount,
		"notes":                  notes,
	}, nil
}

func (s *Service) ReceivableOutputInvoices(ctx context.Context, receivableID, actorID string) ([]Record, error) {
	receivable, err := s.requireReceivableAccess(ctx, receivableID, actorID, []string{"accounting.view", "accounting.receivable_view"}, "Bạn không có quyền xem hoá đơn đầu ra")
	if err != nil {
		return nil, err
	}
	return s.repo.ListOutputInvoicesForReceivable(ctx, cleanID(receivable["id"]))
}

func (s *Service) ReceivableReconciliation(ctx context.Context, receivableID, actorID string) (Record, error) {
	receivable, err := s.requireReceivableAccess(ctx, receivableID, actorID, []string{"accounting.view", "accounting.receivable_view"}, "Bạn không có quyền xem đối soát công nợ phải thu")
	if err != nil {
		return nil, err
	}
	return s.repo.ReceivableReconciliation(ctx, cleanID(receivable["id"]))
}

func (s *Service) ProjectOutputInvoices(ctx context.Context, projectID, actorID string) ([]Record, error) {
	projectID = cleanID(projectID)
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"accounting.view", "accounting.receivable_view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem hoá đơn đầu ra", Record{"permissions": []string{"accounting.view", "accounting.receivable_view"}})
	}
	return s.repo.ListOutputInvoicesForProject(ctx, projectID)
}

func (s *Service) CreateOutputInvoice(ctx context.Context, receivableID string, req OutputInvoiceRequest, fileHeader *multipart.FileHeader, actorID, actorName string) (Record, error) {
	receivable, err := s.requireReceivableAccess(ctx, receivableID, actorID, []string{"accounting.receivable_approve"}, "Bạn không có quyền upload hoá đơn đầu ra")
	if err != nil {
		return nil, err
	}
	body, err := s.normalizeOutputInvoiceRequest(ctx, receivable, req)
	if err != nil {
		return nil, err
	}
	fileMeta, storedObject, originalName, err := s.storeOutputInvoicePDF(ctx, cleanID(receivable["project_id"]), cleanID(receivable["id"]), fileHeader)
	if err != nil {
		return nil, err
	}
	body["file_name"] = originalName
	record, err := s.repo.CreateOutputInvoice(ctx, receivable, body, fileMeta, actorID, actorName)
	if err != nil {
		return nil, s.cleanupStoredObjectAfterError(storedObject, err)
	}
	return record, nil
}

func (s *Service) OutputInvoiceAction(ctx context.Context, invoiceID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	invoice, err := s.repo.GetOutputInvoiceDetail(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(invoice["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"accounting.receivable_approve"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền cập nhật hoá đơn đầu ra", Record{"permissions": []string{"accounting.receivable_approve"}})
	}
	return s.repo.OutputInvoiceAction(ctx, cleanID(invoice["id"]), action, req, actorID, actorName)
}

func (s *Service) OutputInvoiceFile(ctx context.Context, invoiceID, actorID string) (EvidenceFileResult, error) {
	invoiceID = cleanID(invoiceID)
	if !validActivityUUID(invoiceID) {
		return EvidenceFileResult{}, validationError("invoice_id không hợp lệ", Record{"invoice_id": invoiceID})
	}
	invoice, err := s.repo.GetOutputInvoiceDetail(ctx, invoiceID)
	if err != nil {
		return EvidenceFileResult{}, err
	}
	projectID := cleanID(invoice["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"accounting.view", "accounting.receivable_view"})
	if err != nil {
		return EvidenceFileResult{}, err
	}
	if !allowed {
		return EvidenceFileResult{}, forbiddenError("Bạn không có quyền xem file hoá đơn đầu ra")
	}
	if fmt.Sprint(invoice["status"]) == "deleted" {
		return EvidenceFileResult{}, validationError("Hoá đơn đầu ra đã bị xoá", Record{"invoice_id": invoiceID})
	}
	return s.storedFileResult(ctx, invoice, "storage_provider", "storage_key", "mime_type", fmt.Sprint(invoice["file_name"]))
}

func (s *Service) requireReceivableAccess(ctx context.Context, receivableID, actorID string, permissions []string, message string) (Record, error) {
	receivableID = cleanID(receivableID)
	if !validActivityUUID(receivableID) {
		return nil, validationError("receivable_id không hợp lệ", Record{"receivable_id": receivableID})
	}
	receivable, err := s.repo.GetReceivableDetail(ctx, receivableID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(receivable["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, permissions)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError(message, Record{"permissions": permissions})
	}
	return receivable, nil
}

func (s *Service) normalizeOutputInvoiceRequest(ctx context.Context, receivable Record, req OutputInvoiceRequest) (Record, error) {
	projectID := cleanID(receivable["project_id"])
	invoiceNumber := strings.TrimSpace(req.InvoiceNumber)
	if invoiceNumber == "" {
		return nil, validationError("Số hoá đơn là bắt buộc")
	}
	if len([]rune(invoiceNumber)) > 120 {
		return nil, validationError("Số hoá đơn không được vượt quá 120 ký tự")
	}
	invoiceSymbol := strings.TrimSpace(req.InvoiceSymbol)
	if len([]rune(invoiceSymbol)) > 80 {
		return nil, validationError("Ký hiệu hoá đơn không được vượt quá 80 ký tự")
	}
	invoiceDate := strings.TrimSpace(req.InvoiceDate)
	if invoiceDate == "" {
		return nil, validationError("Ngày hoá đơn là bắt buộc")
	}
	if _, err := time.Parse("2006-01-02", invoiceDate); err != nil {
		return nil, validationError("Ngày hoá đơn không hợp lệ", Record{"invoice_date": invoiceDate})
	}
	clientPartnerID := cleanID(req.ClientPartnerID)
	if clientPartnerID == "" {
		clientPartnerID = cleanID(receivable["client_partner_id"])
	}
	clientName := strings.TrimSpace(req.ClientName)
	clientTaxCode := strings.TrimSpace(req.ClientTaxCode)
	if clientPartnerID != "" {
		partner, err := s.requireActivePartner(ctx, clientPartnerID, []string{"client"})
		if err != nil {
			return nil, err
		}
		clientName = cleanOptionalText(partner["name"])
		clientTaxCode = cleanOptionalText(partner["tax_code"])
	}
	if clientName == "" {
		clientName = cleanOptionalText(receivable["client_name"])
	}
	if clientName == "" {
		return nil, validationError("Tên khách hàng trên hoá đơn là bắt buộc")
	}
	if len([]rune(clientName)) > 240 {
		return nil, validationError("Tên khách hàng trên hoá đơn không được vượt quá 240 ký tự")
	}
	if len([]rune(clientTaxCode)) > 80 {
		return nil, validationError("Mã số thuế khách hàng không được vượt quá 80 ký tự")
	}
	if req.AmountBeforeVAT < 0 || req.VATAmount < 0 || req.TotalAmount < 0 {
		return nil, validationError("Số tiền hoá đơn không được âm")
	}
	totalAmount := req.TotalAmount
	if totalAmount == 0 && req.AmountBeforeVAT+req.VATAmount > 0 {
		totalAmount = req.AmountBeforeVAT + req.VATAmount
	}
	if totalAmount <= 0 {
		return nil, validationError("Tổng tiền hoá đơn phải lớn hơn 0")
	}
	accountingBookID, revenueAccountID, err := s.resolveAccountingAccount(ctx, projectID, firstNonEmpty(req.RevenueAccountID, cleanID(receivable["revenue_account_id"])), "5111")
	if err != nil {
		return nil, err
	}
	taxCodeID, err := s.resolveInvoiceTaxCode(ctx, projectID, req.TaxCodeID, "output", req.AmountBeforeVAT, req.VATAmount)
	if err != nil {
		return nil, err
	}
	if req.AmountBeforeVAT+req.VATAmount > 0 && totalAmount+1 < req.AmountBeforeVAT+req.VATAmount {
		return nil, validationError("Tổng tiền hoá đơn nhỏ hơn trước VAT cộng VAT", Record{
			"amount_before_vat": req.AmountBeforeVAT,
			"vat_amount":        req.VATAmount,
			"total_amount":      totalAmount,
		})
	}
	paymentID := cleanID(req.PaymentID)
	if paymentID != "" {
		if !validActivityUUID(paymentID) {
			return nil, validationError("payment_id không hợp lệ", Record{"payment_id": paymentID})
		}
		payment, err := s.repo.GetByID(ctx, "payments", paymentID)
		if err != nil {
			return nil, validationError("Phiếu thu liên kết hoá đơn không tồn tại", Record{"payment_id": paymentID})
		}
		if cleanID(payment["project_id"]) != projectID || fmt.Sprint(payment["status"]) == "cancelled" || fmt.Sprint(payment["payment_type"]) != "client_receipt" {
			return nil, validationError("Phiếu thu liên kết hoá đơn không thuộc công trình hoặc không phải phiếu thu khách hàng", Record{"payment_id": paymentID})
		}
	}
	documentID := cleanID(req.DocumentID)
	if documentID != "" {
		if !validActivityUUID(documentID) {
			return nil, validationError("document_id không hợp lệ", Record{"document_id": documentID})
		}
		document, err := s.repo.GetByID(ctx, "documents", documentID)
		if err != nil {
			return nil, validationError("Tài liệu liên kết hoá đơn không tồn tại", Record{"document_id": documentID})
		}
		if cleanID(document["project_id"]) != projectID || fmt.Sprint(document["status"]) != "active" {
			return nil, validationError("Tài liệu liên kết hoá đơn không thuộc công trình", Record{"document_id": documentID})
		}
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, validationError("Ghi chú hoá đơn không được vượt quá 2000 ký tự")
	}
	return Record{
		"project_id":         projectID,
		"accounting_book_id": accountingBookID,
		"tax_code_id":        taxCodeID,
		"revenue_account_id": revenueAccountID,
		"receivable_id":      cleanID(receivable["id"]),
		"payment_id":         paymentID,
		"document_id":        documentID,
		"invoice_number":     invoiceNumber,
		"invoice_symbol":     invoiceSymbol,
		"invoice_date":       invoiceDate,
		"client_partner_id":  clientPartnerID,
		"client_name":        clientName,
		"client_tax_code":    clientTaxCode,
		"amount_before_vat":  req.AmountBeforeVAT,
		"vat_amount":         req.VATAmount,
		"total_amount":       totalAmount,
		"notes":              notes,
	}, nil
}
