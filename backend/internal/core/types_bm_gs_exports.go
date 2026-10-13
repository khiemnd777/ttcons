package core

// NativeDailyReportFilter mirrors the applied Operations journal range.
type NativeDailyReportFilter struct {
	DateFrom string
	DateTo   string
}

type NativeConstructionActionFilter struct {
	Queue, Search, AssigneeID, Priority, ImpactScope string
}

type BMGSSourceRef struct {
	Kind       string `json:"kind"`
	ID         string `json:"id"`
	RevisionNo int    `json:"revision_no"`
}
