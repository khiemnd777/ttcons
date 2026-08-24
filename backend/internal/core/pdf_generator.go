package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"codeberg.org/go-pdf/fpdf"
)

const (
	workflowPDFPageWidth    = 210.0
	workflowPDFPageBottom   = 278.0
	workflowPDFMargin       = 14.0
	workflowPDFContentWidth = workflowPDFPageWidth - 2*workflowPDFMargin
	workflowPDFFontFamily   = pipelinePDFFontFamily
)

var workflowPDFLocation = time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)

type WorkflowFormPDFContext struct {
	GeneratedAt   time.Time
	GeneratedBy   string
	ProjectCode   string
	ProjectName   string
	NodeName      string
	NodeOrder     int
	StepName      string
	StepOrder     int
	WorkItemTitle string
	SealImages    map[string][]byte
}

type workflowFormPDFDocument struct {
	Title            string
	TemplateName     string
	TemplateVersion  int
	FormType         string
	Status           string
	Revision         int
	ProjectID        string
	WorkItemID       string
	Project          string
	Workflow         string
	SubmittedAt      string
	GeneratedAt      string
	GeneratedBy      string
	PayloadHash      string
	Fields           []workflowFormPDFField
	Confirmations    []workflowFormPDFField
	Signatures       []Record
	SealApplications []Record
	SealImages       map[string][]byte
}

type workflowFormPDFField struct {
	Definition Record
	Label      string
	FieldType  string
	Value      any
	Display    string
}

type workflowFormPDFRenderer struct {
	pdf      *fpdf.Fpdf
	document workflowFormPDFDocument
}

func GenerateWorkflowFormPDF(form Record) ([]byte, string, error) {
	return GenerateWorkflowFormPDFWithContext(form, WorkflowFormPDFContext{})
}

func GenerateWorkflowFormPDFWithContext(form Record, context WorkflowFormPDFContext) ([]byte, string, error) {
	rawSnapshot, err := json.Marshal(workflowFormPDFSnapshot(form))
	if err != nil {
		return nil, "", fmt.Errorf("encode workflow form PDF snapshot: %w", err)
	}
	hash := sha256.Sum256(rawSnapshot)
	payloadHash := hex.EncodeToString(hash[:])
	document, err := buildWorkflowFormPDFDocument(form, context, payloadHash)
	if err != nil {
		return nil, "", err
	}

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(workflowPDFMargin, 12, workflowPDFMargin)
	pdf.SetAutoPageBreak(false, 12)
	pdf.SetCellMargin(0)
	pdf.SetCompression(true)
	pdf.AddUTF8FontFromBytes(workflowPDFFontFamily, "", pipelinePDFRegularFont)
	pdf.AddUTF8FontFromBytes(workflowPDFFontFamily, "B", pipelinePDFBoldFont)
	pdf.SetTitle(document.Title, true)
	pdf.SetSubject("Hồ sơ nghiệp vụ của đầu việc quy trình", true)
	pdf.SetCreator("Thủ Thiêm Workflow", true)
	pdf.SetLang("vi-VN")
	if !context.GeneratedAt.IsZero() {
		pdf.SetCreationDate(context.GeneratedAt.UTC())
		pdf.SetModificationDate(context.GeneratedAt.UTC())
	}
	pdf.AliasNbPages("{nb}")
	pdf.SetFooterFunc(func() {
		drawWorkflowPDFFooter(pdf, payloadHash)
	})

	renderer := &workflowFormPDFRenderer{pdf: pdf, document: document}
	renderer.addPage(true)
	renderer.drawSummary()
	renderer.drawFormContent()
	renderer.drawConfirmations()
	renderer.drawSignatures()
	renderer.drawVerification()

	if err := pdf.Error(); err != nil {
		return nil, "", fmt.Errorf("render workflow form PDF: %w", err)
	}
	var buffer bytes.Buffer
	if err := pdf.Output(&buffer); err != nil {
		return nil, "", fmt.Errorf("write workflow form PDF: %w", err)
	}
	return buffer.Bytes(), payloadHash, nil
}

func workflowFormPDFSnapshot(form Record) Record {
	return Record{
		"instance":          form["instance"],
		"template":          form["template"],
		"version":           form["version"],
		"fields":            form["fields"],
		"answers":           form["answers"],
		"signatures":        form["signatures"],
		"seal_applications": workflowFormPDFSealApplicationSnapshots(form["seal_applications"]),
	}
}

func selectWorkflowFormPDFConfirmations(form Record) (Record, Record) {
	instance := workflowPDFRecord(form["instance"])
	currentRevision := int(number(instance["current_revision"]))
	signatures := workflowPDFRecordsForRevision(form["signatures"], currentRevision)
	sealApplications := workflowPDFRecordsForRevision(form["seal_applications"], currentRevision)
	preferences := workflowPDFRecords(form["pdf_confirmation_preferences"])

	hasSignatures := map[string]bool{}
	hasSeals := map[string]bool{}
	preferenceByPurpose := map[string]Record{}
	for _, signature := range signatures {
		hasSignatures[strings.TrimSpace(fmt.Sprint(signature["purpose"]))] = true
	}
	for _, application := range sealApplications {
		hasSeals[strings.TrimSpace(fmt.Sprint(application["purpose"]))] = true
	}
	for _, preference := range preferences {
		purpose := strings.TrimSpace(fmt.Sprint(preference["purpose"]))
		if purpose != "" {
			preferenceByPurpose[purpose] = preference
		}
	}

	selectedSignatures := make([]Record, 0, len(signatures))
	selectedSignatureIDs := make([]string, 0, len(signatures))
	for _, signature := range signatures {
		purpose := strings.TrimSpace(fmt.Sprint(signature["purpose"]))
		preference, hasPreference := preferenceByPurpose[purpose]
		if hasSignatures[purpose] && hasSeals[purpose] && hasPreference && !boolValue(preference["include_signatures"]) {
			continue
		}
		selectedSignatures = append(selectedSignatures, signature)
		if id := cleanID(signature["id"]); id != "" {
			selectedSignatureIDs = append(selectedSignatureIDs, id)
		}
	}

	selectedSealApplications := make([]Record, 0, len(sealApplications))
	selectedSealApplicationIDs := make([]string, 0, len(sealApplications))
	for _, application := range sealApplications {
		purpose := strings.TrimSpace(fmt.Sprint(application["purpose"]))
		preference, hasPreference := preferenceByPurpose[purpose]
		if hasSignatures[purpose] && hasSeals[purpose] && hasPreference && !boolValue(preference["include_seals"]) {
			continue
		}
		selectedSealApplications = append(selectedSealApplications, application)
		if id := cleanID(application["id"]); id != "" {
			selectedSealApplicationIDs = append(selectedSealApplicationIDs, id)
		}
	}

	selectedForm := make(Record, len(form))
	for key, value := range form {
		selectedForm[key] = value
	}
	selectedForm["signatures"] = selectedSignatures
	selectedForm["seal_applications"] = selectedSealApplications

	selection := Record{
		"signature_ids":          selectedSignatureIDs,
		"seal_application_ids":   selectedSealApplicationIDs,
		"signature_count":        len(selectedSignatures),
		"seal_application_count": len(selectedSealApplications),
	}
	return selectedForm, selection
}

