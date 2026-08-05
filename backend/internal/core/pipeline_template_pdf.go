package core

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"codeberg.org/go-pdf/fpdf"
)

const (
	pipelinePDFPageBottom = 195.0
	pipelinePDFFontFamily = "NotoSans"
	pipelinePDFMimeType   = "application/pdf"
)

const pipelinePDFStepSemanticsNote = "Mỗi step biểu diễn một nghĩa vụ nghiệp vụ. Step Thi công được thực hiện lặp lại theo từng hạng mục và ngày thi công."

//go:embed pdfassets/NotoSans-Regular.ttf
var pipelinePDFRegularFont []byte

//go:embed pdfassets/NotoSans-Bold.ttf
var pipelinePDFBoldFont []byte

type PipelineTemplatePDFResult struct {
	Data     []byte
	Filename string
	MIMEType string
}

type pipelineTemplatePDFDocument struct {
	Name               string
	Description        string
	Revision           int
	Status             string
	GeneratedAt        time.Time
	NodeReferenceTitle string
	NodeReferences     []pipelineTemplatePDFNodeReference
	Nodes              []pipelineTemplatePDFNode
	Edges              []pipelineTemplatePDFEdge
}

type pipelineTemplatePDFNode struct {
	ID                     string
	Order                  int
	BusinessCode           string
	Name                   string
	Description            string
	Type                   string
	DiagramType            string
	Permissions            string
	ApprovalRoles          string
	CommentRequired        string
	RequesterCannotApprove string
	PositionX              float64
	PositionY              float64
	HasPosition            bool
	Steps                  []pipelineTemplatePDFStep
}

type pipelineTemplatePDFEdge struct {
	ID         string
	FromNodeID string
	ToNodeID   string
	Label      string
	PortMode   string
	SourceSide string
	TargetSide string
	Priority   int
}

type pipelineTemplatePDFStep struct {
	Order             int
	BusinessCode      string
	Name              string
	Type              string
	ChecklistRequired string
	DocumentRequired  string
	PhotoRequired     string
	Roles             string
}

type pipelineTemplatePDFNodeReference struct {
	SourceRef    string
	TargetOrder  int
	BusinessCode string
	TargetName   string
	Note         string
}

type pipelinePDFTableColumn struct {
	Header string
	Width  float64
	Align  string
}

func (s *Service) PipelineTemplatePDF(ctx context.Context, templateID string) (PipelineTemplatePDFResult, error) {
	template, err := s.repo.GetTemplateDetail(ctx, templateID)
	if err != nil {
		return PipelineTemplatePDFResult{}, err
	}
	permissionLabels, err := s.repo.PipelineTemplatePermissionLabels(ctx)
	if err != nil {
		return PipelineTemplatePDFResult{}, err
	}
	data, err := GeneratePipelineTemplatePDF(template, permissionLabels, time.Now())
	if err != nil {
		return PipelineTemplatePDFResult{}, err
	}
	return PipelineTemplatePDFResult{
		Data:     data,
		Filename: pipelineTemplatePDFFilename(template),
		MIMEType: pipelinePDFMimeType,
	}, nil
}

func (r *Repository) PipelineTemplatePermissionLabels(ctx context.Context) (map[string]string, error) {
	rows, err := r.Query(ctx, `SELECT key, name FROM permissions ORDER BY module, key`)
	if err != nil {
		return nil, err
	}
	labels := make(map[string]string, len(rows))
	for _, row := range rows {
		key := strings.TrimSpace(fmt.Sprint(row["key"]))
		name := strings.TrimSpace(fmt.Sprint(row["name"]))
		if key != "" && key != "<nil>" {
			labels[key] = name
		}
	}
	return labels, nil
}

