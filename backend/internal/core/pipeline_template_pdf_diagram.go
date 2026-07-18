package core

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"codeberg.org/go-pdf/fpdf"
)

const (
	pipelinePDFDiagramPadding   = 4.0
	pipelinePDFDiagramRouteExit = 2.5
)

var pipelinePDFDiagramSides = map[string]bool{
	"top":    true,
	"right":  true,
	"bottom": true,
	"left":   true,
}

type pipelinePDFDiagramStyle struct {
	NodeWidth  float64
	NodeHeight float64
	ColumnGap  float64
	LaneGap    float64
	BandGap    float64
	MaxScale   float64
	MaxColumns int
}

var pipelinePDFDiagramDocumentStyle = pipelinePDFDiagramStyle{
	NodeWidth:  190,
	NodeHeight: 72,
	ColumnGap:  95,
	LaneGap:    34,
	BandGap:    95,
	MaxScale:   0.36,
	MaxColumns: 5,
}

type pipelinePDFDiagramPoint struct {
	X float64
	Y float64
}

type pipelinePDFDiagramRect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

type pipelinePDFDiagramSlot struct {
	Position pipelinePDFDiagramPoint
	Rank     int
	Band     int
	Column   int
	Lane     int
}

type pipelinePDFDiagramNodeLayout struct {
	Node   pipelineTemplatePDFNode
	Rect   pipelinePDFDiagramRect
	Rank   int
	Band   int
	Column int
	Lane   int
}

type pipelinePDFDiagramEdgeLayout struct {
	Edge          pipelineTemplatePDFEdge
	SourceSide    string
	TargetSide    string
	Route         []pipelinePDFDiagramPoint
	LabelPoint    pipelinePDFDiagramPoint
	HasLabelPoint bool
}

type pipelinePDFDiagramLayout struct {
	Mode           string
	FallbackReason string
	Topology       bool
	Nodes          []pipelinePDFDiagramNodeLayout
	Edges          []pipelinePDFDiagramEdgeLayout
	DanglingEdges  int
}

type pipelinePDFDiagramLabelPlacement struct {
	Lines []string
	Rect  pipelinePDFDiagramRect
}

func drawPipelinePDFWorkflowDiagram(pdf *fpdf.Fpdf, document pipelineTemplatePDFDocument) {
	if len(document.Nodes) == 0 {
		drawPipelinePDFEmptyState(pdf, "Workflow chưa có node để dựng sơ đồ.")
		return
	}
	drawPipelinePDFDiagramPage(pdf, document, pipelinePDFDiagramDocumentStyle)
}

func drawPipelinePDFDiagramPage(pdf *fpdf.Fpdf, document pipelineTemplatePDFDocument, style pipelinePDFDiagramStyle) {
	availableHeight := pipelinePDFPageBottom - pdf.GetY() - 18
	if availableHeight < 30 {
		availableHeight = 30
	}
	layout := buildPipelinePDFDiagramLayoutWithStyle(document, 277, availableHeight, style)

	pdf.SetFont(pipelinePDFFontFamily, "", 7)
	pdf.SetTextColor(71, 85, 105)
	pdf.CellFormat(0, 5, fmt.Sprintf("Bố cục: %s | %d node | %d connector", layout.Mode, len(layout.Nodes), len(layout.Edges)), "", 1, "L", false, 0, "")
	for _, warning := range pipelinePDFDiagramWarnings(document, layout) {
		pdf.SetFont(pipelinePDFFontFamily, "", 6.6)
		pdf.SetTextColor(180, 83, 9)
		pdf.CellFormat(0, 4.2, warning, "", 1, "L", false, 0, "")
	}
	pdf.SetTextColor(15, 23, 42)

	chartHeight := pipelinePDFPageBottom - pdf.GetY()
	if chartHeight < 30 {
		chartHeight = 30
	}
	layout = buildPipelinePDFDiagramLayoutWithStyle(document, 277, chartHeight, style)
	chartX, chartY := 10.0, pdf.GetY()
	drawPipelinePDFDiagramCanvas(pdf, layout, chartX, chartY, 277, chartHeight)
	pdf.SetXY(10, chartY+chartHeight)
}

func pipelinePDFDiagramWarnings(document pipelineTemplatePDFDocument, layout pipelinePDFDiagramLayout) []string {
	warnings := []string{}
	if len(document.Edges) == 0 {
		warnings = append(warnings, "Workflow chưa có connector; node được hiển thị theo thứ tự cấu hình.")
	} else if layout.DanglingEdges > 0 {
		warnings = append(warnings, fmt.Sprintf("%d connector không thể hiển thị vì thiếu node nguồn hoặc node đích.", layout.DanglingEdges))
	}
	if layout.FallbackReason == "cycle" {
		warnings = append(warnings, "Không thể phân tầng topology vì graph có cycle; sơ đồ dùng thứ tự node ổn định.")
	}
	return warnings
}

func buildPipelinePDFDiagramLayout(document pipelineTemplatePDFDocument, width, height float64) pipelinePDFDiagramLayout {
	return buildPipelinePDFDiagramLayoutWithStyle(document, width, height, pipelinePDFDiagramDocumentStyle)
}