func workflowPDFRecordsForRevision(value any, revisionNo int) []Record {
	records := workflowPDFRecords(value)
	if revisionNo <= 0 {
		return records
	}
	filtered := make([]Record, 0, len(records))
	for _, record := range records {
		if int(number(record["form_revision_no"])) == revisionNo {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func workflowFormPDFSealApplicationSnapshots(value any) []Record {
	applications := workflowPDFRecords(value)
	snapshots := make([]Record, 0, len(applications))
	for _, application := range applications {
		snapshots = append(snapshots, Record{
			"id":                    application["id"],
			"project_id":            application["project_id"],
			"work_item_id":          application["work_item_id"],
			"form_instance_id":      application["form_instance_id"],
			"form_version_id":       application["form_version_id"],
			"form_revision_no":      application["form_revision_no"],
			"seal_asset_id":         application["seal_asset_id"],
			"purpose":               application["purpose"],
			"applied_by":            application["applied_by"],
			"applied_by_name":       application["applied_by_name"],
			"applied_by_roles_json": application["applied_by_roles_json"],
			"comment":               application["comment"],
			"payload_json":          application["payload_json"],
			"payload_hash":          application["payload_hash"],
			"applied_at":            application["applied_at"],
			"created_at":            application["created_at"],
		})
	}
	return snapshots
}

func buildWorkflowFormPDFDocument(form Record, context WorkflowFormPDFContext, payloadHash string) (workflowFormPDFDocument, error) {
	instance := workflowPDFRecord(form["instance"])
	template := workflowPDFRecord(form["template"])
	version := workflowPDFRecord(form["version"])
	answers := workflowPDFRecord(form["answers"])
	fields := workflowPDFRecords(form["fields"])

	generatedAt := context.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now()
	}
	document := workflowFormPDFDocument{
		Title:            workflowPDFText(instance["title"], "Hồ sơ nghiệp vụ"),
		TemplateName:     workflowPDFText(template["name"], "Biểu mẫu quy trình"),
		TemplateVersion:  int(number(version["version_no"])),
		FormType:         workflowPDFFormTypeLabel(fmt.Sprint(instance["form_type"])),
		Status:           workflowPDFStatusLabel(fmt.Sprint(instance["status"])),
		Revision:         int(number(instance["current_revision"])),
		ProjectID:        workflowPDFText(instance["project_id"], ""),
		WorkItemID:       workflowPDFText(instance["work_item_id"], ""),
		Project:          workflowPDFProjectLabel(context, instance),
		Workflow:         workflowPDFWorkflowLabel(context, instance),
		SubmittedAt:      workflowPDFDateTime(instance["submitted_at"]),
		GeneratedAt:      workflowPDFDateTime(generatedAt),
		GeneratedBy:      workflowPDFText(context.GeneratedBy, "Không xác định"),
		PayloadHash:      payloadHash,
		Signatures:       workflowPDFRecords(form["signatures"]),
		SealApplications: workflowPDFRecords(form["seal_applications"]),
		SealImages:       context.SealImages,
	}

	for _, field := range fields {
		visible, err := workflowFormFieldVisible(field, answers)
		if err != nil {
			return workflowFormPDFDocument{}, fmt.Errorf("evaluate PDF field visibility %s: %w", field["field_key"], err)
		}
		if !visible {
			continue
		}
		key := strings.TrimSpace(fmt.Sprint(field["field_key"]))
		presentationField := workflowPDFPresentationField(strings.TrimSpace(fmt.Sprint(template["code"])), field)
		entry := workflowFormPDFField{
			Definition: presentationField,
			Label:      workflowPDFText(field["label"], key),
			FieldType:  strings.TrimSpace(fmt.Sprint(field["field_type"])),
			Value:      answers[key],
		}
		entry.Display = workflowPDFDisplayValue(entry.Definition, entry.Value)
		if entry.FieldType == "checkbox" {
			document.Confirmations = append(document.Confirmations, entry)
		} else {
			document.Fields = append(document.Fields, entry)
		}
	}
	return document, nil
}

func workflowPDFPresentationField(templateCode string, field Record) Record {
	fieldKey := strings.TrimSpace(fmt.Sprint(field["field_key"]))
	if strings.TrimSpace(fmt.Sprint(field["field_type"])) != "structured_list" {
		return field
	}
	hiddenItemFieldKey := ""
	switch {
	case templateCode == clientSiteRiskAssessmentFormCode && fieldKey == clientSiteRiskItemsFieldKey:
		hiddenItemFieldKey = clientSiteRiskCategoryFieldKey
	case templateCode == clientConstructionHandoffPlanFormCode && fieldKey == clientConstructionInspectionItemsFieldKey:
		hiddenItemFieldKey = clientConstructionEvidenceRequiredFieldKey
	default:
		return field
	}
	validation := workflowPDFRecord(field["validation_json"])
	specs := workflowPDFRecords(validation["item_fields"])
	filteredSpecs := make([]Record, 0, len(specs))
	for _, spec := range specs {
		if strings.TrimSpace(fmt.Sprint(spec["key"])) == hiddenItemFieldKey {
			continue
		}
		filteredSpecs = append(filteredSpecs, spec)
	}
	if len(filteredSpecs) == len(specs) {
		return field
	}
	presentationField := cloneWorkflowFormRecord(field)
	presentationValidation := cloneWorkflowFormRecord(validation)
	presentationValidation["item_fields"] = filteredSpecs
	presentationField["validation_json"] = presentationValidation
	return presentationField
}

func (r *workflowFormPDFRenderer) addPage(first bool) {
	r.pdf.AddPage()
	if first {
		r.drawDocumentHeader()
		return
	}
	r.drawContinuationHeader()
}

func (r *workflowFormPDFRenderer) drawDocumentHeader() {
	pdf := r.pdf
	x := workflowPDFMargin
	y := 11.0
	pdf.SetFillColor(23, 32, 51)
	pdf.RoundedRect(x, y, 13, 13, 2, "1234", "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont(workflowPDFFontFamily, "B", 9)
	pdf.SetXY(x, y+2.1)
	pdf.CellFormat(13, 4, "THỦ", "", 1, "C", false, 0, "")
	pdf.SetTextColor(96, 165, 250)
	pdf.SetFont(workflowPDFFontFamily, "B", 6.2)
	pdf.SetX(x)
	pdf.CellFormat(13, 3.6, "THIÊM", "", 0, "C", false, 0, "")

	pdf.SetXY(x+17, y+0.7)
	pdf.SetTextColor(23, 32, 51)
	pdf.SetFont(workflowPDFFontFamily, "B", 9.2)
	pdf.CellFormat(80, 4.5, "THỦ THIÊM", "", 1, "L", false, 0, "")
	pdf.SetX(x + 17)
	pdf.SetTextColor(37, 99, 235)
	pdf.SetFont(workflowPDFFontFamily, "B", 7.1)
	pdf.CellFormat(80, 4, "WORKFLOW", "", 0, "L", false, 0, "")

	pdf.SetXY(139, y+1)
	pdf.SetTextColor(71, 85, 105)
	pdf.SetFont(workflowPDFFontFamily, "B", 8)
	pdf.CellFormat(57, 4.5, "HỒ SƠ NGHIỆP VỤ", "", 0, "R", false, 0, "")
	pdf.SetDrawColor(226, 232, 240)
	pdf.Line(x, 28, workflowPDFPageWidth-workflowPDFMargin, 28)

	pdf.SetXY(x, 33)
	pdf.SetTextColor(15, 23, 42)
	pdf.SetFont(workflowPDFFontFamily, "B", 17)
	pdf.MultiCell(135, 7.5, strings.ToUpper(r.document.Title), "", "L", false)
	titleBottom := pdf.GetY()
	pillLabel := strings.ToUpper(r.document.Status)
	pdf.SetFont(workflowPDFFontFamily, "B", 8)
	pillWidth := math.Max(31, pdf.GetStringWidth(pillLabel)+10)
	pillX := workflowPDFPageWidth - workflowPDFMargin - pillWidth
	pdf.SetFillColor(236, 253, 245)
	pdf.SetDrawColor(167, 243, 208)
	pdf.RoundedRect(pillX, 34, pillWidth, 9, 2, "1234", "FD")
	pdf.SetTextColor(4, 120, 87)
	pdf.SetXY(pillX, 36.1)
	pdf.CellFormat(pillWidth, 4, pillLabel, "", 0, "C", false, 0, "")

	pdf.SetXY(x, math.Max(titleBottom+1.5, 49))
	pdf.SetTextColor(71, 85, 105)
	pdf.SetFont(workflowPDFFontFamily, "", 8.5)
	pdf.CellFormat(0, 4.5, fmt.Sprintf("%s · Phiên bản mẫu %02d", r.document.TemplateName, r.document.TemplateVersion), "", 1, "L", false, 0, "")
	pdf.SetY(math.Max(pdf.GetY()+4, 58))
}

func (r *workflowFormPDFRenderer) drawContinuationHeader() {
	pdf := r.pdf
	x := workflowPDFMargin
	pdf.SetXY(x, 10)
	pdf.SetTextColor(37, 99, 235)
	pdf.SetFont(workflowPDFFontFamily, "B", 7.5)
	pdf.CellFormat(55, 4, "THỦ THIÊM WORKFLOW", "", 0, "L", false, 0, "")
	pdf.SetTextColor(71, 85, 105)
	pdf.SetFont(workflowPDFFontFamily, "", 7.5)
	pdf.CellFormat(workflowPDFContentWidth-55, 4, r.document.Title, "", 0, "R", false, 0, "")
	pdf.SetDrawColor(226, 232, 240)
	pdf.Line(x, 16, workflowPDFPageWidth-workflowPDFMargin, 16)
	pdf.SetY(22)
}

func drawWorkflowPDFFooter(pdf *fpdf.Fpdf, payloadHash string) {
	pdf.SetY(-10)
	pdf.SetDrawColor(226, 232, 240)
	pdf.Line(workflowPDFMargin, pdf.GetY(), workflowPDFPageWidth-workflowPDFMargin, pdf.GetY())
	pdf.SetY(-8)
	pdf.SetTextColor(100, 116, 139)
	pdf.SetFont(workflowPDFFontFamily, "", 6.7)
	pdf.CellFormat(125, 4, "Mã xác thực: "+workflowPDFShortHash(payloadHash), "", 0, "L", false, 0, "")
	pdf.CellFormat(workflowPDFContentWidth-125, 4, fmt.Sprintf("Trang %d / {nb}", pdf.PageNo()), "", 0, "R", false, 0, "")
}

func (r *workflowFormPDFRenderer) ensureSpace(height float64) {
	if r.pdf.GetY()+height <= workflowPDFPageBottom {
		return
	}
	r.addPage(false)
}

func (r *workflowFormPDFRenderer) drawSummary() {
	r.ensureSpace(39)
	pdf := r.pdf
	y := pdf.GetY()
	pdf.SetFillColor(248, 250, 252)
	pdf.SetDrawColor(226, 232, 240)
	pdf.RoundedRect(workflowPDFMargin, y, workflowPDFContentWidth, 35, 2, "1234", "FD")
	cellWidth := workflowPDFContentWidth / 2
	r.drawSummaryCell(workflowPDFMargin+5, y+4, cellWidth-9, "CÔNG TRÌNH", r.document.Project)
	r.drawSummaryCell(workflowPDFMargin+cellWidth+4, y+4, cellWidth-9, "QUY TRÌNH", r.document.Workflow)
	r.drawSummaryCell(workflowPDFMargin+5, y+20, cellWidth-9, "PHIÊN BẢN", fmt.Sprintf("Hồ sơ %02d · Mẫu %02d", r.document.Revision, r.document.TemplateVersion))
	r.drawSummaryCell(workflowPDFMargin+cellWidth+4, y+20, cellWidth-9, "THỜI ĐIỂM CHỐT", workflowPDFText(r.document.SubmittedAt, "Chưa ghi nhận"))
	pdf.SetY(y + 40)
}

func (r *workflowFormPDFRenderer) drawSummaryCell(x, y, width float64, label, value string) {
	pdf := r.pdf
	pdf.SetXY(x, y)
	pdf.SetTextColor(100, 116, 139)
	pdf.SetFont(workflowPDFFontFamily, "B", 6.6)
	pdf.CellFormat(width, 3.5, label, "", 1, "L", false, 0, "")
	pdf.SetX(x)
	pdf.SetTextColor(30, 41, 59)
	pdf.SetFont(workflowPDFFontFamily, "B", 8.4)
	pdf.CellFormat(width, 5, value, "", 0, "L", false, 0, "")
}

func (r *workflowFormPDFRenderer) drawFormContent() {
	r.drawSectionTitle("01", "NỘI DUNG BIỂU MẪU", "Thông tin nghiệp vụ đã được chốt tại phiên bản này")
	if len(r.document.Fields) == 0 {
		r.drawEmptyState("Biểu mẫu không có nội dung cần hiển thị.")
		return
	}
	for _, field := range r.document.Fields {
		if field.FieldType == "structured_list" {
			r.drawStructuredField(field)
			continue
		}
		r.drawTextField(field.Label, field.Display)
	}
}

func (r *workflowFormPDFRenderer) drawSectionTitle(index, title, subtitle string) {
	r.ensureSpace(18)
	pdf := r.pdf
	y := pdf.GetY()
	pdf.SetFillColor(239, 246, 255)
	pdf.SetTextColor(30, 64, 175)
	pdf.SetFont(workflowPDFFontFamily, "B", 8)
	pdf.RoundedRect(workflowPDFMargin, y, 9, 9, 2, "1234", "F")
	pdf.SetXY(workflowPDFMargin, y+2.2)
	pdf.CellFormat(9, 4, index, "", 0, "C", false, 0, "")
	pdf.SetXY(workflowPDFMargin+13, y)
	pdf.SetTextColor(15, 23, 42)
	pdf.SetFont(workflowPDFFontFamily, "B", 10)
	pdf.CellFormat(workflowPDFContentWidth-13, 4.5, title, "", 1, "L", false, 0, "")
	pdf.SetX(workflowPDFMargin + 13)
	pdf.SetTextColor(100, 116, 139)
	pdf.SetFont(workflowPDFFontFamily, "", 7.2)
	pdf.CellFormat(workflowPDFContentWidth-13, 4, subtitle, "", 0, "L", false, 0, "")
	pdf.SetY(y + 13)
}

func (r *workflowFormPDFRenderer) drawTextField(label, value string) {
	value = workflowPDFText(value, "Chưa cung cấp")
	pdf := r.pdf
	pdf.SetFont(workflowPDFFontFamily, "", 8.8)
	lines := pdf.SplitText(value, workflowPDFContentWidth-10)
	if len(lines) == 0 {
		lines = []string{"Chưa cung cấp"}
	}
	first := true
	for len(lines) > 0 {
		available := workflowPDFPageBottom - pdf.GetY()
		maxLines := int(math.Floor((available - 12) / 4.5))
		if maxLines < 1 {
			r.addPage(false)
			continue
		}
		count := min(maxLines, len(lines))
		chunk := lines[:count]
		lines = lines[count:]
		height := 10 + float64(len(chunk))*4.5
		y := pdf.GetY()
		pdf.SetFillColor(255, 255, 255)
		pdf.SetDrawColor(226, 232, 240)
		pdf.RoundedRect(workflowPDFMargin, y, workflowPDFContentWidth, height, 1.6, "1234", "FD")
		pdf.SetFillColor(37, 99, 235)
		pdf.Rect(workflowPDFMargin, y+1.6, 1.1, height-3.2, "F")
		pdf.SetXY(workflowPDFMargin+5, y+2.4)
		pdf.SetTextColor(71, 85, 105)
		pdf.SetFont(workflowPDFFontFamily, "B", 7.4)
		shownLabel := label
		if !first {
			shownLabel += " (tiếp theo)"
		}
		pdf.CellFormat(workflowPDFContentWidth-10, 3.7, shownLabel, "", 1, "L", false, 0, "")
		pdf.SetX(workflowPDFMargin + 5)
		if len(chunk) == 1 && chunk[0] == "Chưa cung cấp" {
			pdf.SetTextColor(148, 163, 184)
		} else {
			pdf.SetTextColor(15, 23, 42)
		}
		pdf.SetFont(workflowPDFFontFamily, "", 8.8)
		pdf.MultiCell(workflowPDFContentWidth-10, 4.5, strings.Join(chunk, "\n"), "", "L", false)
		pdf.SetY(y + height + 2)
		first = false
	}
}

func (r *workflowFormPDFRenderer) drawStructuredField(field workflowFormPDFField) {
	rows := workflowPDFRecords(field.Value)
	if len(rows) == 0 {
		r.drawTextField(field.Label, "Chưa cung cấp")
		return
	}
	r.ensureSpace(15)
	pdf := r.pdf
	pdf.SetTextColor(30, 41, 59)
	pdf.SetFont(workflowPDFFontFamily, "B", 8.4)
	pdf.CellFormat(workflowPDFContentWidth, 6, fmt.Sprintf("%s · %d mục", field.Label, len(rows)), "", 1, "L", false, 0, "")
	specs := workflowPDFStructuredSpecs(field.Definition)
	for index, row := range rows {
		itemHeight := workflowPDFStructuredItemHeight(pdf, specs, row)
		if itemHeight <= workflowPDFPageBottom-24 {
			r.ensureSpace(itemHeight)
		}
		r.drawStructuredItemHeader(index + 1)
		if len(specs) == 0 {
			keys := make([]string, 0, len(row))
			for key := range row {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				r.drawStructuredRow(key, workflowPDFValueText(row[key]))
			}
		} else {
			for _, spec := range specs {
				r.drawStructuredRow(workflowPDFText(spec["label"], fmt.Sprint(spec["key"])), workflowPDFDisplayStructuredValue(spec, row[fmt.Sprint(spec["key"])]))
			}
		}
		pdf.SetY(pdf.GetY() + 2)
	}
}

func (r *workflowFormPDFRenderer) drawStructuredItemHeader(index int) {
	r.ensureSpace(9)
	pdf := r.pdf
	pdf.SetFillColor(239, 246, 255)
	pdf.SetTextColor(30, 64, 175)
	pdf.SetFont(workflowPDFFontFamily, "B", 7.7)
	pdf.CellFormat(workflowPDFContentWidth, 7, fmt.Sprintf("  HẠNG MỤC %02d", index), "1", 1, "L", true, 0, "")
}

func (r *workflowFormPDFRenderer) drawStructuredRow(label, value string) {
	value = workflowPDFText(value, "Chưa cung cấp")
	pdf := r.pdf
	labelWidth := 45.0
	valueWidth := workflowPDFContentWidth - labelWidth
	pdf.SetFont(workflowPDFFontFamily, "B", 7.2)
	labelLines := pdf.SplitText(label, labelWidth-5)
	pdf.SetFont(workflowPDFFontFamily, "", 7.7)
	valueLines := pdf.SplitText(value, valueWidth-5)
	lineCount := max(1, max(len(labelLines), len(valueLines)))
	height := math.Max(7, float64(lineCount)*4.2+3)
	if height > workflowPDFPageBottom-24 {
		r.drawTextField(label, value)
		return
	}
	r.ensureSpace(height)
	y := pdf.GetY()
	pdf.SetFillColor(248, 250, 252)
	pdf.SetDrawColor(226, 232, 240)
	pdf.Rect(workflowPDFMargin, y, labelWidth, height, "FD")
	pdf.Rect(workflowPDFMargin+labelWidth, y, valueWidth, height, "D")
	pdf.SetXY(workflowPDFMargin+3, y+1.8)
	pdf.SetTextColor(71, 85, 105)
	pdf.SetFont(workflowPDFFontFamily, "B", 7.2)
	pdf.MultiCell(labelWidth-5, 4.2, label, "", "L", false)
	pdf.SetXY(workflowPDFMargin+labelWidth+3, y+1.8)
	if value == "Chưa cung cấp" {
		pdf.SetTextColor(148, 163, 184)
	} else {
		pdf.SetTextColor(15, 23, 42)
	}
	pdf.SetFont(workflowPDFFontFamily, "", 7.7)
	pdf.MultiCell(valueWidth-5, 4.2, value, "", "L", false)
	pdf.SetY(y + height)
}

func (r *workflowFormPDFRenderer) drawConfirmations() {
	r.ensureSpace(31)
	r.drawSectionTitle("02", "XÁC NHẬN HỒ SƠ", "Các xác nhận thuộc phiên bản nội dung đã chốt")
	if len(r.document.Confirmations) == 0 {
		r.drawEmptyState("Biểu mẫu không yêu cầu xác nhận riêng.")
		return
	}
	for _, field := range r.document.Confirmations {
		r.ensureSpace(14)
		pdf := r.pdf
		y := pdf.GetY()
		confirmed := workflowPDFBool(field.Value)
		if confirmed {
			pdf.SetFillColor(236, 253, 245)
			pdf.SetDrawColor(167, 243, 208)
			pdf.SetTextColor(4, 120, 87)
		} else {
			pdf.SetFillColor(248, 250, 252)
			pdf.SetDrawColor(226, 232, 240)
			pdf.SetTextColor(100, 116, 139)
		}
		pdf.RoundedRect(workflowPDFMargin, y, workflowPDFContentWidth, 11, 1.8, "1234", "FD")
		pdf.SetFont(workflowPDFFontFamily, "B", 8.5)
		status := "Chưa xác nhận"
		if confirmed {
			status = "Đã xác nhận"
		}
		boxX, boxY := workflowPDFMargin+5, y+3.4
		pdf.SetLineWidth(0.35)
		pdf.Rect(boxX, boxY, 3.5, 3.5, "D")
		if confirmed {
			pdf.Line(boxX+0.7, boxY+1.8, boxX+1.5, boxY+2.7)
			pdf.Line(boxX+1.5, boxY+2.7, boxX+3.0, boxY+0.7)
		}
		pdf.SetXY(workflowPDFMargin+13, y+3.2)
		pdf.CellFormat(124, 4, field.Label, "", 0, "L", false, 0, "")
		pdf.SetFont(workflowPDFFontFamily, "B", 7)
		pdf.CellFormat(workflowPDFContentWidth-137, 4, status, "", 0, "R", false, 0, "")
		pdf.SetY(y + 13)
	}
}

func (r *workflowFormPDFRenderer) drawSignatures() {
	minimumHeight := 32.0
	if len(r.document.Signatures) > 0 || len(r.document.SealApplications) > 0 {
		minimumHeight = 69
	}
	r.ensureSpace(minimumHeight)
	r.drawSectionTitle("03", "XÁC NHẬN VÀ CON DẤU", "Dấu vết xác nhận gắn với đúng nội dung và phiên bản hồ sơ")
	if len(r.document.Signatures) == 0 && len(r.document.SealApplications) == 0 {
		r.drawEmptyState("Chưa có chữ ký cá nhân hoặc con dấu công ty.")
		return
	}
	for _, signature := range r.document.Signatures {
		r.drawSignature(signature)
	}
	for _, application := range r.document.SealApplications {
		r.drawSealApplication(application)
	}
}

func (r *workflowFormPDFRenderer) drawSignature(signature Record) {
	r.ensureSpace(51)
	pdf := r.pdf
	x := workflowPDFMargin
	y := pdf.GetY()
	height := 47.0
	pdf.SetFillColor(255, 255, 255)
	pdf.SetDrawColor(203, 213, 225)
	pdf.RoundedRect(x, y, workflowPDFContentWidth, height, 2, "1234", "FD")

	canvasX, canvasY, canvasW, canvasH := x+4, y+8, 64.0, 31.0
	pdf.SetFillColor(248, 250, 252)
	pdf.SetDrawColor(226, 232, 240)
	pdf.RoundedRect(canvasX, canvasY, canvasW, canvasH, 1.5, "1234", "FD")
	strokes := signatureStrokes(signature)
	if len(strokes) == 0 {
		pdf.SetXY(canvasX+3, canvasY+12)
		pdf.SetTextColor(148, 163, 184)
		pdf.SetFont(workflowPDFFontFamily, "", 7)
		pdf.CellFormat(canvasW-6, 5, "Chữ ký audit kiểu cũ", "", 0, "C", false, 0, "")
	} else {
		pdf.SetDrawColor(15, 23, 42)
		pdf.SetLineWidth(0.55)
		for _, stroke := range strokes {
			for pointIndex := 1; pointIndex < len(stroke.Points); pointIndex++ {
				from := stroke.Points[pointIndex-1]
				to := stroke.Points[pointIndex]
				pdf.Line(canvasX+from.X*canvasW, canvasY+from.Y*canvasH, canvasX+to.X*canvasW, canvasY+to.Y*canvasH)
			}
		}
	}

	metaX := canvasX + canvasW + 6
	metaW := workflowPDFContentWidth - canvasW - 14
	pdf.SetXY(metaX, y+5)
	pdf.SetTextColor(30, 64, 175)
	pdf.SetFont(workflowPDFFontFamily, "B", 7)
	pdf.CellFormat(metaW, 4, strings.ToUpper(workflowPDFPurposeLabel(fmt.Sprint(signature["purpose"]))), "", 1, "L", false, 0, "")
	pdf.SetX(metaX)
	pdf.SetTextColor(15, 23, 42)
	pdf.SetFont(workflowPDFFontFamily, "B", 10)
	pdf.CellFormat(metaW, 5.5, workflowPDFText(signature["signer_name"], "Không xác định"), "", 1, "L", false, 0, "")
	pdf.SetX(metaX)
	pdf.SetTextColor(71, 85, 105)
	pdf.SetFont(workflowPDFFontFamily, "", 7.3)
	pdf.CellFormat(metaW, 4.5, "Ký lúc: "+workflowPDFText(workflowPDFDateTime(signature["signed_at"]), "Chưa ghi nhận"), "", 1, "L", false, 0, "")
	pdf.SetX(metaX)
	pdf.SetFont(workflowPDFFontFamily, "", 6.3)
	pdf.CellFormat(metaW, 4, "Hash nội dung: "+workflowPDFShortHash(fmt.Sprint(signature["payload_hash"])), "", 1, "L", false, 0, "")
	pdf.SetX(metaX)
	pdf.CellFormat(metaW, 4, "Checksum chữ ký: "+workflowPDFShortHash(fmt.Sprint(signature["visual_checksum_sha256"])), "", 0, "L", false, 0, "")
	pdf.SetY(y + height + 3)
}

func (r *workflowFormPDFRenderer) drawSealApplication(application Record) {
	r.ensureSpace(51)
	pdf := r.pdf
	x := workflowPDFMargin
	y := pdf.GetY()
	height := 47.0
	pdf.SetFillColor(255, 255, 255)
	pdf.SetDrawColor(203, 213, 225)
	pdf.RoundedRect(x, y, workflowPDFContentWidth, height, 2, "1234", "FD")

	canvasX, canvasY, canvasW, canvasH := x+4, y+8, 64.0, 31.0
	pdf.SetFillColor(248, 250, 252)
	pdf.SetDrawColor(226, 232, 240)
	pdf.RoundedRect(canvasX, canvasY, canvasW, canvasH, 1.5, "1234", "FD")
	applicationID := cleanID(application["id"])
	imageBytes := r.document.SealImages[applicationID]
	if len(imageBytes) == 0 {
		pdf.SetXY(canvasX+3, canvasY+12)
		pdf.SetTextColor(148, 163, 184)
		pdf.SetFont(workflowPDFFontFamily, "", 7)
		pdf.CellFormat(canvasW-6, 5, "Không tải được ảnh con dấu", "", 0, "C", false, 0, "")
	} else {
		imageName := "seal-" + applicationID
		info := pdf.RegisterImageOptionsReader(imageName, fpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}, bytes.NewReader(imageBytes))
		if info != nil {
			maxW, maxH := canvasW-8, canvasH-4
			imageW, imageH := maxW, maxH
			if width := number(application["width_px"]); width > 0 {
				if heightPx := number(application["height_px"]); heightPx > 0 {
					ratio := width / heightPx
					imageH = imageW / ratio
					if imageH > maxH {
						imageH = maxH
						imageW = imageH * ratio
					}
				}
			}
			imageX := canvasX + (canvasW-imageW)/2
			imageY := canvasY + (canvasH-imageH)/2
			pdf.ImageOptions(imageName, imageX, imageY, imageW, imageH, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")
		}
	}

	metaX := canvasX + canvasW + 6
	metaW := workflowPDFContentWidth - canvasW - 14
	pdf.SetXY(metaX, y+5)
	pdf.SetTextColor(30, 64, 175)
	pdf.SetFont(workflowPDFFontFamily, "B", 7)
	pdf.CellFormat(metaW, 4, "CON DẤU CÔNG TY · "+strings.ToUpper(workflowPDFPurposeLabel(fmt.Sprint(application["purpose"]))), "", 1, "L", false, 0, "")
	pdf.SetX(metaX)
	pdf.SetTextColor(15, 23, 42)
	pdf.SetFont(workflowPDFFontFamily, "B", 10)
	pdf.CellFormat(metaW, 5.5, workflowPDFText(application["seal_asset_name"], "Con dấu công ty"), "", 1, "L", false, 0, "")
	pdf.SetX(metaX)
	pdf.SetTextColor(71, 85, 105)
	pdf.SetFont(workflowPDFFontFamily, "", 7.3)
	pdf.CellFormat(metaW, 4.5, "Áp dụng bởi: "+workflowPDFText(application["applied_by_name"], "Không xác định"), "", 1, "L", false, 0, "")
	pdf.SetX(metaX)
	pdf.SetFont(workflowPDFFontFamily, "", 6.3)
	pdf.CellFormat(metaW, 4, "Đóng dấu lúc: "+workflowPDFText(workflowPDFDateTime(application["applied_at"]), "Chưa ghi nhận"), "", 1, "L", false, 0, "")
	pdf.SetX(metaX)
	pdf.CellFormat(metaW, 4, "Hash nội dung: "+workflowPDFShortHash(fmt.Sprint(application["payload_hash"])), "", 1, "L", false, 0, "")
	pdf.SetX(metaX)
	pdf.CellFormat(metaW, 4, "Checksum con dấu: "+workflowPDFShortHash(fmt.Sprint(application["checksum_sha256"])), "", 0, "L", false, 0, "")
	pdf.SetY(y + height + 3)
}

func (r *workflowFormPDFRenderer) drawVerification() {
	r.ensureSpace(64)
	r.drawSectionTitle("04", "THÔNG TIN XÁC THỰC", "Thông tin kỹ thuật phục vụ kiểm tra và đối chiếu hồ sơ")
	rows := [][2]string{
		{"Mã hồ sơ", r.document.WorkItemID},
		{"Mã công trình", r.document.ProjectID},
		{"Loại biểu mẫu", r.document.FormType},
		{"Mã kiểm tra nội dung", r.document.PayloadHash},
		{"Xuất bởi", r.document.GeneratedBy},
		{"Xuất lúc", r.document.GeneratedAt},
	}
	for _, row := range rows {
		r.drawStructuredRow(row[0], workflowPDFText(row[1], "Không xác định"))
	}
	r.pdf.SetY(r.pdf.GetY() + 3)
}

func (r *workflowFormPDFRenderer) drawEmptyState(message string) {
	r.ensureSpace(14)
	pdf := r.pdf
	pdf.SetFillColor(248, 250, 252)
	pdf.SetDrawColor(226, 232, 240)
	pdf.SetTextColor(100, 116, 139)
	pdf.SetFont(workflowPDFFontFamily, "", 8)
	pdf.CellFormat(workflowPDFContentWidth, 11, "  "+message, "1", 1, "L", true, 0, "")
	pdf.SetY(pdf.GetY() + 2)
}

func signatureStrokes(signature Record) []WorkflowFormSignatureStroke {
	payload := workflowPDFRecord(signature["payload_json"])
	visual := workflowPDFRecord(payload["visual_signature"])
	rawStrokes := workflowPDFSlice(visual["strokes"])
	out := make([]WorkflowFormSignatureStroke, 0, len(rawStrokes))
	for _, rawStroke := range rawStrokes {
		strokeMap := workflowPDFRecord(rawStroke)
		rawPoints := workflowPDFSlice(strokeMap["points"])
		points := make([]WorkflowFormSignaturePoint, 0, len(rawPoints))
		for _, rawPoint := range rawPoints {
			pointMap := workflowPDFRecord(rawPoint)
			points = append(points, WorkflowFormSignaturePoint{X: number(pointMap["x"]), Y: number(pointMap["y"])})
		}
		if len(points) >= 2 {
			out = append(out, WorkflowFormSignatureStroke{Points: points})
		}
	}
	return out
}

func workflowPDFDisplayValue(field Record, value any) string {
	fieldType := strings.TrimSpace(fmt.Sprint(field["field_type"]))
	switch fieldType {
	case "select":
		return workflowPDFOptionLabel(field["options_json"], value)
	case "multi_select":
		values := workflowPDFSlice(value)
		labels := make([]string, 0, len(values))
		for _, item := range values {
			labels = append(labels, workflowPDFOptionLabel(field["options_json"], item))
		}
		return workflowPDFNonEmptyJoin(labels, " · ")
	case "checkbox":
		if workflowPDFBool(value) {
			return "Đã xác nhận"
		}
		return "Chưa xác nhận"
	case "date":
		return workflowPDFDate(value)
	case "currency":
		return workflowPDFCurrency(value, workflowPDFRecord(field["validation_json"]))
	case "number":
		return workflowPDFNumber(value)
	case "evidence_reference":
		text := workflowPDFText(value, "")
		if text == "" {
			return "Chưa liên kết minh chứng"
		}
		return "Đã liên kết minh chứng · " + workflowPDFShortHash(text)
	case "structured_list":
		return fmt.Sprintf("%d mục", len(workflowPDFRecords(value)))
	default:
		return workflowPDFValueText(value)
	}
}

func workflowPDFDisplayStructuredValue(spec Record, value any) string {
	field := Record{
		"field_type":      spec["type"],
		"options_json":    spec["options"],
		"validation_json": spec,
	}
	return workflowPDFDisplayValue(field, value)
}

func workflowPDFStructuredSpecs(field Record) []Record {
	validation := workflowPDFRecord(field["validation_json"])
	return workflowPDFRecords(validation["item_fields"])
}

func workflowPDFStructuredItemHeight(pdf *fpdf.Fpdf, specs []Record, row Record) float64 {
	height := 9.0
	if len(specs) == 0 {
		return height
	}
	const labelWidth = 45.0
	const valueWidth = workflowPDFContentWidth - labelWidth
	for _, spec := range specs {
		label := workflowPDFText(spec["label"], fmt.Sprint(spec["key"]))
		value := workflowPDFDisplayStructuredValue(spec, row[fmt.Sprint(spec["key"])])
		pdf.SetFont(workflowPDFFontFamily, "B", 7.2)
		labelLines := pdf.SplitText(label, labelWidth-5)
		pdf.SetFont(workflowPDFFontFamily, "", 7.7)
		valueLines := pdf.SplitText(value, valueWidth-5)
		lineCount := max(1, max(len(labelLines), len(valueLines)))
		height += math.Max(7, float64(lineCount)*4.2+3)
	}
	return height + 2
}

func workflowPDFOptionLabel(rawOptions, value any) string {
	raw := workflowPDFText(value, "")
	if raw == "" {
		return "Chưa cung cấp"
	}
	for _, option := range workflowPDFRecords(rawOptions) {
		if fmt.Sprint(option["value"]) == fmt.Sprint(value) {
			return workflowPDFHumanLabel(workflowPDFText(option["label"], raw))
		}
	}
	return workflowPDFHumanLabel(raw)
}

func workflowPDFHumanLabel(value string) string {
	labels := map[string]string{
		"true": "Có", "false": "Không", "submitted": "Đã chốt", "draft": "Bản nháp",
		"done": "Hoàn tất", "blocked": "Đang vướng", "pending": "Đang chờ", "approved": "Đã duyệt",
		"rejected": "Từ chối", "not_applicable": "Không áp dụng", "N/A": "Không áp dụng",
		"low": "Thấp", "normal": "Bình thường", "high": "Cao", "urgent": "Khẩn cấp",
	}
	if label, ok := labels[strings.TrimSpace(value)]; ok {
		return label
	}
	return value
}

func workflowPDFValueText(value any) string {
	switch typed := value.(type) {
	case nil:
		return "Chưa cung cấp"
	case string:
		return workflowPDFText(typed, "Chưa cung cấp")
	case bool:
		if typed {
			return "Có"
		}
		return "Không"
	case time.Time:
		return workflowPDFDateTime(typed)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, workflowPDFValueText(item))
		}
		return workflowPDFNonEmptyJoin(parts, " · ")
	case []Record:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, workflowPDFValueText(item))
		}
		return workflowPDFNonEmptyJoin(parts, " · ")
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, fmt.Sprintf("%s: %s", key, workflowPDFValueText(typed[key])))
		}
		return workflowPDFNonEmptyJoin(parts, "; ")
	case Record:
		return workflowPDFValueText(map[string]any(typed))
	default:
		return workflowPDFText(fmt.Sprint(value), "Chưa cung cấp")
	}
}

