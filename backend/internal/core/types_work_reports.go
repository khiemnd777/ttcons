package core

type WorkReportFilter struct {
	Scope        string
	ProjectID    string
	Status       string
	DateFrom     string
	DateTo       string
	Search       string
	SupervisorID string
	ReadStatus   string
	Limit        int
	Offset       int
}

type WorkReportSubmitRequest struct {
	RecipientID string `json:"recipient_id"`
	Comment     string `json:"comment"`
}

type WorkReportDeliveryRequest struct {
	Mode        string `json:"mode"`
	RecipientID string `json:"recipient_id"`
	Comment     string `json:"comment"`
}

type WorkReportReviewRequest struct {
	Comment    string `json:"comment"`
	RevisionNo int    `json:"revision_no"`
}