func buildPipelinePDFDiagramLayoutWithStyle(document pipelineTemplatePDFDocument, width, height float64, style pipelinePDFDiagramStyle) pipelinePDFDiagramLayout {
	slots, topology, fallbackReason := pipelinePDFDiagramTopologySlots(document.Nodes, document.Edges, style)
	if !topology {
		slots = pipelinePDFDiagramFallbackSlots(document.Nodes, style)
	}

	bounds := pipelinePDFDiagramWorldBounds(document.Nodes, slots, style)
	scale := pipelinePDFDiagramScale(bounds, width, height, style.MaxScale)
	contentWidth := bounds.Width * scale
	contentHeight := bounds.Height * scale
	offsetX := pipelinePDFDiagramPadding + (width-pipelinePDFDiagramPadding*2-contentWidth)/2 - bounds.X*scale
	offsetY := pipelinePDFDiagramPadding + (height-pipelinePDFDiagramPadding*2-contentHeight)/2 - bounds.Y*scale

	layout := pipelinePDFDiagramLayout{
		Mode:           "Theo luồng workflow",
		FallbackReason: fallbackReason,
		Topology:       topology,
	}
	if fallbackReason == "no_edges" {
		layout.Mode = "Theo thứ tự node"
	} else if fallbackReason == "cycle" {
		layout.Mode = "Theo thứ tự node (graph có cycle)"
	}

	nodeByID := make(map[string]pipelinePDFDiagramNodeLayout, len(document.Nodes))
	for _, node := range document.Nodes {
		slot := slots[node.ID]
		nodeLayout := pipelinePDFDiagramNodeLayout{
			Node:   node,
			Rank:   slot.Rank,
			Band:   slot.Band,
			Column: slot.Column,
			Lane:   slot.Lane,
			Rect: pipelinePDFDiagramRect{
				X:      slot.Position.X*scale + offsetX,
				Y:      slot.Position.Y*scale + offsetY,
				Width:  style.NodeWidth * scale,
				Height: style.NodeHeight * scale,
			},
		}
		nodeByID[node.ID] = nodeLayout
		layout.Nodes = append(layout.Nodes, nodeLayout)
	}

	bandTop, bandBottom := pipelinePDFDiagramBandBounds(layout.Nodes)
	for _, edge := range document.Edges {
		source, sourceOK := nodeByID[edge.FromNodeID]
		target, targetOK := nodeByID[edge.ToNodeID]
		if !sourceOK || !targetOK {
			layout.DanglingEdges++
			continue
		}

		sourceSide, targetSide := pipelinePDFDiagramAutomaticEdgeSides(source.Rect, target.Rect)
		var route []pipelinePDFDiagramPoint
		if topology && target.Band > source.Band {
			sourceSide, targetSide = "bottom", "top"
			route = pipelinePDFDiagramBandWrapRoute(source.Rect, target.Rect, bandBottom[source.Band], bandTop[target.Band], width)
		} else {
			rectByID := make(map[string]pipelinePDFDiagramRect, len(nodeByID))
			for nodeID, node := range nodeByID {
				rectByID[nodeID] = node.Rect
			}
			route = pipelinePDFDiagramRoute(source.Rect, target.Rect, sourceSide, targetSide, rectByID, edge.FromNodeID, edge.ToNodeID, width, height)
		}

		edgeLayout := pipelinePDFDiagramEdgeLayout{
			Edge:       edge,
			SourceSide: sourceSide,
			TargetSide: targetSide,
			Route:      route,
		}
		if strings.TrimSpace(edge.Label) != "" && len(route) >= 2 {
			edgeLayout.LabelPoint = pipelinePDFDiagramRouteLabelPoint(route)
			edgeLayout.HasLabelPoint = true
		}
		layout.Edges = append(layout.Edges, edgeLayout)
	}
	return layout
}