func workflowPDFProjectLabel(context WorkflowFormPDFContext, instance Record) string {
	code := workflowPDFText(context.ProjectCode, "")
	name := workflowPDFText(context.ProjectName, "")
	if code != "" && name != "" {
		return code + " · " + name
	}
	if code != "" {
		return code
	}
	if name != "" {
		return name
	}
	return workflowPDFText(instance["project_id"], "Không xác định")
}

func workflowPDFWorkflowLabel(context WorkflowFormPDFContext, instance Record) string {
	parts := make([]string, 0, 2)
	if context.NodeName != "" {
		label := context.NodeName
		if context.NodeOrder > 0 {
			label = fmt.Sprintf("Node %d: %s", context.NodeOrder, label)
		}
		parts = append(parts, label)
	}
	stepName := workflowPDFText(context.StepName, context.WorkItemTitle)
	if stepName == "" {
		stepName = workflowPDFText(instance["work_item_title"], "")
	}
	if stepName != "" {
		label := stepName
		if context.StepOrder > 0 {
			label = fmt.Sprintf("Bước %d: %s", context.StepOrder, label)
		}
		parts = append(parts, label)
	}
	if len(parts) == 0 {
		return "Đầu việc quy trình"
	}
	return strings.Join(parts, " › ")
}

func workflowPDFFormTypeLabel(value string) string {
	labels := map[string]string{
		"generic_submission": "Hồ sơ gửi kết quả",
		"meeting_minutes":    "Biên bản cuộc họp",
		"checklist":          "Danh sách kiểm tra",
		"daily_report":       "Báo cáo hằng ngày",
	}
	if label, ok := labels[strings.TrimSpace(value)]; ok {
		return label
	}
	return workflowPDFText(value, "Biểu mẫu quy trình")
}

