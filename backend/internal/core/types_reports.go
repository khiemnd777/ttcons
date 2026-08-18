package core

type DashboardPeriodRequest struct {
	PeriodType string
	Year       int
	PeriodNo   int
}

type DashboardPeriodMeta struct {
	PeriodType     string `json:"period_type"`
	Year           int    `json:"year"`
	PeriodNo       int    `json:"period_no,omitempty"`
	FromDate       string `json:"from"`
	ToDate         string `json:"to"`
	AsOfDate       string `json:"as_of"`
	GroupBy        string `json:"group_by"`
	IsCurrent      bool   `json:"is_current_period"`
	CanMoveForward bool   `json:"can_move_forward"`
}

type DashboardCapabilities struct {
	FinanceVisible bool `json:"finance_visible"`
}

type DashboardStatusMetric struct {
	Status string `json:"status"`
	Total  int    `json:"total"`
}

type DashboardProjectSnapshot struct {
	TotalProjects     int                     `json:"total_projects"`
	ActiveProjects    int                     `json:"active_projects"`
	CompletedProjects int                     `json:"completed_projects"`
	BlockedProjects   int                     `json:"blocked_projects"`
	OpenIssues        int                     `json:"open_issues"`
	PendingApprovals  int                     `json:"pending_approvals"`
	AverageProgress   float64                 `json:"average_progress"`
	StatusCounts      []DashboardStatusMetric `json:"status_counts"`
}

type DashboardCashflowSummary struct {
	OpeningBalance      float64 `json:"opening_balance"`
	TotalInflow         float64 `json:"total_inflow"`
	TotalOutflow        float64 `json:"total_outflow"`
	NetFlow             float64 `json:"net_flow"`
	ClosingBalance      float64 `json:"closing_balance"`
	PaymentCount        int     `json:"payment_count"`
	PendingPaymentCount int     `json:"pending_payment_count"`
	PendingPaymentTotal float64 `json:"pending_payment_total"`
	UnclassifiedCount   int     `json:"unclassified_count"`
	NonCashCount        int     `json:"non_cash_count"`
}

type DashboardCashflowSeries struct {
	PeriodStart    string  `json:"period_start"`
	InflowAmount   float64 `json:"inflow_amount"`
	OutflowAmount  float64 `json:"outflow_amount"`
	NetAmount      float64 `json:"net_amount"`
	RunningBalance float64 `json:"running_balance"`
	PaymentCount   int     `json:"payment_count"`
}

type DashboardCashflowProject struct {
	ProjectID     string  `json:"project_id"`
	ProjectName   string  `json:"project_name"`
	InflowAmount  float64 `json:"inflow_amount"`
	OutflowAmount float64 `json:"outflow_amount"`
	NetAmount     float64 `json:"net_amount"`
	PaymentCount  int     `json:"payment_count"`
}

type DashboardCashflow struct {
	Summary  DashboardCashflowSummary   `json:"summary"`
	Series   []DashboardCashflowSeries  `json:"series"`
	Projects []DashboardCashflowProject `json:"projects"`
}

type DashboardReportResponse struct {
	Meta            DashboardPeriodMeta      `json:"meta"`
	Capabilities    DashboardCapabilities    `json:"capabilities"`
	ProjectSnapshot DashboardProjectSnapshot `json:"project_snapshot"`
	Cashflow        *DashboardCashflow       `json:"cashflow,omitempty"`
}

type DashboardProjectsRequest struct {
	DashboardPeriodRequest
	Search    string
	Status    string
	Sort      string
	Direction string
	Cursor    string
	Limit     int
}

type DashboardProjectItem struct {
	ID               string   `json:"id"`
	Code             string   `json:"code"`
	Name             string   `json:"name"`
	Status           string   `json:"status"`
	ProgressPercent  float64  `json:"progress_percent"`
	OpenIssues       int      `json:"open_issues"`
	PendingApprovals int      `json:"pending_approvals"`
	FinanceVisible   bool     `json:"finance_visible"`
	InflowAmount     *float64 `json:"inflow_amount,omitempty"`
	OutflowAmount    *float64 `json:"outflow_amount,omitempty"`
	NetAmount        *float64 `json:"net_amount,omitempty"`
}

type DashboardProjectsPage struct {
	Items      []DashboardProjectItem `json:"items"`
	Total      int                    `json:"total"`
	HasMore    bool                   `json:"has_more"`
	NextCursor string                 `json:"next_cursor,omitempty"`
}

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
	Search    string `json:"search"`
	Sort      string `json:"sort"`
	Direction string `json:"direction"`
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