func pipelinePDFDiagramTopologySlots(nodes []pipelineTemplatePDFNode, edges []pipelineTemplatePDFEdge, style pipelinePDFDiagramStyle) (map[string]pipelinePDFDiagramSlot, bool, string) {
	if len(nodes) == 0 {
		return map[string]pipelinePDFDiagramSlot{}, true, ""
	}
	nodeByID := make(map[string]pipelineTemplatePDFNode, len(nodes))
	indegree := make(map[string]int, len(nodes))
	adjacency := make(map[string][]string, len(nodes))
	for _, node := range nodes {
		nodeByID[node.ID] = node
		indegree[node.ID] = 0
	}
	validEdges := 0
	for _, edge := range edges {
		if _, sourceOK := nodeByID[edge.FromNodeID]; !sourceOK {
			continue
		}
		if _, targetOK := nodeByID[edge.ToNodeID]; !targetOK {
			continue
		}
		adjacency[edge.FromNodeID] = append(adjacency[edge.FromNodeID], edge.ToNodeID)
		indegree[edge.ToNodeID]++
		validEdges++
	}
	if validEdges == 0 {
		return nil, false, "no_edges"
	}

	queue := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if indegree[node.ID] == 0 {
			queue = append(queue, node.ID)
		}
	}
	pipelinePDFDiagramSortNodeIDs(queue, nodeByID)
	ranks := make(map[string]int, len(nodes))
	processed := 0
	for len(queue) > 0 {
		nodeID := queue[0]
		queue = queue[1:]
		processed++
		targets := append([]string(nil), adjacency[nodeID]...)
		pipelinePDFDiagramSortNodeIDs(targets, nodeByID)
		for _, targetID := range targets {
			if ranks[targetID] < ranks[nodeID]+1 {
				ranks[targetID] = ranks[nodeID] + 1
			}
			indegree[targetID]--
			if indegree[targetID] == 0 {
				queue = append(queue, targetID)
				pipelinePDFDiagramSortNodeIDs(queue, nodeByID)
			}
		}
	}
	if processed != len(nodes) {
		return nil, false, "cycle"
	}

	rankNodes := map[int][]pipelineTemplatePDFNode{}
	maxRank := 0
	for _, node := range nodes {
		rank := ranks[node.ID]
		rankNodes[rank] = append(rankNodes[rank], node)
		if rank > maxRank {
			maxRank = rank
		}
	}
	for rank := range rankNodes {
		sort.SliceStable(rankNodes[rank], func(i, j int) bool {
			return pipelinePDFDiagramNodeLess(rankNodes[rank][i], rankNodes[rank][j])
		})
	}

	bandLaneCounts := map[int]int{}
	for rank := 0; rank <= maxRank; rank++ {
		band := rank / style.MaxColumns
		if len(rankNodes[rank]) > bandLaneCounts[band] {
			bandLaneCounts[band] = len(rankNodes[rank])
		}
	}
	maxBand := maxRank / style.MaxColumns
	bandY := make(map[int]float64, maxBand+1)
	bandHeights := make(map[int]float64, maxBand+1)
	cursorY := 0.0
	for band := 0; band <= maxBand; band++ {
		lanes := bandLaneCounts[band]
		if lanes < 1 {
			lanes = 1
		}
		height := float64(lanes)*style.NodeHeight + float64(lanes-1)*style.LaneGap
		bandY[band] = cursorY
		bandHeights[band] = height
		cursorY += height + style.BandGap
	}

	slots := make(map[string]pipelinePDFDiagramSlot, len(nodes))
	for rank := 0; rank <= maxRank; rank++ {
		items := rankNodes[rank]
		if len(items) == 0 {
			continue
		}
		band := rank / style.MaxColumns
		column := rank % style.MaxColumns
		stackHeight := float64(len(items))*style.NodeHeight + float64(len(items)-1)*style.LaneGap
		startY := bandY[band] + (bandHeights[band]-stackHeight)/2
		for lane, node := range items {
			slots[node.ID] = pipelinePDFDiagramSlot{
				Position: pipelinePDFDiagramPoint{
					X: float64(column) * (style.NodeWidth + style.ColumnGap),
					Y: startY + float64(lane)*(style.NodeHeight+style.LaneGap),
				},
				Rank:   rank,
				Band:   band,
				Column: column,
				Lane:   lane,
			}
		}
	}
	return slots, true, ""
}

func pipelinePDFDiagramFallbackSlots(nodes []pipelineTemplatePDFNode, style pipelinePDFDiagramStyle) map[string]pipelinePDFDiagramSlot {
	columns := style.MaxColumns
	if len(nodes) < columns {
		columns = len(nodes)
	}
	if columns < 1 {
		columns = 1
	}
	slots := make(map[string]pipelinePDFDiagramSlot, len(nodes))
	for index, node := range nodes {
		column := index % columns
		band := index / columns
		slots[node.ID] = pipelinePDFDiagramSlot{
			Position: pipelinePDFDiagramPoint{
				X: float64(column) * (style.NodeWidth + style.ColumnGap),
				Y: float64(band) * (style.NodeHeight + style.BandGap),
			},
			Rank:   index,
			Band:   band,
			Column: column,
		}
	}
	return slots
}

func pipelinePDFDiagramSortNodeIDs(ids []string, nodeByID map[string]pipelineTemplatePDFNode) {
	sort.SliceStable(ids, func(i, j int) bool {
		return pipelinePDFDiagramNodeLess(nodeByID[ids[i]], nodeByID[ids[j]])
	})
}

func pipelinePDFDiagramNodeLess(left, right pipelineTemplatePDFNode) bool {
	if left.Order != right.Order {
		return left.Order < right.Order
	}
	return left.ID < right.ID
}

func pipelinePDFDiagramWorldBounds(nodes []pipelineTemplatePDFNode, slots map[string]pipelinePDFDiagramSlot, style pipelinePDFDiagramStyle) pipelinePDFDiagramRect {
	if len(nodes) == 0 {
		return pipelinePDFDiagramRect{Width: 1, Height: 1}
	}
	first := slots[nodes[0].ID].Position
	minX, minY := first.X, first.Y
	maxX := first.X + style.NodeWidth
	maxY := first.Y + style.NodeHeight
	for _, node := range nodes[1:] {
		position := slots[node.ID].Position
		minX = math.Min(minX, position.X)
		minY = math.Min(minY, position.Y)
		maxX = math.Max(maxX, position.X+style.NodeWidth)
		maxY = math.Max(maxY, position.Y+style.NodeHeight)
	}
	return pipelinePDFDiagramRect{X: minX, Y: minY, Width: math.Max(1, maxX-minX), Height: math.Max(1, maxY-minY)}
}