func GeneratePipelineTemplatePDF(template Record, permissionLabels map[string]string, generatedAt time.Time) ([]byte, error) {
	document := buildPipelineTemplatePDFDocument(template, permissionLabels, generatedAt)
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(10, 10, 10)
	pdf.SetAutoPageBreak(false, 12)
	pdf.SetCellMargin(0)
	pdf.SetCompression(true)
	pdf.AddUTF8FontFromBytes(pipelinePDFFontFamily, "", pipelinePDFRegularFont)
	pdf.AddUTF8FontFromBytes(pipelinePDFFontFamily, "B", pipelinePDFBoldFont)
	pdf.SetTitle("Mô tả Workflow Template - "+document.Name, true)
	pdf.SetSubject("Cấu hình node và checklist/step của workflow template", true)
	pdf.SetCreator("Thủ Thiêm Workflow", true)
	pdf.SetLang("vi-VN")
	pdf.SetCreationDate(generatedAt.UTC())
	pdf.SetModificationDate(generatedAt.UTC())
	pdf.AliasNbPages("{nb}")
	pdf.SetFooterFunc(func() {
		pdf.SetY(-8)
		pdf.SetFont(pipelinePDFFontFamily, "", 7)
		pdf.SetTextColor(100, 116, 139)
		pdf.CellFormat(0, 4, fmt.Sprintf("Trang %d / {nb}", pdf.PageNo()), "", 0, "R", false, 0, "")
	})

	pdf.AddPage()
	drawPipelinePDFDocumentHeader(pdf, document)
	drawPipelinePDFSectionTitle(pdf, "1. SƠ ĐỒ")
	drawPipelinePDFWorkflowDiagram(pdf, document)

	pdf.AddPage()
	drawPipelinePDFSectionTitle(pdf, "2. DANH SÁCH NODE")
	nodeColumns := []pipelinePDFTableColumn{
		{Header: "STT", Width: 8, Align: "C"},
		{Header: "Business code", Width: 34, Align: "L"},
		{Header: "Tên node", Width: 30, Align: "L"},
		{Header: "Mô tả", Width: 50, Align: "L"},
		{Header: "Loại", Width: 26, Align: "L"},
		{Header: "Phân quyền", Width: 38, Align: "L"},
		{Header: "Role được duyệt", Width: 35, Align: "L"},
		{Header: "Bắt buộc nhập ý kiến duyệt", Width: 26, Align: "C"},
		{Header: "Người gửi không được tự duyệt", Width: 30, Align: "C"},
	}
	nodeRows := make([][]string, 0, len(document.Nodes))
	for _, node := range document.Nodes {
		nodeRows = append(nodeRows, []string{
			strconv.Itoa(node.Order),
			node.BusinessCode,
			node.Name,
			node.Description,
			node.Type,
			node.Permissions,
			node.ApprovalRoles,
			node.CommentRequired,
			node.RequesterCannotApprove,
		})
	}
	if len(nodeRows) == 0 {
		drawPipelinePDFEmptyState(pdf, "Workflow chưa có node.")
	} else {
		drawPipelinePDFTable(pdf, nodeColumns, nodeRows, 6.2, 3.3, "2. DANH SÁCH NODE (tiếp theo)")
	}

	ensurePipelinePDFSpace(pdf, 18)
	drawPipelinePDFSectionTitle(pdf, "3. CHECKLIST / STEP THEO NODE")
	pdf.SetFont(pipelinePDFFontFamily, "", 8)
	pdf.SetTextColor(71, 85, 105)
	pdf.MultiCell(0, 4.3, pipelinePDFStepSemanticsNote, "", "L", false)
	pdf.SetTextColor(15, 23, 42)
	pdf.Ln(1)
	if len(document.Nodes) == 0 {
		drawPipelinePDFEmptyState(pdf, "Chưa có checklist/step để hiển thị.")
	}
	stepColumns := []pipelinePDFTableColumn{
		{Header: "STT", Width: 8, Align: "C"},
		{Header: "Business code", Width: 43, Align: "L"},
		{Header: "Tên", Width: 60, Align: "L"},
		{Header: "Loại", Width: 35, Align: "L"},
		{Header: "Checklist bắt buộc", Width: 25, Align: "C"},
		{Header: "Tài liệu bắt buộc", Width: 25, Align: "C"},
		{Header: "Hình ảnh bắt buộc", Width: 25, Align: "C"},
		{Header: "Role phụ trách", Width: 56, Align: "L"},
	}
	for _, node := range document.Nodes {
		title := fmt.Sprintf("NODE %02d - %s", node.Order, node.Name)
		ensurePipelinePDFSpace(pdf, 18)
		drawPipelinePDFNodeTitle(pdf, title)
		if len(node.Steps) == 0 {
			drawPipelinePDFEmptyState(pdf, "Chưa có checklist/step.")
			continue
		}
		rows := make([][]string, 0, len(node.Steps))
		for _, step := range node.Steps {
			rows = append(rows, []string{
				strconv.Itoa(step.Order), step.BusinessCode, step.Name, step.Type,
				step.ChecklistRequired, step.DocumentRequired, step.PhotoRequired, step.Roles,
			})
		}
		drawPipelinePDFTable(pdf, stepColumns, rows, 6.8, 3.7, title+" (tiếp theo)")
		pdf.Ln(3)
	}

	if len(document.NodeReferences) > 0 {
		ensurePipelinePDFSpace(pdf, 20)
		drawPipelinePDFSectionTitle(pdf, "PHỤ LỤC A - "+strings.ToUpper(document.NodeReferenceTitle))
		referenceColumns := []pipelinePDFTableColumn{
			{Header: "Client", Width: 26, Align: "L"},
			{Header: "Rev4", Width: 18, Align: "C"},
			{Header: "Business code", Width: 58, Align: "L"},
			{Header: "Node hiện tại", Width: 65, Align: "L"},
			{Header: "Ghi chú", Width: 110, Align: "L"},
		}
		referenceRows := make([][]string, 0, len(document.NodeReferences))
		for _, reference := range document.NodeReferences {
			referenceRows = append(referenceRows, []string{
				reference.SourceRef,
				fmt.Sprintf("%02d", reference.TargetOrder),
				reference.BusinessCode,
				reference.TargetName,
				reference.Note,
			})
		}
		drawPipelinePDFTable(pdf, referenceColumns, referenceRows, 7, 3.8, "PHỤ LỤC A - "+strings.ToUpper(document.NodeReferenceTitle)+" (tiếp theo)")
	}

	if err := pdf.Error(); err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	if err := pdf.Output(&buffer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func buildPipelineTemplatePDFDocument(template Record, permissionLabels map[string]string, generatedAt time.Time) pipelineTemplatePDFDocument {
	revision := int(number(template["revision_no"]))
	if revision <= 0 {
		revision = int(number(template["version"]))
	}
	if revision <= 0 {
		revision = 1
	}
	document := pipelineTemplatePDFDocument{
		Name:        pipelinePDFText(template["name"], "Workflow chưa đặt tên"),
		Description: pipelinePDFText(template["description"], "Không có mô tả."),
		Revision:    revision,
		Status:      pipelineTemplateStatusLabel(fmt.Sprint(template["status"])),
		GeneratedAt: generatedAt,
	}
	nodes := pipelinePDFRecords(template["nodes"])
	sort.SliceStable(nodes, func(i, j int) bool {
		left, right := int(number(nodes[i]["order_index"])), int(number(nodes[j]["order_index"]))
		if left != right {
			return left < right
		}
		return fmt.Sprint(nodes[i]["id"]) < fmt.Sprint(nodes[j]["id"])
	})
	for nodeIndex, node := range nodes {
		policy := approvalPolicyFromValue(node["approval_policy_json"])
		positionX, positionXOK := pipelinePDFDiagramCoordinate(node["position_x"])
		positionY, positionYOK := pipelinePDFDiagramCoordinate(node["position_y"])
		nodeID := strings.TrimSpace(fmt.Sprint(node["id"]))
		if nodeID == "" || nodeID == "<nil>" {
			nodeID = fmt.Sprintf("node-%d", nodeIndex+1)
		}
		typeKey := strings.TrimSpace(fmt.Sprint(node["type"]))
		pdfNode := pipelineTemplatePDFNode{
			ID:                     nodeID,
			Order:                  pipelinePDFOrder(node["order_index"], nodeIndex+1),
			BusinessCode:           pipelinePDFText(node["business_code"], "-"),
			Name:                   pipelinePDFText(node["name"], "Node chưa đặt tên"),
			Description:            pipelinePDFText(node["description"], "-"),
			Type:                   pipelinePDFTypeLabel(pipelineNodeTypeLabels, typeKey),
			DiagramType:            pipelinePDFTypeDisplayLabel(pipelineNodeTypeLabels, typeKey),
			Permissions:            pipelinePDFPermissionText(nodePermissions(node), permissionLabels),
			ApprovalRoles:          pipelinePDFApprovalRoles(policy),
			CommentRequired:        pipelinePDFApprovalBoolean(policy, policy.CommentRequired),
			RequesterCannotApprove: pipelinePDFApprovalBoolean(policy, policy.RequesterCannotApprove),
			PositionX:              positionX,
			PositionY:              positionY,
			HasPosition:            positionXOK && positionYOK,
		}
		steps := pipelinePDFRecords(node["steps"])
		sort.SliceStable(steps, func(i, j int) bool {
			left, right := int(number(steps[i]["order_index"])), int(number(steps[j]["order_index"]))
			if left != right {
				return left < right
			}
			return fmt.Sprint(steps[i]["id"]) < fmt.Sprint(steps[j]["id"])
		})
		for stepIndex, step := range steps {
			roles := stringSlice(step["required_role_ids"])
			roleText := "Chưa gán role"
			if len(roles) > 0 {
				roleText = strings.Join(roles, ", ")
			}
			pdfNode.Steps = append(pdfNode.Steps, pipelineTemplatePDFStep{
				Order:             pipelinePDFOrder(step["order_index"], stepIndex+1),
				BusinessCode:      pipelinePDFText(step["business_code"], "-"),
				Name:              pipelinePDFText(step["name"], "Step chưa đặt tên"),
				Type:              pipelinePDFTypeLabel(pipelineStepTypeLabels, fmt.Sprint(step["step_type"])),
				ChecklistRequired: pipelinePDFBoolean(step["checklist_required"]),
				DocumentRequired:  pipelinePDFBoolean(step["document_required"]),
				PhotoRequired:     pipelinePDFBoolean(step["photo_required"]),
				Roles:             roleText,
			})
		}
		document.Nodes = append(document.Nodes, pdfNode)
	}
	edges := pipelinePDFRecords(template["edges"])
	sort.SliceStable(edges, func(i, j int) bool {
		left, right := int(number(edges[i]["priority"])), int(number(edges[j]["priority"]))
		if left != right {
			return left < right
		}
		return fmt.Sprint(edges[i]["id"]) < fmt.Sprint(edges[j]["id"])
	})
	for edgeIndex, edge := range edges {
		portMode, sourceSide, targetSide := pipelinePDFDiagramMetadata(edge["condition_config"])
		edgeID := strings.TrimSpace(fmt.Sprint(edge["id"]))
		if edgeID == "" || edgeID == "<nil>" {
			edgeID = fmt.Sprintf("edge-%d", edgeIndex+1)
		}
		document.Edges = append(document.Edges, pipelineTemplatePDFEdge{
			ID:         edgeID,
			FromNodeID: strings.TrimSpace(fmt.Sprint(edge["from_node_id"])),
			ToNodeID:   strings.TrimSpace(fmt.Sprint(edge["to_node_id"])),
			Label:      pipelinePDFText(edge["label"], ""),
			PortMode:   portMode,
			SourceSide: sourceSide,
			TargetSide: targetSide,
			Priority:   int(number(edge["priority"])),
		})
	}
	document.NodeReferenceTitle, document.NodeReferences = pipelinePDFNodeReferences(template["report_config_json"], document.Nodes)
	return document
}

func drawPipelinePDFDocumentHeader(pdf *fpdf.Fpdf, document pipelineTemplatePDFDocument) {
	pdf.SetTextColor(37, 99, 235)
	pdf.SetFont(pipelinePDFFontFamily, "B", 9)
	pdf.CellFormat(0, 5, "THỦ THIÊM WORKFLOW", "", 1, "L", false, 0, "")
	pdf.SetTextColor(15, 23, 42)
	pdf.SetFont(pipelinePDFFontFamily, "B", 17)
	pdf.CellFormat(0, 8, "MÔ TẢ WORKFLOW TEMPLATE", "", 1, "L", false, 0, "")
	pdf.SetFont(pipelinePDFFontFamily, "B", 11)
	pdf.MultiCell(0, 5.5, document.Name, "", "L", false)
	pdf.SetFont(pipelinePDFFontFamily, "", 8)
	pdf.SetTextColor(71, 85, 105)
	pdf.CellFormat(0, 5, pipelinePDFDefinitionMetadata(document), "", 1, "L", false, 0, "")
	pdf.MultiCell(0, 4.3, "Mô tả: "+document.Description, "", "L", false)
	pdf.Ln(2)
}

func pipelinePDFDefinitionMetadata(document pipelineTemplatePDFDocument) string {
	return fmt.Sprintf("Revision định nghĩa: %d    Trạng thái định nghĩa: %s    Tổng số node: %d    Xuất lúc: %s", document.Revision, document.Status, len(document.Nodes), document.GeneratedAt.Format("02/01/2006 15:04 MST"))
}

func drawPipelinePDFSectionTitle(pdf *fpdf.Fpdf, title string) {
	pdf.SetFont(pipelinePDFFontFamily, "B", 10)
	pdf.SetTextColor(15, 23, 42)
	pdf.CellFormat(0, 6, title, "", 1, "L", false, 0, "")
}

func drawPipelinePDFNodeTitle(pdf *fpdf.Fpdf, title string) {
	pdf.SetFillColor(239, 246, 255)
	pdf.SetTextColor(30, 64, 175)
	pdf.SetFont(pipelinePDFFontFamily, "B", 9)
	pdf.CellFormat(0, 6, title, "", 1, "L", true, 0, "")
	pdf.SetTextColor(15, 23, 42)
}

func drawPipelinePDFEmptyState(pdf *fpdf.Fpdf, message string) {
	pdf.SetFont(pipelinePDFFontFamily, "", 8)
	pdf.SetTextColor(100, 116, 139)
	pdf.SetFillColor(248, 250, 252)
	pdf.CellFormat(0, 9, message, "1", 1, "L", true, 0, "")
	pdf.SetTextColor(15, 23, 42)
}

func drawPipelinePDFTable(pdf *fpdf.Fpdf, columns []pipelinePDFTableColumn, rows [][]string, fontSize, lineHeight float64, continuationTitle string) {
	drawPipelinePDFTableHeader(pdf, columns)
	for _, row := range rows {
		rowHeight := pipelinePDFRowHeight(pdf, columns, row, fontSize, lineHeight, 2)
		if pdf.GetY()+rowHeight > pipelinePDFPageBottom {
			pdf.AddPage()
			drawPipelinePDFSectionTitle(pdf, continuationTitle)
			drawPipelinePDFTableHeader(pdf, columns)
		}
		drawPipelinePDFTableRow(pdf, columns, row, fontSize, lineHeight, rowHeight, false)
	}
}

func drawPipelinePDFTableHeader(pdf *fpdf.Fpdf, columns []pipelinePDFTableColumn) {
	headers := make([]string, len(columns))
	for index, column := range columns {
		headers[index] = column.Header
	}
	height := pipelinePDFRowHeight(pdf, columns, headers, 7, 3.6, 2.4)
	drawPipelinePDFTableRow(pdf, columns, headers, 7, 3.6, height, true)
}

func drawPipelinePDFTableRow(pdf *fpdf.Fpdf, columns []pipelinePDFTableColumn, values []string, fontSize, lineHeight, rowHeight float64, header bool) {
	x, y := pdf.GetX(), pdf.GetY()
	style := ""
	if header {
		style = "B"
		pdf.SetFillColor(226, 232, 240)
		pdf.SetTextColor(30, 41, 59)
	} else {
		pdf.SetFillColor(255, 255, 255)
		pdf.SetTextColor(15, 23, 42)
	}
	pdf.SetFont(pipelinePDFFontFamily, style, fontSize)
	pdf.SetDrawColor(203, 213, 225)
	for index, column := range columns {
		value := "-"
		if index < len(values) && strings.TrimSpace(values[index]) != "" {
			value = values[index]
		}
		pdf.Rect(x, y, column.Width, rowHeight, "DF")
		pdf.SetXY(x+1, y+1.2)
		pdf.MultiCell(column.Width-2, lineHeight, value, "", column.Align, false)
		x += column.Width
	}
	pdf.SetXY(10, y+rowHeight)
}

func pipelinePDFRowHeight(pdf *fpdf.Fpdf, columns []pipelinePDFTableColumn, values []string, fontSize, lineHeight, verticalPadding float64) float64 {
	pdf.SetFont(pipelinePDFFontFamily, "", fontSize)
	maxLines := 1
	for index, column := range columns {
		value := "-"
		if index < len(values) && strings.TrimSpace(values[index]) != "" {
			value = values[index]
		}
		lines := pdf.SplitText(value, column.Width-2)
		if len(lines) > maxLines {
			maxLines = len(lines)
		}
	}
	return float64(maxLines)*lineHeight + verticalPadding
}

func ensurePipelinePDFSpace(pdf *fpdf.Fpdf, height float64) {
	if pdf.GetY()+height <= pipelinePDFPageBottom {
		return
	}
	pdf.AddPage()
}

func pipelinePDFPermissionText(keys []string, labels map[string]string) string {
	if len(keys) == 0 {
		return "Không yêu cầu"
	}
	values := make([]string, 0, len(keys))
	for _, key := range keys {
		label := strings.TrimSpace(labels[key])
		if label == "" || label == "<nil>" {
			values = append(values, key)
			continue
		}
		values = append(values, fmt.Sprintf("%s (%s)", label, key))
	}
	return strings.Join(values, "\n")
}

func pipelinePDFApprovalRoles(policy ApprovalPolicy) string {
	if !policy.enabled() {
		return "Không áp dụng"
	}
	roles := policy.Roles
	if policy.Mode == "ordered" {
		orderedRoles := make([]string, 0, len(policy.Roles))
		for _, stage := range policy.Stages {
			orderedRoles = append(orderedRoles, stage.Roles...)
		}
		roles = normalizedRoleList(orderedRoles)
	}
	roleText := strings.Join(roles, ", ")
	switch policy.Mode {
	case "all":
		return "Tất cả: " + roleText
	case "quorum":
		return fmt.Sprintf("Quorum %d/%d: %s", policy.Quorum, len(roles), roleText)
	case "ordered":
		return "Theo thứ tự: " + strings.Join(roles, " > ")
	default:
		return "Một trong: " + roleText
	}
}

func pipelinePDFApprovalBoolean(policy ApprovalPolicy, value bool) string {
	if !policy.enabled() {
		return "Không áp dụng"
	}
	if value {
		return "Có"
	}
	return "Không"
}

func pipelinePDFTypeLabel(labels map[string]string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "<nil>" {
		return "Không xác định"
	}
	if label := labels[value]; label != "" {
		return fmt.Sprintf("%s (%s)", label, value)
	}
	return value
}

func pipelinePDFTypeDisplayLabel(labels map[string]string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "<nil>" {
		return "Không xác định"
	}
	if label := labels[value]; label != "" {
		return label
	}
	return value
}

func pipelinePDFText(value any, fallback string) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return fallback
	}
	return text
}

