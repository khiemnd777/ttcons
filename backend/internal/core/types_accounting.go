package core

type AccountingJournalLineRequest struct {
	AccountID    string  `json:"account_id"`
	PartnerID    string  `json:"partner_id"`
	CostCode     string  `json:"cost_code"`
	Description  string  `json:"description"`
	DebitAmount  float64 `json:"debit_amount"`
	CreditAmount float64 `json:"credit_amount"`
}

type AccountingJournalCreateRequest struct {
	EntryDate        string                         `json:"entry_date"`
	DocumentDate     string                         `json:"document_date"`
	SourceDocumentNo string                         `json:"source_document_no"`
	Description      string                         `json:"description"`
	Lines            []AccountingJournalLineRequest `json:"lines"`
}

type AccountingJournalReverseRequest struct {
	EntryDate string `json:"entry_date"`
	Reason    string `json:"reason"`
}

type AccountingPeriodActionRequest struct {
	Reason string `json:"reason"`
}

type AccountingTaxPeriodActionRequest struct {
	Reason          string `json:"reason"`
	FilingReference string `json:"filing_reference"`
}

type AccountingTaxAdjustmentCreateRequest struct {
	ProjectID        string  `json:"project_id"`
	Direction        string  `json:"direction"`
	TaxCodeID        string  `json:"tax_code_id"`
	OffsetAccountID  string  `json:"offset_account_id"`
	EntryDate        string  `json:"entry_date"`
	TaxableDelta     float64 `json:"taxable_delta"`
	VATDelta         float64 `json:"vat_delta"`
	Reason           string  `json:"reason"`
	SourceDocumentNo string  `json:"source_document_no"`
}

type AccountingTaxAdjustmentReverseRequest struct {
	EntryDate string `json:"entry_date"`
	Reason    string `json:"reason"`
}

type AccountingTaxPaymentCreateRequest struct {
	ProjectID     string  `json:"project_id"`
	PaymentCode   string  `json:"payment_code"`
	Amount        float64 `json:"amount"`
	PaymentMethod string  `json:"payment_method"`
	CashAccountID string  `json:"cash_account_id"`
}

type AccountingReconciliationResolveRequest struct {
	Action         string `json:"action"`
	ResolutionNote string `json:"resolution_note"`
	JournalEntryID string `json:"journal_entry_id"`
}

type AccountingJournalFilter struct {
	ProjectID string
	DateFrom  string
	DateTo    string
	AccountID string
}

type AccountingReportFilter struct {
	ProjectID string
	DateFrom  string
	DateTo    string
}