func pipelinePDFDiagramScale(bounds pipelinePDFDiagramRect, width, height, maxScale float64) float64 {
	scale := math.Min((width-pipelinePDFDiagramPadding*2)/bounds.Width, (height-pipelinePDFDiagramPadding*2)/bounds.Height)
	if scale > maxScale {
		return maxScale
	}
	return math.Max(scale, 0.001)
}

func pipelinePDFDiagramBandBounds(nodes []pipelinePDFDiagramNodeLayout) (map[int]float64, map[int]float64) {
	tops := map[int]float64{}
	bottoms := map[int]float64{}
	for _, node := range nodes {
		if top, exists := tops[node.Band]; !exists || node.Rect.Y < top {
			tops[node.Band] = node.Rect.Y
		}
		bottom := node.Rect.Y + node.Rect.Height
		if existing, exists := bottoms[node.Band]; !exists || bottom > existing {
			bottoms[node.Band] = bottom
		}
	}
	return tops, bottoms
}

func pipelinePDFDiagramAutomaticEdgeSides(source, target pipelinePDFDiagramRect) (string, string) {
	sourceCenter := pipelinePDFDiagramRectCenter(source)
	targetCenter := pipelinePDFDiagramRectCenter(target)
	dx, dy := targetCenter.X-sourceCenter.X, targetCenter.Y-sourceCenter.Y
	if math.Abs(dx) >= math.Abs(dy) {
		if dx >= 0 {
			return "right", "left"
		}
		return "left", "right"
	}
	if dy >= 0 {
		return "bottom", "top"
	}
	return "top", "bottom"
}

func pipelinePDFDiagramBandWrapRoute(source, target pipelinePDFDiagramRect, sourceBandBottom, targetBandTop, width float64) []pipelinePDFDiagramPoint {
	sourceBoundary := pipelinePDFDiagramPortPoint(source, "bottom")
	targetBoundary := pipelinePDFDiagramPortPoint(target, "top")
	gap := math.Max(4, targetBandTop-sourceBandBottom)
	laneOffset := math.Min(7, gap/3)
	sourceLaneY := sourceBandBottom + laneOffset
	targetLaneY := targetBandTop - laneOffset
	rightGutter := width - 1.8
	return pipelinePDFDiagramCompactRoute([]pipelinePDFDiagramPoint{
		sourceBoundary,
		{X: sourceBoundary.X, Y: sourceLaneY},
		{X: rightGutter, Y: sourceLaneY},
		{X: rightGutter, Y: targetLaneY},
		{X: targetBoundary.X, Y: targetLaneY},
		targetBoundary,
	})
}

func pipelinePDFDiagramRoute(
	sourceRect, targetRect pipelinePDFDiagramRect,
	sourceSide, targetSide string,
	rectByID map[string]pipelinePDFDiagramRect,
	sourceID, targetID string,
	width, height float64,
) []pipelinePDFDiagramPoint {
	sourceBoundary := pipelinePDFDiagramPortPoint(sourceRect, sourceSide)
	targetBoundary := pipelinePDFDiagramPortPoint(targetRect, targetSide)
	sourceExit := pipelinePDFDiagramOffsetPoint(sourceBoundary, sourceSide, pipelinePDFDiagramRouteExit)
	targetExit := pipelinePDFDiagramOffsetPoint(targetBoundary, targetSide, pipelinePDFDiagramRouteExit)

	minX, minY := width, height
	maxX, maxY := 0.0, 0.0
	for _, rect := range rectByID {
		minX = math.Min(minX, rect.X)
		minY = math.Min(minY, rect.Y)
		maxX = math.Max(maxX, rect.X+rect.Width)
		maxY = math.Max(maxY, rect.Y+rect.Height)
	}
	detourGap := 3.0
	top := math.Max(1.5, minY-detourGap)
	bottom := math.Min(height-1.5, maxY+detourGap)
	left := math.Max(1.5, minX-detourGap)
	right := math.Min(width-1.5, maxX+detourGap)

	candidates := [][]pipelinePDFDiagramPoint{
		{sourceExit, {X: targetExit.X, Y: sourceExit.Y}, targetExit},
		{sourceExit, {X: sourceExit.X, Y: targetExit.Y}, targetExit},
		{sourceExit, {X: sourceExit.X, Y: top}, {X: targetExit.X, Y: top}, targetExit},
		{sourceExit, {X: sourceExit.X, Y: bottom}, {X: targetExit.X, Y: bottom}, targetExit},
		{sourceExit, {X: left, Y: sourceExit.Y}, {X: left, Y: targetExit.Y}, targetExit},
		{sourceExit, {X: right, Y: sourceExit.Y}, {X: right, Y: targetExit.Y}, targetExit},
	}
	best := candidates[0]
	bestScore := math.MaxFloat64
	for _, candidate := range candidates {
		candidate = pipelinePDFDiagramCompactRoute(candidate)
		score := pipelinePDFDiagramRouteScore(candidate, rectByID, sourceID, targetID)
		if score < bestScore {
			best = candidate
			bestScore = score
		}
	}
	route := append([]pipelinePDFDiagramPoint{sourceBoundary}, best...)
	route = append(route, targetBoundary)
	return pipelinePDFDiagramCompactRoute(route)
}

