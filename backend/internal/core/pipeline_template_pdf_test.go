package core

import (
	"bytes"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"codeberg.org/go-pdf/fpdf"
)

func TestBuildPipelineTemplatePDFDocument(t *testing.T) {
	template := Record{
		"name":        "Workflow hoàn thiện công trình",
		"description": "Quy trình mẫu",
		"revision_no": 2,
		"status":      "published",
		"report_config_json": Record{
			"node_reference_title": "Đối chiếu đánh số client -> Rev4",
			"node_references": []any{
				Record{"source_ref": "Node 1", "target_business_code": "survey", "note": "Khảo sát"},
			},
		},
		"nodes": []Record{
			{
				"id":                  "node-2",
				"order_index":         20,
				"business_code":       "construction_execution",
				"name":                "Thi công",
				"type":                "construction",
				"position_x":          360,
				"position_y":          40,
				"required_permission": "project.execute",
				"approval_policy_json": Record{
					"mode": "none",
				},
			},
			{
				"id":            "node-1",
				"order_index":   10,
				"business_code": "survey",
				"name":          "Khảo sát hiện trạng",
				"description":   "Ghi nhận kích thước và rủi ro",
				"type":          "survey",
				"position_x":    40,
				"position_y":    40,
				"permissions":   []any{"pipeline.view", "project.execute"},
				"approval_policy_json": Record{
					"mode": "ordered",
					"stages": []any{
						Record{"roles": []any{"PM"}, "quorum": 1},
						Record{"roles": []any{"Director"}, "quorum": 1},
					},
					"comment_required":         true,
					"requester_cannot_approve": true,
				},
				"steps": []Record{
					{"id": "step-2", "order_index": 20, "business_code": "survey.site_photos", "name": "Tải ảnh hiện trạng", "step_type": "photo_upload", "required_role_ids": []any{"QS"}, "photo_required": true},
					{"id": "step-1", "order_index": 10, "business_code": "survey.minutes", "name": "Lập biên bản khảo sát", "step_type": "checklist", "required_role_ids": []any{"PM", "QA/QC"}, "checklist_required": true, "document_required": true},
				},
			},
		},
		"edges": []Record{
			{
				"id":           "edge-1",
				"from_node_id": "node-1",
				"to_node_id":   "node-2",
				"label":        "Khảo sát xong",
				"priority":     100,
				"condition_config": Record{
					"diagram": Record{
						"portMode": "manual",
						"source":   Record{"nodeId": "node-1", "side": "right"},
						"target":   Record{"nodeId": "node-2", "side": "left"},
					},
				},
			},
		},
	}

	document := buildPipelineTemplatePDFDocument(template, map[string]string{
		"pipeline.view":   "Xem workflow",
		"project.execute": "Thực hiện công trình",
	}, time.Date(2026, time.July, 17, 9, 30, 0, 0, time.FixedZone("ICT", 7*60*60)))

	if document.Name != "Workflow hoàn thiện công trình" || document.Revision != 2 || document.Status != "Đã phát hành" {
		t.Fatalf("unexpected document metadata: %#v", document)
	}
	if len(document.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(document.Nodes))
	}
	first := document.Nodes[0]
	if first.Name != "Khảo sát hiện trạng" || first.Order != 10 || first.BusinessCode != "survey" {
		t.Fatalf("nodes are not ordered by order_index: %#v", first)
	}
	if first.Type != "Khảo sát (survey)" {
		t.Fatalf("unexpected node type label: %q", first.Type)
	}
	if first.Permissions != "Xem workflow (pipeline.view)\nThực hiện công trình (project.execute)" {
		t.Fatalf("unexpected permissions: %q", first.Permissions)
	}
	if first.ApprovalRoles != "Theo thứ tự: PM > Director" {
		t.Fatalf("unexpected approval roles: %q", first.ApprovalRoles)
	}
	if first.CommentRequired != "Có" || first.RequesterCannotApprove != "Có" {
		t.Fatalf("unexpected approval constraints: %#v", first)
	}
	if len(first.Steps) != 2 || first.Steps[0].Name != "Lập biên bản khảo sát" {
		t.Fatalf("steps are not ordered by order_index: %#v", first.Steps)
	}
	if !first.HasPosition || first.PositionX != 40 || first.PositionY != 40 {
		t.Fatalf("saved diagram position was not exported: %#v", first)
	}
	if first.Steps[0].Type != "Checklist (checklist)" || first.Steps[0].Roles != "PM, QA/QC" {
		t.Fatalf("unexpected first step: %#v", first.Steps[0])
	}
	if first.Steps[0].BusinessCode != "survey.minutes" || first.Steps[0].ChecklistRequired != "Có" || first.Steps[0].DocumentRequired != "Có" || first.Steps[0].PhotoRequired != "Không" {
		t.Fatalf("step business code/evidence gates were not exported: %#v", first.Steps[0])
	}
	if document.NodeReferenceTitle != "Đối chiếu đánh số client -> Rev4" || len(document.NodeReferences) != 1 {
		t.Fatalf("expected report-only client node reference appendix: %#v", document.NodeReferences)
	}
	reference := document.NodeReferences[0]
	if reference.SourceRef != "Node 1" || reference.TargetOrder != 10 || reference.BusinessCode != "survey" || reference.TargetName != "Khảo sát hiện trạng" {
		t.Fatalf("unexpected client node reference: %#v", reference)
	}

	second := document.Nodes[1]
	if second.Permissions != "Thực hiện công trình (project.execute)" {
		t.Fatalf("legacy permission fallback was not exported: %q", second.Permissions)
	}
	if second.ApprovalRoles != "Không áp dụng" || second.CommentRequired != "Không áp dụng" || second.RequesterCannotApprove != "Không áp dụng" {
		t.Fatalf("approval-disabled node should use not-applicable labels: %#v", second)
	}
	if len(document.Edges) != 1 {
		t.Fatalf("expected one diagram edge, got %#v", document.Edges)
	}
	edge := document.Edges[0]
	if edge.FromNodeID != "node-1" || edge.ToNodeID != "node-2" || edge.PortMode != "manual" || edge.SourceSide != "right" || edge.TargetSide != "left" {
		t.Fatalf("unexpected diagram edge metadata: %#v", edge)
	}
}