func workflowPDFStatusLabel(value string) string {
	labels := map[string]string{
		"draft": "Bản nháp", "submitted": "Đã chốt", "archived": "Đã lưu trữ",
	}
	if label, ok := labels[strings.TrimSpace(value)]; ok {
		return label
	}
	return workflowPDFText(value, "Không xác định")
}

func workflowPDFPurposeLabel(value string) string {
	labels := map[string]string{
		"internal_ack": "Xác nhận nội bộ", "prepared_by": "Người lập", "reviewed_by": "Người rà soát",
		"approved_by": "Người phê duyệt", "received_by": "Người tiếp nhận",
	}
	if label, ok := labels[strings.TrimSpace(value)]; ok {
		return label
	}
	return workflowPDFText(value, "Chữ ký nội bộ")
}

func workflowPDFDate(value any) string {
	if parsed, ok := workflowPDFTime(value); ok {
		return parsed.In(workflowPDFLocation).Format("02/01/2006")
	}
	return workflowPDFText(value, "Chưa cung cấp")
}

func workflowPDFDateTime(value any) string {
	if parsed, ok := workflowPDFTime(value); ok {
		return parsed.In(workflowPDFLocation).Format("02/01/2006 · 15:04")
	}
	return workflowPDFText(value, "")
}

func workflowPDFTime(value any) (time.Time, bool) {
	switch typed := value.(type) {
	case time.Time:
		return typed, !typed.IsZero()
	case *time.Time:
		if typed != nil && !typed.IsZero() {
			return *typed, true
		}
	case string:
		text := strings.TrimSpace(typed)
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999-07", "2006-01-02 15:04:05-07", "2006-01-02"} {
			if parsed, err := time.Parse(layout, text); err == nil {
				return parsed, true
			}
		}
	}
	return time.Time{}, false
}