func pipelinePDFDiagramRouteScore(route []pipelinePDFDiagramPoint, rectByID map[string]pipelinePDFDiagramRect, sourceID, targetID string) float64 {
	score := 0.0
	for index := 1; index < len(route); index++ {
		start, end := route[index-1], route[index]
		score += math.Abs(end.X-start.X) + math.Abs(end.Y-start.Y)
		for nodeID, rect := range rectByID {
			if nodeID == sourceID || nodeID == targetID {
				continue
			}
			if pipelinePDFDiagramSegmentIntersectsRect(start, end, pipelinePDFDiagramExpandRect(rect, 0.8)) {
				score += 10_000
			}
		}
	}
	if len(route) > 2 {
		score += float64(len(route)-2) * 2
	}
	return score
}

func drawPipelinePDFDiagramCanvas(pdf *fpdf.Fpdf, layout pipelinePDFDiagramLayout, x, y, width, height float64) {
	pdf.SetDrawColor(203, 213, 225)
	pdf.SetFillColor(248, 250, 252)
	pdf.SetLineWidth(0.25)
	pdf.RoundedRect(x, y, width, height, 2, "1234", "DF")

	pdf.SetDrawColor(100, 116, 139)
	pdf.SetFillColor(100, 116, 139)
	pdf.SetLineWidth(0.45)
	for _, edge := range layout.Edges {
		for index := 1; index < len(edge.Route); index++ {
			start, end := edge.Route[index-1], edge.Route[index]
			pdf.Line(x+start.X, y+start.Y, x+end.X, y+end.Y)
		}
		drawPipelinePDFDiagramArrow(pdf, edge.Route, x, y)
	}

	labelPlacements := make([]pipelinePDFDiagramLabelPlacement, 0, len(layout.Edges))
	for _, edge := range layout.Edges {
		if !edge.HasLabelPoint {
			continue
		}
		if placement, ok := pipelinePDFDiagramEdgeLabelPlacement(pdf, edge.Edge.Label, edge.Route, layout.Nodes, labelPlacements, width, height); ok {
			labelPlacements = append(labelPlacements, placement)
		}
	}
	for _, node := range layout.Nodes {
		drawPipelinePDFDiagramNode(pdf, node, x, y)
	}
	for _, placement := range labelPlacements {
		drawPipelinePDFDiagramEdgeLabel(pdf, placement, x, y)
	}
	pdf.SetLineWidth(0.2)
}

func drawPipelinePDFDiagramArrow(pdf *fpdf.Fpdf, route []pipelinePDFDiagramPoint, offsetX, offsetY float64) {
	if len(route) < 2 {
		return
	}
	end := route[len(route)-1]
	previous := route[len(route)-2]
	angle := math.Atan2(end.Y-previous.Y, end.X-previous.X)
	length, halfWidth := 2.3, 1.15
	baseX := end.X - math.Cos(angle)*length
	baseY := end.Y - math.Sin(angle)*length
	left := fpdf.PointType{X: offsetX + baseX + math.Sin(angle)*halfWidth, Y: offsetY + baseY - math.Cos(angle)*halfWidth}
	right := fpdf.PointType{X: offsetX + baseX - math.Sin(angle)*halfWidth, Y: offsetY + baseY + math.Cos(angle)*halfWidth}
	pdf.Polygon([]fpdf.PointType{{X: offsetX + end.X, Y: offsetY + end.Y}, left, right}, "F")
}

func drawPipelinePDFDiagramEdgeLabel(pdf *fpdf.Fpdf, placement pipelinePDFDiagramLabelPlacement, offsetX, offsetY float64) {
	x, y := offsetX+placement.Rect.X, offsetY+placement.Rect.Y
	pdf.SetFillColor(255, 255, 255)
	pdf.SetDrawColor(226, 232, 240)
	pdf.RoundedRect(x, y, placement.Rect.Width, placement.Rect.Height, 1, "1234", "DF")
	pdf.SetFont(pipelinePDFFontFamily, "B", 5.1)
	pdf.SetTextColor(71, 85, 105)
	lineHeight := 3.0
	startY := y + (placement.Rect.Height-float64(len(placement.Lines))*lineHeight)/2
	for index, line := range placement.Lines {
		pdf.SetXY(x, startY+float64(index)*lineHeight)
		pdf.CellFormat(placement.Rect.Width, lineHeight, line, "", 0, "C", false, 0, "")
	}
	pdf.SetTextColor(15, 23, 42)
}