func pipelinePDFBoolean(value any) string {
	if boolValue(value) {
		return "Có"
	}
	return "Không"
}

func pipelinePDFNodeReferences(value any, nodes []pipelineTemplatePDFNode) (string, []pipelineTemplatePDFNodeReference) {
	config, ok := pipelinePDFRecord(value)
	if !ok {
		return "", nil
	}
	title := pipelinePDFText(config["node_reference_title"], "Đối chiếu node client")
	nodeByBusinessCode := make(map[string]pipelineTemplatePDFNode, len(nodes))
	for _, node := range nodes {
		code := strings.TrimSpace(node.BusinessCode)
		if code != "" && code != "-" {
			nodeByBusinessCode[code] = node
		}
	}
	references := []pipelineTemplatePDFNodeReference{}
	for _, item := range pipelinePDFRecords(config["node_references"]) {
		businessCode := pipelinePDFText(item["target_business_code"], "")
		target, exists := nodeByBusinessCode[businessCode]
		if !exists {
			continue
		}
		sourceRef := pipelinePDFText(item["source_ref"], "")
		if sourceRef == "" {
			continue
		}
		references = append(references, pipelineTemplatePDFNodeReference{
			SourceRef: sourceRef, TargetOrder: target.Order, BusinessCode: businessCode,
			TargetName: target.Name, Note: pipelinePDFText(item["note"], "-"),
		})
	}
	return title, references
}