func TestBuildPipelinePDFDiagramLayoutUsesDocumentTopologyInsteadOfCanvasPresentation(t *testing.T) {
	document := pipelineTemplatePDFDocument{
		Nodes: []pipelineTemplatePDFNode{
			{ID: "node-1", Order: 1, Name: "Khảo sát", DiagramType: "Khảo sát", PositionX: 20, PositionY: 30, HasPosition: true},
			{ID: "node-2", Order: 2, Name: "Thiết kế", DiagramType: "Thiết kế", PositionX: 20, PositionY: 30, HasPosition: true},
		},
		Edges: []pipelineTemplatePDFEdge{{
			ID: "edge-1", FromNodeID: "node-1", ToNodeID: "node-2", Label: "Có thiết kế", PortMode: "manual", SourceSide: "bottom", TargetSide: "top",
		}},
	}

	layout := buildPipelinePDFDiagramLayout(document, 277, 130)

	if layout.Mode != "Theo luồng workflow" || !layout.Topology {
		t.Fatalf("expected topology layout mode, got %#v", layout)
	}
	if len(layout.Nodes) != 2 || len(layout.Edges) != 1 || layout.DanglingEdges != 0 {
		t.Fatalf("unexpected diagram layout counts: %#v", layout)
	}
	edge := layout.Edges[0]
	if edge.SourceSide != "right" || edge.TargetSide != "left" {
		t.Fatalf("PDF topology should calculate connector sides independently of canvas metadata: %#v", edge)
	}
	if len(edge.Route) < 2 || !edge.HasLabelPoint {
		t.Fatalf("expected routed and labeled connector: %#v", edge)
	}
	if layout.Nodes[0].Column != 0 || layout.Nodes[1].Column != 1 {
		t.Fatalf("expected consecutive topology columns: %#v", layout.Nodes)
	}
}

