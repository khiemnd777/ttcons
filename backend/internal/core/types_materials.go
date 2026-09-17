package core

type SurplusHandlingRequest struct {
	SiteBalanceID          string                       `json:"site_balance_id"`
	MaterialSnapshotID     string                       `json:"material_snapshot_id"`
	MaterialSnapshotItemID string                       `json:"material_snapshot_item_id"`
	HandlingType           string                       `json:"handling_type"`
	Quantity               *float64                     `json:"quantity"`
	Unit                   string                       `json:"unit"`
	WarehouseID            string                       `json:"warehouse_id"`
	CustodyLocation        string                       `json:"custody_location"`
	Condition              string                       `json:"condition"`
	EvidenceID             string                       `json:"evidence_id"`
	Comment                string                       `json:"comment"`
	StockUnitAllocations   []StockUnitAllocationRequest `json:"stock_unit_allocations"`
	WorkItemContext        WorkItemMutationContext      `json:"work_item_context"`
}

type IncomingAcceptanceRequest struct {
	WorkItemID             string                          `json:"work_item_id"`
	ProcurementRequestID   string                          `json:"procurement_request_id"`
	WarehouseID            string                          `json:"warehouse_id"`
	SourceType             string                          `json:"source_type"`
	SupplierPartnerID      string                          `json:"supplier_partner_id"`
	SupplierName           string                          `json:"supplier_name"`
	SubcontractorPartnerID string                          `json:"subcontractor_partner_id"`
	SubcontractorName      string                          `json:"subcontractor_name"`
	DeliveryReference      string                          `json:"delivery_reference"`
	ReceivedAt             string                          `json:"received_at"`
	Result                 string                          `json:"result"`
	Comment                string                          `json:"comment"`
	Items                  []IncomingAcceptanceItemRequest `json:"items"`
	WorkItemContext        WorkItemMutationContext         `json:"work_item_context"`
}

type IncomingAcceptanceItemRequest struct {
	ItemIndex                int      `json:"item_index"`
	MaterialID               string   `json:"material_id"`
	MaterialName             string   `json:"material_name"`
	ProcurementRequestItemID string   `json:"procurement_request_item_id"`
	DeliveredQuantity        *float64 `json:"delivered_quantity"`
	AcceptedQuantity         *float64 `json:"accepted_quantity"`
	RejectedQuantity         *float64 `json:"rejected_quantity"`
	Unit                     string   `json:"unit"`
	Condition                string   `json:"condition"`
	DefectNote               string   `json:"defect_note"`
	EvidenceID               string   `json:"evidence_id"`
	Decision                 string   `json:"decision"`
	Comment                  string   `json:"comment"`
}

type ActivityAttentionSummary struct {
	Count   int  `json:"count"`
	HasMore bool `json:"has_more"`
}

type StockActionRequest struct {
	ProjectID         string  `json:"project_id"`
	WarehouseID       string  `json:"warehouse_id"`
	TargetWarehouseID string  `json:"target_warehouse_id"`
	MaterialID        string  `json:"material_id"`
	MovementType      string  `json:"movement_type"`
	Quantity          float64 `json:"quantity"`
	Unit              string  `json:"unit"`
	Reason            string  `json:"reason"`
	ReferenceType     string  `json:"reference_type"`
	ReferenceID       string  `json:"reference_id"`
	StockDocumentID   string  `json:"-"`
}

type StockDocumentItemRequest struct {
	MaterialID           string                       `json:"material_id"`
	Quantity             float64                      `json:"quantity"`
	Unit                 string                       `json:"unit"`
	Reason               string                       `json:"reason"`
	SourceLineType       string                       `json:"source_line_type"`
	SourceLineID         string                       `json:"source_line_id"`
	StockUnits           []MaterialStockUnitInput     `json:"stock_units"`
	StockUnitAllocations []StockUnitAllocationRequest `json:"stock_unit_allocations"`
}

type MaterialStockUnitInput struct {
	UnitCode        string   `json:"unit_code"`
	WarehouseID     string   `json:"warehouse_id"`
	Quantity        float64  `json:"quantity"`
	Unit            string   `json:"unit"`
	LengthMM        *float64 `json:"length_mm"`
	WidthMM         *float64 `json:"width_mm"`
	ThicknessMM     *float64 `json:"thickness_mm"`
	Condition       string   `json:"condition"`
	CustodyLocation string   `json:"custody_location"`
}

type StockUnitAllocationRequest struct {
	StockUnitID          string   `json:"stock_unit_id"`
	Quantity             float64  `json:"quantity"`
	ResultUnitCode       string   `json:"result_unit_code"`
	LengthMM             *float64 `json:"length_mm"`
	WidthMM              *float64 `json:"width_mm"`
	ThicknessMM          *float64 `json:"thickness_mm"`
	RemainingLengthMM    *float64 `json:"remaining_length_mm"`
	RemainingWidthMM     *float64 `json:"remaining_width_mm"`
	RemainingThicknessMM *float64 `json:"remaining_thickness_mm"`
	Condition            string   `json:"condition"`
	CustodyLocation      string   `json:"custody_location"`
}

type InitializeMaterialStockUnitsRequest struct {
	FormFactor string                   `json:"form_factor"`
	Units      []MaterialStockUnitInput `json:"units"`
}

type StockDocumentRequest struct {
	ProjectID         string                     `json:"project_id"`
	MovementType      string                     `json:"movement_type"`
	SourceWarehouseID string                     `json:"source_warehouse_id"`
	TargetWarehouseID string                     `json:"target_warehouse_id"`
	ReferenceType     string                     `json:"reference_type"`
	ReferenceID       string                     `json:"reference_id"`
	DocumentDate      string                     `json:"document_date"`
	Note              string                     `json:"note"`
	CorrectionReason  string                     `json:"correction_reason"`
	Items             []StockDocumentItemRequest `json:"items"`
	WorkItemContext   WorkItemMutationContext    `json:"work_item_context"`
}

type StockAdjustmentRequest struct {
	WarehouseID    string  `json:"warehouse_id"`
	MaterialID     string  `json:"material_id"`
	TargetQuantity float64 `json:"target_quantity"`
	DocumentDate   string  `json:"document_date"`
	Reason         string  `json:"reason"`
}

type SiteMaterialUsageAdjustmentRequest struct {
	TargetUsedQuantity float64 `json:"target_used_quantity"`
	Reason             string  `json:"reason"`
}