func workflowPDFCurrency(value any, validation Record) string {
	amount, ok := workflowPDFFloat(value)
	if !ok {
		return "Chưa cung cấp"
	}
	code := strings.TrimSpace(fmt.Sprint(validation["currency_code"]))
	if code == "" || code == "<nil>" {
		code = "VND"
	}
	digits := 2
	if code == "VND" {
		digits = 0
	} else if raw, exists := validation["maximum_fraction_digits"]; exists {
		digits = int(number(raw))
	}
	return workflowPDFFormatDecimal(amount, digits) + " " + code
}

func workflowPDFNumber(value any) string {
	numeric, ok := workflowPDFFloat(value)
	if !ok {
		return workflowPDFText(value, "Chưa cung cấp")
	}
	formatted := strconv.FormatFloat(numeric, 'f', -1, 64)
	parts := strings.SplitN(formatted, ".", 2)
	parts[0] = workflowPDFGroupThousands(parts[0])
	if len(parts) == 2 {
		return parts[0] + "," + parts[1]
	}
	return parts[0]
}

func workflowPDFFormatDecimal(value float64, digits int) string {
	formatted := strconv.FormatFloat(value, 'f', digits, 64)
	parts := strings.SplitN(formatted, ".", 2)
	parts[0] = workflowPDFGroupThousands(parts[0])
	if len(parts) == 2 && digits > 0 {
		return parts[0] + "," + parts[1]
	}
	return parts[0]
}