func pipelinePDFDiagramEdgeLabelPlacement(
	pdf *fpdf.Fpdf,
	label string,
	route []pipelinePDFDiagramPoint,
	nodes []pipelinePDFDiagramNodeLayout,
	used []pipelinePDFDiagramLabelPlacement,
	canvasWidth, canvasHeight float64,
) (pipelinePDFDiagramLabelPlacement, bool) {
	label = strings.TrimSpace(label)
	if label == "" || len(route) < 2 {
		return pipelinePDFDiagramLabelPlacement{}, false
	}
	pdf.SetFont(pipelinePDFFontFamily, "B", 5.1)
	// Prefer a compact two-line pill before a wide one-line pill. This keeps the
	// label visually attached to its connector instead of forcing it into a
	// distant free lane between node cards.
	widthLimits := []float64{18, 24, 30, 36, 42}
	for _, allowTruncation := range []bool{false, true} {
		for _, widthLimit := range widthLimits {
			lines := pdf.SplitText(label, widthLimit-3.2)
			if len(lines) == 0 {
				continue
			}
			if !allowTruncation && len(lines) > 2 {
				continue
			}
			if len(lines) > 2 {
				lines = append([]string(nil), lines[:2]...)
				lines[1] = pipelinePDFDiagramFitText(pdf, strings.TrimSpace(lines[1])+"...", widthLimit-3.2)
			}
			labelWidth := 0.0
			for _, line := range lines {
				labelWidth = math.Max(labelWidth, pdf.GetStringWidth(line))
			}
			labelWidth = math.Min(widthLimit, labelWidth+3.2)
			labelHeight := float64(len(lines))*3 + 1.6
			for _, center := range pipelinePDFDiagramLabelCandidates(route) {
				rect := pipelinePDFDiagramRect{X: center.X - labelWidth/2, Y: center.Y - labelHeight/2, Width: labelWidth, Height: labelHeight}
				if rect.X < 1 || rect.Y < 1 || rect.X+rect.Width > canvasWidth-1 || rect.Y+rect.Height > canvasHeight-1 {
					continue
				}
				if pipelinePDFDiagramLabelOverlapsNodes(rect, nodes) || pipelinePDFDiagramLabelOverlapsLabels(rect, used) {
					continue
				}
				return pipelinePDFDiagramLabelPlacement{Lines: lines, Rect: rect}, true
			}
		}
	}

	lines := []string{pipelinePDFDiagramFitText(pdf, label, 8)}
	labelWidth := math.Min(11.2, pdf.GetStringWidth(lines[0])+3.2)
	center := pipelinePDFDiagramRouteLabelPoint(route)
	center.X = math.Max(1+labelWidth/2, math.Min(canvasWidth-1-labelWidth/2, center.X))
	center.Y = math.Max(3.3, math.Min(canvasHeight-3.3, center.Y))
	return pipelinePDFDiagramLabelPlacement{
		Lines: lines,
		Rect:  pipelinePDFDiagramRect{X: center.X - labelWidth/2, Y: center.Y - 2.3, Width: labelWidth, Height: 4.6},
	}, true
}

func pipelinePDFDiagramLabelCandidates(route []pipelinePDFDiagramPoint) []pipelinePDFDiagramPoint {
	type segment struct {
		Start, End pipelinePDFDiagramPoint
		Length     float64
	}
	segments := make([]segment, 0, len(route)-1)
	for index := 1; index < len(route); index++ {
		start, end := route[index-1], route[index]
		segments = append(segments, segment{Start: start, End: end, Length: math.Abs(end.X-start.X) + math.Abs(end.Y-start.Y)})
	}
	// Start near the target so labels on sibling branches sit on their own
	// target-specific leg instead of competing for the shared source segment.
	orderedSegments := make([]segment, 0, len(segments))
	for index := len(segments) - 1; index >= 0; index-- {
		if segments[index].Length >= 6 {
			orderedSegments = append(orderedSegments, segments[index])
		}
	}
	for index := len(segments) - 1; index >= 0; index-- {
		if segments[index].Length < 6 {
			orderedSegments = append(orderedSegments, segments[index])
		}
	}

	candidates := make([]pipelinePDFDiagramPoint, 0, len(segments)*27)
	for _, segment := range orderedSegments {
		for _, fraction := range []float64{0.5, 0.35, 0.65} {
			base := pipelinePDFDiagramPoint{
				X: segment.Start.X + (segment.End.X-segment.Start.X)*fraction,
				Y: segment.Start.Y + (segment.End.Y-segment.Start.Y)*fraction,
			}
			if pipelinePDFDiagramClose(segment.Start.Y, segment.End.Y) {
				for _, offset := range []float64{0, -5, 5, -9, 9, -13, 13, -17, 17} {
					candidates = append(candidates, pipelinePDFDiagramPoint{X: base.X, Y: base.Y + offset})
				}
				continue
			}
			for _, offset := range []float64{0, -5, 5, -9, 9, -13, 13, -17, 17} {
				candidates = append(candidates, pipelinePDFDiagramPoint{X: base.X + offset, Y: base.Y})
			}
		}
	}
	return candidates
}

func pipelinePDFDiagramLabelOverlapsNodes(rect pipelinePDFDiagramRect, nodes []pipelinePDFDiagramNodeLayout) bool {
	for _, node := range nodes {
		if pipelinePDFDiagramRectsOverlap(rect, pipelinePDFDiagramExpandRect(node.Rect, 0.8)) {
			return true
		}
	}
	return false
}

func pipelinePDFDiagramLabelOverlapsLabels(rect pipelinePDFDiagramRect, labels []pipelinePDFDiagramLabelPlacement) bool {
	for _, label := range labels {
		if pipelinePDFDiagramRectsOverlap(rect, pipelinePDFDiagramExpandRect(label.Rect, 0.8)) {
			return true
		}
	}
	return false
}

func pipelinePDFDiagramRectsOverlap(left, right pipelinePDFDiagramRect) bool {
	return left.X < right.X+right.Width && left.X+left.Width > right.X && left.Y < right.Y+right.Height && left.Y+left.Height > right.Y
}