func TestBuildPipelinePDFDiagramLayoutSkipsDanglingEdgesWithoutUsingSparseCanvasPositions(t *testing.T) {
	document := pipelineTemplatePDFDocument{
		Nodes: []pipelineTemplatePDFNode{
			{ID: "node-1", Order: 1, Name: "Khảo sát", DiagramType: "Khảo sát", PositionX: 0, PositionY: 0, HasPosition: true},
			{ID: "node-2", Order: 2, Name: "Thi công", DiagramType: "Thi công", PositionX: 10_000, PositionY: 10_000, HasPosition: true},
		},
		Edges: []pipelineTemplatePDFEdge{
			{ID: "edge-1", FromNodeID: "node-1", ToNodeID: "node-2", PortMode: "auto"},
			{ID: "edge-2", FromNodeID: "node-2", ToNodeID: "missing-node", PortMode: "auto"},
		},
	}

	layout := buildPipelinePDFDiagramLayout(document, 277, 130)

	if layout.Mode != "Theo luồng workflow" || !layout.Topology {
		t.Fatalf("expected topology layout, got %#v", layout)
	}
	if len(layout.Nodes) != 2 || len(layout.Edges) != 1 || layout.DanglingEdges != 1 {
		t.Fatalf("unexpected fallback layout: %#v", layout)
	}
	if layout.Edges[0].SourceSide != "right" || layout.Edges[0].TargetSide != "left" {
		t.Fatalf("unexpected automatic connector sides: %#v", layout.Edges[0])
	}
}

func TestBuildPipelinePDFDiagramLayoutUsesStableOrderWhenThereAreNoEdges(t *testing.T) {
	document := pipelineTemplatePDFDocument{Nodes: []pipelineTemplatePDFNode{
		{ID: "node-1", Order: 1, Name: "Khảo sát", PositionX: 100, PositionY: 100, HasPosition: true},
		{ID: "node-2", Order: 2, Name: "Thi công", PositionX: 120, PositionY: 110, HasPosition: true},
	}}

	layout := buildPipelinePDFDiagramLayout(document, 277, 130)

	if layout.Mode != "Theo thứ tự node" || layout.FallbackReason != "no_edges" {
		t.Fatalf("expected stable-order no-edge layout, got %#v", layout)
	}
	if pipelinePDFDiagramRectsOverlap(layout.Nodes[0].Rect, layout.Nodes[1].Rect) {
		t.Fatalf("stable-order cards must not overlap: %#v", layout.Nodes)
	}
}

func TestBuildPipelinePDFDiagramLayoutRanksBranchesAndWrapsTopologyBands(t *testing.T) {
	document := pipelinePDFTopologyTestDocument()

	layout := buildPipelinePDFDiagramLayout(document, 277, 130)

	if layout.Mode != "Theo luồng workflow" || len(layout.Nodes) != 13 || len(layout.Edges) != 13 {
		t.Fatalf("unexpected topology layout: %#v", layout)
	}
	nodes := make(map[string]pipelinePDFDiagramNodeLayout, len(layout.Nodes))
	for _, node := range layout.Nodes {
		nodes[node.Node.ID] = node
	}
	if nodes["node-3"].Rank != 2 || nodes["node-4"].Rank != 2 || nodes["node-3"].Column != nodes["node-4"].Column || nodes["node-3"].Lane == nodes["node-4"].Lane {
		t.Fatalf("branch nodes should share a topology rank in separate lanes: node3=%#v node4=%#v", nodes["node-3"], nodes["node-4"])
	}
	if nodes["node-6"].Band != 0 || nodes["node-6"].Column != 4 || nodes["node-7"].Band != 1 || nodes["node-7"].Column != 0 {
		t.Fatalf("expected rank wrapping after five columns: node6=%#v node7=%#v", nodes["node-6"], nodes["node-7"])
	}
	if nodes["node-1"].Rect.Width < 35 || nodes["node-1"].Rect.Width > 42 || nodes["node-1"].Rect.Height < 13 || nodes["node-1"].Rect.Height > 17 {
		t.Fatalf("detail cards should stay compact and readable: %#v", nodes["node-1"].Rect)
	}
	for _, edge := range layout.Edges {
		if edge.Edge.FromNodeID == "node-6" && edge.Edge.ToNodeID == "node-7" {
			if edge.SourceSide != "bottom" || edge.TargetSide != "top" || len(edge.Route) < 5 {
				t.Fatalf("band transition should use the outer gutter: %#v", edge)
			}
			return
		}
	}
	t.Fatal("expected node-6 -> node-7 band transition")
}

