package core

type SubcontractorReconciliationRequest struct {
	Comment         string                                   `json:"comment"`
	Items           []SubcontractorReconciliationItemRequest `json:"items"`
	WorkItemContext WorkItemMutationContext                  `json:"work_item_context"`
}

type SubcontractorReconciliationItemRequest struct {
	ItemIndex         int      `json:"item_index"`
	BOQItemID         string   `json:"boq_item_id"`
	ConfirmedQuantity *float64 `json:"confirmed_quantity"`
	Decision          string   `json:"decision"`
	Comment           string   `json:"comment"`
}

type OwnerClientReconciliationRequest struct {
	Comment         string                                 `json:"comment"`
	Items           []OwnerClientReconciliationItemRequest `json:"items"`
	WorkItemContext WorkItemMutationContext                `json:"work_item_context"`
}

type OwnerClientReconciliationItemRequest struct {
	ItemIndex    int    `json:"item_index"`
	AcceptanceID string `json:"acceptance_id"`
	Decision     string `json:"decision"`
	Comment      string `json:"comment"`
}

type MaterialSnapshotRequest struct {
	SnapshotDate     string                        `json:"snapshot_date"`
	ConstructionArea string                        `json:"construction_area"`
	Comment          string                        `json:"comment"`
	Items            []MaterialSnapshotItemRequest `json:"items"`
	WorkItemContext  WorkItemMutationContext       `json:"work_item_context"`
}

type MaterialSnapshotItemRequest struct {
	ItemIndex          int                     `json:"item_index"`
	MaterialID         string                  `json:"material_id"`
	MaterialName       string                  `json:"material_name"`
	Unit               string                  `json:"unit"`
	Source             string                  `json:"source"`
	BeforeQuantity     *float64                `json:"before_quantity"`
	AddedQuantity      *float64                `json:"added_quantity"`
	AfterQuantity      *float64                `json:"after_quantity"`
	WasteQuantity      *float64                `json:"waste_quantity"`
	AffectsSiteBalance *bool                   `json:"affects_site_balance"`
	BeforeEvidenceID   string                  `json:"before_evidence_id"`
	AfterEvidenceID    string                  `json:"after_evidence_id"`
	ConstructionArea   string                  `json:"construction_area"`
	Condition          string                  `json:"condition"`
	Comment            string                  `json:"comment"`
	StockUnitUsage     []StockUnitUsageRequest `json:"stock_unit_usage"`
}

type StockUnitUsageRequest struct {
	StockUnitID          string   `json:"stock_unit_id"`
	UsedQuantity         float64  `json:"used_quantity"`
	WasteQuantity        float64  `json:"waste_quantity"`
	RemainingCode        string   `json:"remaining_code"`
	RemainingLengthMM    *float64 `json:"remaining_length_mm"`
	RemainingWidthMM     *float64 `json:"remaining_width_mm"`
	RemainingThicknessMM *float64 `json:"remaining_thickness_mm"`
	Condition            string   `json:"condition"`
}