func pipelinePDFRecord(value any) (Record, bool) {
	switch record := value.(type) {
	case Record:
		return record, true
	case map[string]any:
		return Record(record), true
	default:
		return nil, false
	}
}

func pipelinePDFOrder(value any, fallback int) int {
	order := int(number(value))
	if order <= 0 {
		return fallback
	}
	return order
}

func pipelinePDFRecords(value any) []Record {
	switch records := value.(type) {
	case []Record:
		return append([]Record(nil), records...)
	case []any:
		out := make([]Record, 0, len(records))
		for _, item := range records {
			switch record := item.(type) {
			case Record:
				out = append(out, record)
			case map[string]any:
				out = append(out, Record(record))
			}
		}
		return out
	default:
		return nil
	}
}

func pipelineTemplatePDFFilename(template Record) string {
	name := slugText(fmt.Sprint(template["name"]))
	if name == "" {
		name = "template"
	}
	revision := int(number(template["revision_no"]))
	if revision <= 0 {
		revision = int(number(template["version"]))
	}
	if revision <= 0 {
		revision = 1
	}
	return fmt.Sprintf("workflow-%s-rev-%d.pdf", name, revision)
}

func pipelineTemplateStatusLabel(value string) string {
	switch strings.TrimSpace(value) {
	case "draft":
		return "Nháp"
	case "published":
		return "Đã phát hành"
	case "archived":
		return "Lưu trữ"
	case "deleted":
		return "Đã xóa"
	default:
		return pipelinePDFText(value, "Không xác định")
	}
}

