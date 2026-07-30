package core

type ProjectMaterialCostRequest struct {
	ProcurementRequestID string  `json:"procurement_request_id"`
	IncomingAcceptanceID string  `json:"incoming_acceptance_id"`
	StockDocumentID      string  `json:"stock_document_id"`
	DocumentID           string  `json:"document_id"`
	VendorPartnerID      string  `json:"vendor_partner_id"`
	VendorName           string  `json:"vendor_name"`
	InvoiceNumber        string  `json:"invoice_number"`
	InvoiceDate          string  `json:"invoice_date"`
	Description          string  `json:"description"`
	AmountBeforeVAT      float64 `json:"amount_before_vat"`
	VATAmount            float64 `json:"vat_amount"`
	TotalAmount          float64 `json:"total_amount"`
	Notes                string  `json:"notes"`
}

type ProjectCashRequestRequest struct {
	Direction        string  `json:"direction"`
	Title            string  `json:"title"`
	CounterpartyName string  `json:"counterparty_name"`
	Amount           float64 `json:"amount"`
	DueDate          string  `json:"due_date"`
	SourceType       string  `json:"source_type"`
	SourceID         string  `json:"source_id"`
	DocumentID       string  `json:"document_id"`
	Notes            string  `json:"notes"`
}

type ProjectFinanceActionRequest struct {
	Comment              string  `json:"comment"`
	RecognizedCostAmount float64 `json:"recognized_cost_amount"`
}
