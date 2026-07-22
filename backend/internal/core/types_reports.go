package core

type OperationalReportRequest struct {
	ProjectID string `json:"project_id"`
}

type CashflowReportRequest struct {
	ProjectID string `json:"project_id"`
	FromDate  string `json:"from"`
	ToDate    string `json:"to"`
	GroupBy   string `json:"group_by"`
	Status    string `json:"status"`
}

type TaxReportRequest struct {
	ProjectID string `json:"project_id"`
	FromDate  string `json:"from"`
	ToDate    string `json:"to"`
	GroupBy   string `json:"group_by"`
	Source    string `json:"source"`
	Status    string `json:"status"`
}

type ProfitReportRequest struct {
	ProjectID string `json:"project_id"`
	FromDate  string `json:"from"`
	ToDate    string `json:"to"`
	GroupBy   string `json:"group_by"`
	Status    string `json:"status"`
}
