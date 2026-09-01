package core

type ConstructionDailyReportCreateRequest struct {
	ReportDate        string                                   `json:"report_date"`
	Weather           string                                   `json:"weather"`
	ProgressPercent   int                                      `json:"progress_percent"`
	WorkItemID        string                                   `json:"work_item_id"`
	DoneItems         []ConstructionDailyReportWorkItemRequest `json:"done_items"`
	CurrentItems      []ConstructionDailyReportWorkItemRequest `json:"current_items"`
	NextItems         []ConstructionDailyReportWorkItemRequest `json:"next_items"`
	Blockers          []ConstructionDailyReportBlockerRequest  `json:"blockers"`
	FieldNotes        string                                   `json:"field_notes"`
	CrewItems         []ConstructionDailyReportCrewRequest     `json:"crew_items"`
	MaterialItems     []ConstructionDailyReportMaterialRequest `json:"material_items"`
	EvidenceIDs       []string                                 `json:"evidence_ids"`
	PrimaryEvidenceID string                                   `json:"primary_evidence_id"`
	ManpowerSummary   string                                   `json:"manpower_summary"`
	MaterialSummary   string                                   `json:"material_usage_summary"`
	Delivery          WorkReportDeliveryRequest                `json:"delivery"`
}

type ConstructionDailyReportWorkItemRequest struct {
	Title string `json:"title"`
}

type ConstructionDailyReportBlockerRequest struct {
	Content string `json:"content"`
}

type ConstructionDailyReportCrewRequest struct {
	CrewName    string  `json:"crew_name"`
	WorkerCount int     `json:"worker_count"`
	WorkHours   float64 `json:"work_hours"`
	Note        string  `json:"note"`
}

type ConstructionDailyReportMaterialRequest struct {
	MaterialName string  `json:"material_name"`
	Quantity     float64 `json:"quantity"`
	Unit         string  `json:"unit"`
	Note         string  `json:"note"`
}

type ConstructionRecurringPlanRequest struct {
	PlanCode         string `json:"plan_code"`
	Title            string `json:"title"`
	Description      string `json:"description"`
	RecurrenceType   string `json:"recurrence_type"`
	IntervalValue    int    `json:"interval_value"`
	StartDate        string `json:"start_date"`
	EndDate          string `json:"end_date"`
	DueTime          string `json:"due_time"`
	ResponsibleRole  string `json:"responsible_role"`
	AssignedUserID   string `json:"assigned_user_id"`
	EvidenceRequired bool   `json:"evidence_required"`
	SourceWorkItemID string `json:"source_work_item_id"`
}

type ConstructionRecurringOccurrenceRequest struct {
	Comment    string `json:"comment"`
	EvidenceID string `json:"evidence_id"`
	Reason     string `json:"reason"`
}

type ConstructionCheckCriterion struct {
	Title            string `json:"title"`
	Criteria         string `json:"criteria"`
	EvidenceRequired bool   `json:"evidence_required"`
}

type ConstructionHandoffItemRequest struct {
	SourceFieldKey   string                       `json:"source_field_key"`
	SourceItemIndex  int                          `json:"source_item_index"`
	Title            string                       `json:"title"`
	Location         string                       `json:"location"`
	ScheduledAt      string                       `json:"scheduled_at"`
	ResponsibleRole  string                       `json:"responsible_role"`
	AssignedUserID   string                       `json:"assigned_user_id"`
	EvidenceRequired bool                         `json:"evidence_required"`
	Criteria         []ConstructionCheckCriterion `json:"criteria"`
}

type ConstructionHandoffActivateRequest struct {
	Title                string                           `json:"title"`
	Notes                string                           `json:"notes"`
	SourceRunNodeID      string                           `json:"source_run_node_id"`
	SourceFormInstanceID string                           `json:"source_form_instance_id"`
	SourceFormRevisionNo int                              `json:"source_form_revision_no"`
	Items                []ConstructionHandoffItemRequest `json:"items"`
	WorkItemContext      WorkItemMutationContext          `json:"work_item_context"`
}

type ConstructionCheckTemplateRequest struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Criteria    []ConstructionCheckCriterion `json:"criteria"`
}

type ConstructionCheckRequest struct {
	WorkItemID          string                       `json:"work_item_id"`
	SourceDailyReportID string                       `json:"source_daily_report_id"`
	TemplateID          string                       `json:"template_id"`
	Title               string                       `json:"title"`
	Location            string                       `json:"location"`
	ScheduledAt         string                       `json:"scheduled_at"`
	Criteria            []ConstructionCheckCriterion `json:"criteria"`
	ContextEvidenceIDs  []string                     `json:"context_evidence_ids"`
}

type ConstructionCheckUpdateRequest struct {
	WorkItemID         string                       `json:"work_item_id"`
	TemplateID         string                       `json:"template_id"`
	Title              string                       `json:"title"`
	Location           string                       `json:"location"`
	ScheduledAt        string                       `json:"scheduled_at"`
	Criteria           []ConstructionCheckCriterion `json:"criteria"`
	ContextEvidenceIDs []string                     `json:"context_evidence_ids"`
}

type ConstructionCheckCompleteRequest struct {
	Result         string   `json:"result"`
	Note           string   `json:"note"`
	EvidenceID     string   `json:"evidence_id"`
	EvidenceIDs    []string `json:"evidence_ids"`
	ActionTitle    string   `json:"action_title"`
	AssignedUserID string   `json:"assigned_user_id"`
	DueAt          string   `json:"due_at"`
	ImpactScope    string   `json:"impact_scope"`
	Priority       string   `json:"priority"`
}

type ConstructionActionRequest struct {
	WorkItemID          string `json:"work_item_id"`
	SourceDailyReportID string `json:"source_daily_report_id"`
	Title               string `json:"title"`
	Description         string `json:"description"`
	Priority            string `json:"priority"`
	AssignedUserID      string `json:"assigned_user_id"`
	DueAt               string `json:"due_at"`
	ImpactScope         string `json:"impact_scope"`
	EvidenceID          string `json:"evidence_id"`
}

type ConstructionActionUpdateRequest struct {
	Title          string `json:"title"`
	Description    string `json:"description"`
	Priority       string `json:"priority"`
	AssignedUserID string `json:"assigned_user_id"`
	DueAt          string `json:"due_at"`
}

type ConstructionActionResolutionRequest struct {
	EvidenceID string `json:"evidence_id"`
	Note       string `json:"note"`
}

type ConstructionActionVerifyRequest struct {
	Result  string `json:"result"`
	Comment string `json:"comment"`
}