func workflowPDFGroupThousands(value string) string {
	sign := ""
	if strings.HasPrefix(value, "-") {
		sign = "-"
		value = strings.TrimPrefix(value, "-")
	}
	for index := len(value) - 3; index > 0; index -= 3 {
		value = value[:index] + "." + value[index:]
	}
	return sign + value
}

func workflowPDFFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func workflowPDFBool(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		parsed, _ := strconv.ParseBool(strings.TrimSpace(typed))
		return parsed
	default:
		return false
	}
}

func workflowPDFShortHash(value string) string {
	value = strings.TrimSpace(value)
	if len([]rune(value)) <= 20 {
		return value
	}
	runes := []rune(value)
	return string(runes[:12]) + "…" + string(runes[len(runes)-6:])
}

func workflowPDFNonEmptyJoin(values []string, separator string) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && value != "Chưa cung cấp" {
			parts = append(parts, value)
		}
	}
	if len(parts) == 0 {
		return "Chưa cung cấp"
	}
	return strings.Join(parts, separator)
}

func workflowPDFRecord(value any) Record {
	switch typed := value.(type) {
	case Record:
		return typed
	case map[string]any:
		return Record(typed)
	default:
		return Record{}
	}
}

func workflowPDFRecords(value any) []Record {
	switch typed := value.(type) {
	case []Record:
		return typed
	case []any:
		rows := make([]Record, 0, len(typed))
		for _, item := range typed {
			row := workflowPDFRecord(item)
			if len(row) > 0 {
				rows = append(rows, row)
			}
		}
		return rows
	default:
		return nil
	}
}

