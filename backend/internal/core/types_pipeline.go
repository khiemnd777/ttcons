package core

type ActionRequest struct {
	Comment         string                  `json:"comment"`
	Reason          string                  `json:"reason"`
	WorkItemContext WorkItemMutationContext `json:"work_item_context"`
}

type WorkItemMutationContext struct {
	WorkItemID string `json:"work_item_id"`
	Intent     string `json:"intent"`
}

type WorkItemMutationRequest struct {
	WorkItemContext WorkItemMutationContext `json:"work_item_context"`
}

type PipelineTemplateLayoutRequest struct {
	Nodes []PipelineTemplateNodeLayoutRequest `json:"nodes"`
	Edges []PipelineTemplateEdgeLayoutRequest `json:"edges"`
}

type PipelineTemplateNodeLayoutRequest struct {
	ID        string `json:"id"`
	PositionX int    `json:"position_x"`
	PositionY int    `json:"position_y"`
}

type PipelineTemplateEdgeLayoutRequest struct {
	ID         string `json:"id"`
	SourceSide string `json:"source_side"`
	TargetSide string `json:"target_side"`
}