func TestPipelinePDFDiagramShowsEveryConnectorLabel(t *testing.T) {
	document := pipelineTemplatePDFDocument{
		Nodes: []pipelineTemplatePDFNode{
			{ID: "node-1", Order: 1},
			{ID: "node-2", Order: 2},
			{ID: "node-3", Order: 3},
			{ID: "node-4", Order: 4},
		},
		Edges: []pipelineTemplatePDFEdge{
			{ID: "edge-1", FromNodeID: "node-1", ToNodeID: "node-2", Label: "Hoàn tất khảo sát"},
			{ID: "edge-2", FromNodeID: "node-2", ToNodeID: "node-3", Label: "Có thiết kế"},
			{ID: "edge-3", FromNodeID: "node-2", ToNodeID: "node-4", Label: "Không thiết kế"},
		},
	}

	layout := buildPipelinePDFDiagramLayout(document, 277, 130)

	if pipelinePDFDiagramLabeledEdgeCount(layout) != 3 {
		t.Fatalf("diagram should label every connector: %#v", layout.Edges)
	}
}

func TestBuildPipelinePDFDiagramLayoutFallsBackForCycle(t *testing.T) {
	document := pipelineTemplatePDFDocument{
		Nodes: []pipelineTemplatePDFNode{{ID: "node-1", Order: 1}, {ID: "node-2", Order: 2}},
		Edges: []pipelineTemplatePDFEdge{
			{ID: "edge-1", FromNodeID: "node-1", ToNodeID: "node-2"},
			{ID: "edge-2", FromNodeID: "node-2", ToNodeID: "node-1"},
		},
	}

	layout := buildPipelinePDFDiagramLayout(document, 277, 130)

	if layout.FallbackReason != "cycle" || layout.Topology || layout.Mode != "Theo thứ tự node (graph có cycle)" {
		t.Fatalf("expected cycle fallback, got %#v", layout)
	}
}

func TestPipelinePDFDiagramMetadataSupportsLegacyAliases(t *testing.T) {
	mode, source, target := pipelinePDFDiagramMetadata(Record{
		"diagram": Record{
			"port_mode":  "manual",
			"sourcePort": Record{"port_side": "left"},
			"targetPort": Record{"portSide": "bottom"},
		},
	})

	if mode != "manual" || source != "left" || target != "bottom" {
		t.Fatalf("unexpected legacy diagram metadata: %q %q %q", mode, source, target)
	}
}

func TestPipelinePDFDiagramEdgeLabelPlacementAvoidsNodeCards(t *testing.T) {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes(pipelinePDFFontFamily, "", pipelinePDFRegularFont)
	pdf.AddUTF8FontFromBytes(pipelinePDFFontFamily, "B", pipelinePDFBoldFont)
	pdf.AddPage()
	nodes := []pipelinePDFDiagramNodeLayout{
		{Rect: pipelinePDFDiagramRect{X: 10, Y: 10, Width: 40, Height: 20}},
		{Rect: pipelinePDFDiagramRect{X: 60, Y: 10, Width: 40, Height: 20}},
	}
	route := []pipelinePDFDiagramPoint{{X: 50, Y: 20}, {X: 60, Y: 20}}

	placement, ok := pipelinePDFDiagramEdgeLabelPlacement(pdf, "Khảo sát xong", route, nodes, nil, 110, 60)
	if !ok {
		t.Fatal("expected a connector label placement")
	}
	if pipelinePDFDiagramLabelOverlapsNodes(placement.Rect, nodes) {
		t.Fatalf("connector label overlaps a node card: %#v", placement)
	}
	if len(placement.Lines) > 2 || strings.Join(placement.Lines, " ") != "Khảo sát xong" {
		t.Fatalf("expected the complete connector label in at most two lines: %#v", placement)
	}
}

func TestPipelinePDFDiagramOfficialBranchLabelsStayOnOwningRoutes(t *testing.T) {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes(pipelinePDFFontFamily, "", pipelinePDFRegularFont)
	pdf.AddUTF8FontFromBytes(pipelinePDFFontFamily, "B", pipelinePDFBoldFont)
	pdf.AddPage()
	document := pipelinePDFOfficialBranchTestDocument()
	layout := buildPipelinePDFDiagramLayout(document, 277, 130)
	used := make([]pipelinePDFDiagramLabelPlacement, 0, len(layout.Edges))
	found := map[string]bool{}
	for _, edge := range layout.Edges {
		placement, ok := pipelinePDFDiagramEdgeLabelPlacement(pdf, edge.Edge.Label, edge.Route, layout.Nodes, used, 277, 130)
		if !ok {
			t.Fatalf("expected placement for edge %s", edge.Edge.ID)
		}
		used = append(used, placement)
		if edge.Edge.Label != "Có thiết kế" && edge.Edge.Label != "Không thiết kế" {
			continue
		}
		found[edge.Edge.Label] = true
		attached := pipelinePDFDiagramLabelTouchesRoute(placement.Rect, edge.Route) ||
			(placement.HasLeader && pipelinePDFDiagramRouteContainsPoint(edge.Route, placement.Anchor))
		if !attached {
			t.Fatalf("branch label %q drifted away from its connector: placement=%#v route=%#v", edge.Edge.Label, placement, edge.Route)
		}
	}
	if !found["Có thiết kế"] || !found["Không thiết kế"] {
		t.Fatalf("missing official branch labels: %#v", found)
	}
}