func drawPipelinePDFDiagramNode(pdf *fpdf.Fpdf, layout pipelinePDFDiagramNodeLayout, offsetX, offsetY float64) {
	rect := layout.Rect
	x, y := offsetX+rect.X, offsetY+rect.Y
	drawPipelinePDFDiagramNodeCard(pdf, x, y, rect)

	padding := math.Max(1.1, math.Min(1.7, rect.Width/24))
	badgeSize := math.Max(4.4, math.Min(5.8, rect.Height/2.5))
	drawPipelinePDFDiagramNodeBadge(pdf, layout.Node.Order, x+padding, y+padding, badgeSize)
	titleX := x + padding + badgeSize + 1.2
	titleWidth := math.Max(5, rect.Width-(titleX-x)-padding)
	titleSize := math.Max(4.8, math.Min(6.1, rect.Height/2.35))
	pdf.SetFont(pipelinePDFFontFamily, "B", titleSize)
	pdf.SetTextColor(15, 23, 42)
	lines := pipelinePDFDiagramTextLines(pdf, layout.Node.Name, titleWidth, 2)
	lineHeight := 2.9
	for index, line := range lines {
		pdf.Text(titleX, y+padding+2.4+float64(index)*lineHeight, line)
	}

	meta := fmt.Sprintf("%s | %d steps.", layout.Node.DiagramType, len(layout.Node.Steps))
	pdf.SetFont(pipelinePDFFontFamily, "", math.Max(4.4, math.Min(5.3, rect.Height/3)))
	pdf.SetTextColor(37, 99, 235)
	meta = pipelinePDFDiagramFitText(pdf, meta, rect.Width-padding*2)
	pdf.Text(x+padding, y+rect.Height-padding-0.4, meta)
	pdf.SetTextColor(15, 23, 42)
}

func drawPipelinePDFDiagramNodeCard(pdf *fpdf.Fpdf, x, y float64, rect pipelinePDFDiagramRect) {
	pdf.SetDrawColor(148, 163, 184)
	pdf.SetFillColor(255, 255, 255)
	pdf.SetLineWidth(0.35)
	pdf.RoundedRect(x, y, rect.Width, rect.Height, math.Min(1.8, rect.Height/6), "1234", "DF")
}

func drawPipelinePDFDiagramNodeBadge(pdf *fpdf.Fpdf, order int, x, y, size float64) {
	pdf.SetFillColor(241, 245, 249)
	pdf.SetDrawColor(226, 232, 240)
	pdf.RoundedRect(x, y, size, size, 1, "1234", "DF")
	pdf.SetFont(pipelinePDFFontFamily, "B", math.Max(4.1, math.Min(5.2, size*0.85)))
	pdf.SetTextColor(51, 65, 85)
	pdf.SetXY(x, y+0.1)
	pdf.CellFormat(size, size-0.2, strconv.Itoa(order), "", 0, "C", false, 0, "")
}

func pipelinePDFDiagramTextLines(pdf *fpdf.Fpdf, value string, width float64, maxLines int) []string {
	lines := pdf.SplitText(strings.TrimSpace(value), width)
	if len(lines) == 0 {
		return []string{"-"}
	}
	if len(lines) <= maxLines {
		return lines
	}
	lines = append([]string(nil), lines[:maxLines]...)
	lines[maxLines-1] = pipelinePDFDiagramFitText(pdf, strings.TrimSpace(lines[maxLines-1])+"...", width)
	return lines
}

func pipelinePDFDiagramFitText(pdf *fpdf.Fpdf, value string, width float64) string {
	value = strings.TrimSpace(value)
	if value == "" || width <= 0 {
		return ""
	}
	if pdf.GetStringWidth(value) <= width {
		return value
	}
	runes := []rune(value)
	for len(runes) > 0 {
		candidate := strings.TrimSpace(string(runes)) + "..."
		if pdf.GetStringWidth(candidate) <= width {
			return candidate
		}
		runes = runes[:len(runes)-1]
	}
	return "..."
}

func pipelinePDFDiagramRouteLabelPoint(route []pipelinePDFDiagramPoint) pipelinePDFDiagramPoint {
	bestLength := -1.0
	point := route[0]
	for index := 1; index < len(route); index++ {
		start, end := route[index-1], route[index]
		length := math.Abs(end.X-start.X) + math.Abs(end.Y-start.Y)
		if length > bestLength {
			bestLength = length
			point = pipelinePDFDiagramPoint{X: (start.X + end.X) / 2, Y: (start.Y + end.Y) / 2}
		}
	}
	return point
}

func pipelinePDFDiagramCompactRoute(route []pipelinePDFDiagramPoint) []pipelinePDFDiagramPoint {
	out := make([]pipelinePDFDiagramPoint, 0, len(route))
	for _, point := range route {
		if len(out) > 0 && pipelinePDFDiagramSamePoint(out[len(out)-1], point) {
			continue
		}
		out = append(out, point)
	}
	if len(out) < 3 {
		return out
	}
	compacted := []pipelinePDFDiagramPoint{out[0]}
	for index := 1; index < len(out)-1; index++ {
		previous := compacted[len(compacted)-1]
		current, next := out[index], out[index+1]
		if (pipelinePDFDiagramClose(previous.X, current.X) && pipelinePDFDiagramClose(current.X, next.X)) ||
			(pipelinePDFDiagramClose(previous.Y, current.Y) && pipelinePDFDiagramClose(current.Y, next.Y)) {
			continue
		}
		compacted = append(compacted, current)
	}
	return append(compacted, out[len(out)-1])
}

