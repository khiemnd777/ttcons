package core

type ReportOptionsRequest struct {
	Search     string
	Cursor     string
	Limit      int
	SelectedID string
}

type ReportProjectOption struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type ReportProjectOptionsPage struct {
	Items      []ReportProjectOption `json:"items"`
	HasMore    bool                  `json:"has_more"`
	NextCursor string                `json:"next_cursor,omitempty"`
	Selected   *ReportProjectOption  `json:"selected"`
}

type ReportOptionsResponse struct {
	Projects ReportProjectOptionsPage `json:"projects"`
}

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