func pipelinePDFDiagramLabelTouchesRoute(rect pipelinePDFDiagramRect, route []pipelinePDFDiagramPoint) bool {
	for index := 1; index < len(route); index++ {
		if pipelinePDFDiagramSegmentIntersectsRect(route[index-1], route[index], pipelinePDFDiagramExpandRect(rect, 0.2)) {
			return true
		}
	}
	return false
}

func pipelinePDFDiagramRouteContainsPoint(route []pipelinePDFDiagramPoint, point pipelinePDFDiagramPoint) bool {
	for index := 1; index < len(route); index++ {
		start, end := route[index-1], route[index]
		if pipelinePDFDiagramClose(start.X, end.X) && pipelinePDFDiagramClose(point.X, start.X) && point.Y >= math.Min(start.Y, end.Y) && point.Y <= math.Max(start.Y, end.Y) {
			return true
		}
		if pipelinePDFDiagramClose(start.Y, end.Y) && pipelinePDFDiagramClose(point.Y, start.Y) && point.X >= math.Min(start.X, end.X) && point.X <= math.Max(start.X, end.X) {
			return true
		}
	}
	return false
}

func pipelinePDFOfficialBranchTestDocument() pipelineTemplatePDFDocument {
	nodes := make([]pipelineTemplatePDFNode, 15)
	for index := range nodes {
		nodes[index] = pipelineTemplatePDFNode{ID: fmt.Sprintf("node-%d", index+1), Order: index + 1, Name: fmt.Sprintf("Node %d", index+1), DiagramType: "Công việc"}
	}
	edge := func(id, from, to, label string, priority int) pipelineTemplatePDFEdge {
		return pipelineTemplatePDFEdge{ID: id, FromNodeID: from, ToNodeID: to, Label: label, Priority: priority}
	}
	return pipelineTemplatePDFDocument{
		Nodes: nodes,
		Edges: []pipelineTemplatePDFEdge{
			edge("edge-1", "node-1", "node-2", "Cần khảo sát", 10),
			edge("edge-4", "node-3", "node-4", "Có thiết kế", 10),
			edge("edge-2", "node-1", "node-5", "Chủ đầu tư có BOQ", 20),
			edge("edge-5", "node-3", "node-5", "Không thiết kế", 20),
			edge("edge-3", "node-2", "node-3", "Hoàn tất khảo sát", 100),
			edge("edge-6", "node-4", "node-6", "Hoàn tất thiết kế", 100),
			edge("edge-7", "node-5", "node-7", "BOQ đã gửi", 100),
			edge("edge-8", "node-6", "node-7", "BOQ đã gửi", 100),
			edge("edge-9", "node-7", "node-8", "Hợp đồng đầu ra đã đủ điều kiện", 100),
			edge("edge-10", "node-8", "node-9", "Đã bóc tách và lấy giá", 100),
			edge("edge-11", "node-9", "node-10", "Đã chốt giá", 100),
			edge("edge-12", "node-10", "node-11", "Hồ sơ đầu vào hoàn tất", 100),
			edge("edge-13", "node-11", "node-12", "Bàn giao thi công", 100),
			edge("edge-14", "node-12", "node-13", "Thi công hoàn tất", 100),
			edge("edge-15", "node-13", "node-14", "Nghiệm thu đầu vào hoàn tất", 100),
			edge("edge-16", "node-14", "node-15", "Nghiệm thu đầu ra hoàn tất", 100),
		},
	}
}

func pipelinePDFDiagramLabeledEdgeCount(layout pipelinePDFDiagramLayout) int {
	count := 0
	for _, edge := range layout.Edges {
		if edge.HasLabelPoint {
			count++
		}
	}
	return count
}