func pipelinePDFDiagramSegmentIntersectsRect(start, end pipelinePDFDiagramPoint, rect pipelinePDFDiagramRect) bool {
	if pipelinePDFDiagramClose(start.X, end.X) {
		minY, maxY := math.Min(start.Y, end.Y), math.Max(start.Y, end.Y)
		return start.X >= rect.X && start.X <= rect.X+rect.Width && maxY >= rect.Y && minY <= rect.Y+rect.Height
	}
	if pipelinePDFDiagramClose(start.Y, end.Y) {
		minX, maxX := math.Min(start.X, end.X), math.Max(start.X, end.X)
		return start.Y >= rect.Y && start.Y <= rect.Y+rect.Height && maxX >= rect.X && minX <= rect.X+rect.Width
	}
	return false
}

func pipelinePDFDiagramRectCenter(rect pipelinePDFDiagramRect) pipelinePDFDiagramPoint {
	return pipelinePDFDiagramPoint{X: rect.X + rect.Width/2, Y: rect.Y + rect.Height/2}
}

func pipelinePDFDiagramPortPoint(rect pipelinePDFDiagramRect, side string) pipelinePDFDiagramPoint {
	switch side {
	case "top":
		return pipelinePDFDiagramPoint{X: rect.X + rect.Width/2, Y: rect.Y}
	case "bottom":
		return pipelinePDFDiagramPoint{X: rect.X + rect.Width/2, Y: rect.Y + rect.Height}
	case "left":
		return pipelinePDFDiagramPoint{X: rect.X, Y: rect.Y + rect.Height/2}
	default:
		return pipelinePDFDiagramPoint{X: rect.X + rect.Width, Y: rect.Y + rect.Height/2}
	}
}

func pipelinePDFDiagramOffsetPoint(point pipelinePDFDiagramPoint, side string, distance float64) pipelinePDFDiagramPoint {
	switch side {
	case "top":
		point.Y -= distance
	case "bottom":
		point.Y += distance
	case "left":
		point.X -= distance
	default:
		point.X += distance
	}
	return point
}

func pipelinePDFDiagramExpandRect(rect pipelinePDFDiagramRect, padding float64) pipelinePDFDiagramRect {
	return pipelinePDFDiagramRect{X: rect.X - padding, Y: rect.Y - padding, Width: rect.Width + padding*2, Height: rect.Height + padding*2}
}

func pipelinePDFDiagramSamePoint(left, right pipelinePDFDiagramPoint) bool {
	return pipelinePDFDiagramClose(left.X, right.X) && pipelinePDFDiagramClose(left.Y, right.Y)
}

func pipelinePDFDiagramClose(left, right float64) bool {
	return math.Abs(left-right) < 0.001
}

func pipelinePDFDiagramCoordinate(value any) (float64, bool) {
	if value == nil {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || math.Abs(parsed) > maxPipelineDiagramCoordinate {
		return 0, false
	}
	return parsed, true
}

func pipelinePDFDiagramMetadata(value any) (portMode, sourceSide, targetSide string) {
	portMode = "auto"
	config, ok := pipelinePDFDiagramRecord(value)
	if !ok {
		return portMode, "", ""
	}
	diagram, ok := pipelinePDFDiagramRecord(config["diagram"])
	if !ok {
		return portMode, "", ""
	}
	mode := strings.ToLower(strings.TrimSpace(fmt.Sprint(diagram["portMode"])))
	if mode == "<nil>" || mode == "" {
		mode = strings.ToLower(strings.TrimSpace(fmt.Sprint(diagram["port_mode"])))
	}
	if mode == "manual" {
		portMode = "manual"
	}
	source, _ := pipelinePDFDiagramRecord(firstPipelinePDFDiagramValue(diagram, "source", "sourcePort"))
	target, _ := pipelinePDFDiagramRecord(firstPipelinePDFDiagramValue(diagram, "target", "targetPort"))
	sourceSide = pipelinePDFDiagramSide(firstPipelinePDFDiagramValue(source, "side", "portSide", "port_side"))
	targetSide = pipelinePDFDiagramSide(firstPipelinePDFDiagramValue(target, "side", "portSide", "port_side"))
	return portMode, sourceSide, targetSide
}

func pipelinePDFDiagramRecord(value any) (Record, bool) {
	switch record := value.(type) {
	case Record:
		return record, true
	case map[string]any:
		return Record(record), true
	default:
		return nil, false
	}
}

func firstPipelinePDFDiagramValue(record Record, keys ...string) any {
	for _, key := range keys {
		if value, exists := record[key]; exists && value != nil {
			return value
		}
	}
	return nil
}

func pipelinePDFDiagramSide(value any) string {
	side := strings.ToLower(strings.TrimSpace(fmt.Sprint(value)))
	if pipelinePDFDiagramSides[side] {
		return side
	}
	return ""
}