func workflowPDFSlice(value any) []any {
	switch typed := value.(type) {
	case []any:
		return typed
	case []Record:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, item)
		}
		return out
	default:
		return nil
	}
}

func workflowPDFText(value any, fallback string) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return fallback
	}
	return text
}

func asciiFold(value string) string {
	replacer := strings.NewReplacer(
		"à", "a", "á", "a", "ạ", "a", "ả", "a", "ã", "a", "â", "a", "ầ", "a", "ấ", "a", "ậ", "a", "ẩ", "a", "ẫ", "a", "ă", "a", "ằ", "a", "ắ", "a", "ặ", "a", "ẳ", "a", "ẵ", "a",
		"è", "e", "é", "e", "ẹ", "e", "ẻ", "e", "ẽ", "e", "ê", "e", "ề", "e", "ế", "e", "ệ", "e", "ể", "e", "ễ", "e",
		"ì", "i", "í", "i", "ị", "i", "ỉ", "i", "ĩ", "i",
		"ò", "o", "ó", "o", "ọ", "o", "ỏ", "o", "õ", "o", "ô", "o", "ồ", "o", "ố", "o", "ộ", "o", "ổ", "o", "ỗ", "o", "ơ", "o", "ờ", "o", "ớ", "o", "ợ", "o", "ở", "o", "ỡ", "o",
		"ù", "u", "ú", "u", "ụ", "u", "ủ", "u", "ũ", "u", "ư", "u", "ừ", "u", "ứ", "u", "ự", "u", "ử", "u", "ữ", "u",
		"ỳ", "y", "ý", "y", "ỵ", "y", "ỷ", "y", "ỹ", "y", "đ", "d",
		"À", "A", "Á", "A", "Ạ", "A", "Ả", "A", "Ã", "A", "Â", "A", "Ầ", "A", "Ấ", "A", "Ậ", "A", "Ẩ", "A", "Ẫ", "A", "Ă", "A", "Ằ", "A", "Ắ", "A", "Ặ", "A", "Ẳ", "A", "Ẵ", "A",
		"È", "E", "É", "E", "Ẹ", "E", "Ẻ", "E", "Ẽ", "E", "Ê", "E", "Ề", "E", "Ế", "E", "Ệ", "E", "Ể", "E", "Ễ", "E",
		"Ì", "I", "Í", "I", "Ị", "I", "Ỉ", "I", "Ĩ", "I",
		"Ò", "O", "Ó", "O", "Ọ", "O", "Ỏ", "O", "Õ", "O", "Ô", "O", "Ồ", "O", "Ố", "O", "Ộ", "O", "Ổ", "O", "Ỗ", "O", "Ơ", "O", "Ờ", "O", "Ớ", "O", "Ợ", "O", "Ở", "O", "Ỡ", "O",
		"Ù", "U", "Ú", "U", "Ụ", "U", "Ủ", "U", "Ũ", "U", "Ư", "U", "Ừ", "U", "Ứ", "U", "Ự", "U", "Ử", "U", "Ữ", "U",
		"Ỳ", "Y", "Ý", "Y", "Ỵ", "Y", "Ỷ", "Y", "Ỹ", "Y", "Đ", "D",
	)
	folded := replacer.Replace(value)
	var builder strings.Builder
	for _, char := range folded {
		if char == '\n' || char == '\r' || char == '\t' {
			builder.WriteByte(' ')
			continue
		}
		if char >= 32 && char <= 126 {
			builder.WriteRune(char)
			continue
		}
		builder.WriteByte('?')
	}
	return builder.String()
}