func pipelinePDFTopologyTestDocument() pipelineTemplatePDFDocument {
	nodes := make([]pipelineTemplatePDFNode, 13)
	for index := range nodes {
		nodes[index] = pipelineTemplatePDFNode{ID: fmt.Sprintf("node-%d", index+1), Order: index + 1, Name: fmt.Sprintf("Node %d", index+1), DiagramType: "Công việc"}
	}
	edge := func(index int, from, to string) pipelineTemplatePDFEdge {
		return pipelineTemplatePDFEdge{ID: fmt.Sprintf("edge-%d", index), FromNodeID: from, ToNodeID: to, Label: fmt.Sprintf("Chuyển tiếp %d", index)}
	}
	return pipelineTemplatePDFDocument{
		Nodes: nodes,
		Edges: []pipelineTemplatePDFEdge{
			edge(1, "node-1", "node-2"),
			edge(2, "node-2", "node-3"),
			edge(3, "node-2", "node-4"),
			edge(4, "node-3", "node-5"),
			edge(5, "node-4", "node-6"),
			edge(6, "node-5", "node-6"),
			edge(7, "node-6", "node-7"),
			edge(8, "node-7", "node-8"),
			edge(9, "node-8", "node-9"),
			edge(10, "node-9", "node-10"),
			edge(11, "node-10", "node-11"),
			edge(12, "node-11", "node-12"),
			edge(13, "node-12", "node-13"),
		},
	}
}

func TestGeneratePipelineTemplatePDFProducesUnicodePDF(t *testing.T) {
	template := Record{
		"name":        "Hoàn thiện nội thất tiêu chuẩn",
		"description": "Mô tả có dấu tiếng Việt",
		"revision_no": 3,
		"status":      "draft",
		"nodes": []Record{
			{
				"order_index": 1,
				"name":        "Nghiệm thu đầu ra",
				"description": "Kiểm tra chất lượng công trình",
				"type":        "acceptance",
				"permissions": []any{"pipeline.view"},
				"approval_policy_json": Record{
					"mode":                     "any",
					"roles":                    []any{"QA/QC"},
					"quorum":                   1,
					"comment_required":         true,
					"requester_cannot_approve": true,
				},
				"steps": []Record{{
					"order_index":       1,
					"name":              "Lập biên bản nghiệm thu",
					"step_type":         "acceptance_check",
					"required_role_ids": []any{"QA/QC"},
				}},
			},
		},
	}

	data, err := GeneratePipelineTemplatePDF(template, map[string]string{"pipeline.view": "Xem workflow"}, time.Date(2026, time.July, 17, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("generate PDF: %v", err)
	}
	if len(data) < 1_000 || !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Fatalf("expected a non-empty PDF, got %d bytes and prefix %q", len(data), data[:min(len(data), 5)])
	}
	if !strings.HasSuffix(pipelineTemplatePDFFilename(template), "-rev-3.pdf") {
		t.Fatalf("unexpected filename: %q", pipelineTemplatePDFFilename(template))
	}
}

func TestPipelineTemplatePDFAwarenessLabelsDescribeDefinitionOnly(t *testing.T) {
	document := pipelineTemplatePDFDocument{
		Revision:    4,
		Status:      "Đã phát hành",
		Nodes:       []pipelineTemplatePDFNode{{}, {}},
		GeneratedAt: time.Date(2026, time.August, 5, 14, 30, 0, 0, time.FixedZone("ICT", 7*60*60)),
	}
	metadata := pipelinePDFDefinitionMetadata(document)
	if !strings.Contains(metadata, "Revision định nghĩa: 4") || !strings.Contains(metadata, "Trạng thái định nghĩa: Đã phát hành") {
		t.Fatalf("definition metadata is ambiguous: %q", metadata)
	}
	if !strings.Contains(pipelinePDFStepSemanticsNote, "Step Thi công được thực hiện lặp lại") {
		t.Fatalf("construction step semantics note is missing: %q", pipelinePDFStepSemanticsNote)
	}
	for _, forbidden := range []string{"UAT", "runtime", "kiểm thử trình duyệt"} {
		if strings.Contains(metadata, forbidden) || strings.Contains(pipelinePDFStepSemanticsNote, forbidden) {
			t.Fatalf("workflow definition report leaked excluded runtime/UAT content %q", forbidden)
		}
	}
}