var pipelineNodeTypeLabels = map[string]string{
	"survey":                "Khảo sát",
	"design":                "Thiết kế",
	"boq":                   "BOQ / Dự toán",
	"budget_approval":       "Duyệt ngân sách",
	"procurement":           "Mua vật tư",
	"warehouse_issue":       "Xuất kho",
	"contract":              "Hợp đồng",
	"construction_planning": "Kế hoạch thi công",
	"construction":          "Thi công",
	"incoming_acceptance":   "Nghiệm thu đầu vào",
	"acceptance":            "Nghiệm thu",
	"rework":                "Sửa lỗi",
	"replanning":            "Lập kế hoạch lại",
	"handover":              "Bàn giao",
	"financial_closure":     "Đóng tài chính",
	"custom":                "Tùy chỉnh",
}

var pipelineStepTypeLabels = map[string]string{
	"task":             "Công việc",
	"checklist":        "Checklist",
	"approval":         "Phê duyệt",
	"document_upload":  "Tải tài liệu",
	"photo_upload":     "Tải ảnh",
	"design_review":    "Rà soát thiết kế",
	"budget_check":     "Kiểm tra ngân sách",
	"material_check":   "Kiểm tra vật tư",
	"daily_report":     "Nhật ký thi công",
	"meeting_minutes":  "Biên bản họp",
	"acceptance_check": "Kiểm tra nghiệm thu",
	"rework_task":      "Việc sửa lỗi",
	"handover_check":   "Kiểm tra bàn giao",
	"accounting_check": "Kiểm tra kế toán",
}
