package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	pdfPageWidth  = 595.0
	pdfPageHeight = 842.0
)

type simplePDF struct {
	pages []*simplePDFPage
}

type simplePDFPage struct {
	content strings.Builder
}

func GenerateWorkflowFormPDF(form Record) ([]byte, string, error) {
	rawSnapshot, _ := json.Marshal(workflowFormPDFSnapshot(form))
	hash := sha256.Sum256(rawSnapshot)
	payloadHash := hex.EncodeToString(hash[:])

	doc := &simplePDF{}
	page := doc.addPage()
	y := 46.0
	addLine := func(size float64, text string) {
		if y > 780 {
			page = doc.addPage()
			y = 46
		}
		page.text(48, y, size, text)
		y += size + 8
	}
	addGap := func(height float64) {
		y += height
		if y > 780 {
			page = doc.addPage()
			y = 46
		}
	}

	instance, _ := form["instance"].(Record)
	template, _ := form["template"].(Record)
	version, _ := form["version"].(Record)
	fields, _ := form["fields"].([]Record)
	answers, _ := form["answers"].(Record)
	signatures, _ := form["signatures"].([]Record)

	addLine(18, fmt.Sprintf("Workflow Form Export - %s", instance["title"]))
	addLine(10, fmt.Sprintf("Template: %s v%v", template["name"], version["version_no"]))
	addLine(10, fmt.Sprintf("Form type: %s | Status: %s | Revision: %v", instance["form_type"], instance["status"], instance["current_revision"]))
	addLine(10, fmt.Sprintf("Project: %s | Work item: %s", instance["project_id"], instance["work_item_id"]))
	addLine(9, fmt.Sprintf("Payload hash: %s", payloadHash))
	addGap(10)

	addLine(14, "Answers")
	for _, field := range fields {
		key := fmt.Sprint(field["field_key"])
		label := fmt.Sprint(field["label"])
		valueText := workflowPDFValueText(answers[key])
		lines := wrapPDFText(fmt.Sprintf("%s: %s", label, valueText), 92)
		for _, line := range lines {
			addLine(9, line)
		}
		addGap(2)
	}

	addGap(8)
	addLine(14, "Visual Signature Stamps")
	if len(signatures) == 0 {
		addLine(9, "No internal signatures recorded.")
	} else {
		for _, signature := range signatures {
			if y+118 > 780 {
				page = doc.addPage()
				y = 46
			}
			drawWorkflowSignatureStamp(page, 48, y, 498, 104, signature)
			y += 118
		}
	}

	return doc.bytes(), payloadHash, nil
}

func workflowFormPDFSnapshot(form Record) Record {
	return Record{
		"instance":   form["instance"],
		"template":   form["template"],
		"version":    form["version"],
		"fields":     form["fields"],
		"answers":    form["answers"],
		"signatures": form["signatures"],
	}
}

func drawWorkflowSignatureStamp(page *simplePDFPage, x, y, width, height float64, signature Record) {
	page.rect(x, y, width, height)
	page.text(x+12, y+18, 11, fmt.Sprintf("SIGNED BY %s", signature["signer_name"]))
	page.text(x+12, y+34, 9, fmt.Sprintf("Purpose: %s", signature["purpose"]))
	page.text(x+12, y+48, 9, fmt.Sprintf("Signed at: %s", signature["signed_at"]))
	page.text(x+12, y+62, 8, fmt.Sprintf("Hash: %s", signature["payload_hash"]))
	page.text(x+12, y+76, 8, fmt.Sprintf("Visual checksum: %s", signature["visual_checksum_sha256"]))
	strokes := signatureStrokes(signature)
	if len(strokes) == 0 {
		page.text(x+320, y+56, 8, "Legacy audit-only signature")
		return
	}
	drawX := x + 310
	drawY := y + 18
	drawW := 164.0
	drawH := 66.0
	page.rect(drawX, drawY, drawW, drawH)
	for _, stroke := range strokes {
		if len(stroke.Points) < 2 {
			continue
		}
		points := make([]WorkflowFormSignaturePoint, 0, len(stroke.Points))
		for _, point := range stroke.Points {
			points = append(points, WorkflowFormSignaturePoint{
				X: drawX + point.X*drawW,
				Y: drawY + point.Y*drawH,
			})
		}
		page.stroke(points)
	}
}

func signatureStrokes(signature Record) []WorkflowFormSignatureStroke {
	payload, ok := signature["payload_json"].(map[string]any)
	if !ok {
		if record, ok := signature["payload_json"].(Record); ok {
			payload = map[string]any(record)
		}
	}
	visual, _ := payload["visual_signature"].(map[string]any)
	if visual == nil {
		if record, ok := payload["visual_signature"].(Record); ok {
			visual = map[string]any(record)
		}
	}
	rawStrokes, _ := visual["strokes"].([]any)
	out := make([]WorkflowFormSignatureStroke, 0, len(rawStrokes))
	for _, rawStroke := range rawStrokes {
		strokeMap, _ := rawStroke.(map[string]any)
		if strokeMap == nil {
			if record, ok := rawStroke.(Record); ok {
				strokeMap = map[string]any(record)
			}
		}
		rawPoints, _ := strokeMap["points"].([]any)
		points := make([]WorkflowFormSignaturePoint, 0, len(rawPoints))
		for _, rawPoint := range rawPoints {
			pointMap, _ := rawPoint.(map[string]any)
			if pointMap == nil {
				if record, ok := rawPoint.(Record); ok {
					pointMap = map[string]any(record)
				}
			}
			points = append(points, WorkflowFormSignaturePoint{X: number(pointMap["x"]), Y: number(pointMap["y"])})
		}
		if len(points) >= 2 {
			out = append(out, WorkflowFormSignatureStroke{Points: points})
		}
	}
	return out
}

func workflowPDFValueText(value any) string {
	switch v := value.(type) {
	case nil:
		return "-"
	case string:
		if strings.TrimSpace(v) == "" {
			return "-"
		}
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, workflowPDFValueText(item))
		}
		return strings.Join(parts, " | ")
	case []Record:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, workflowPDFValueText(item))
		}
		return strings.Join(parts, " | ")
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, fmt.Sprintf("%s=%s", key, workflowPDFValueText(v[key])))
		}
		return strings.Join(parts, ", ")
	case Record:
		return workflowPDFValueText(map[string]any(v))
	default:
		return fmt.Sprint(value)
	}
}

func wrapPDFText(value string, max int) []string {
	words := strings.Fields(asciiFold(value))
	if len(words) == 0 {
		return []string{"-"}
	}
	lines := []string{}
	current := ""
	for _, word := range words {
		if len(current)+len(word)+1 > max && current != "" {
			lines = append(lines, current)
			current = word
			continue
		}
		if current == "" {
			current = word
		} else {
			current += " " + word
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

func (d *simplePDF) addPage() *simplePDFPage {
	page := &simplePDFPage{}
	d.pages = append(d.pages, page)
	return page
}

func (p *simplePDFPage) text(x, y, size float64, text string) {
	p.content.WriteString(fmt.Sprintf("BT /F1 %.1f Tf %.1f %.1f Td (%s) Tj ET\n", size, x, pdfY(y), pdfEscape(asciiFold(text))))
}

func (p *simplePDFPage) rect(x, y, width, height float64) {
	p.content.WriteString(fmt.Sprintf("q 0.8 w 0.15 0.24 0.36 RG %.1f %.1f %.1f %.1f re S Q\n", x, pdfY(y+height), width, height))
}

func (p *simplePDFPage) stroke(points []WorkflowFormSignaturePoint) {
	if len(points) < 2 {
		return
	}
	p.content.WriteString("q 1.4 w 0.02 0.08 0.18 RG ")
	p.content.WriteString(fmt.Sprintf("%.1f %.1f m ", points[0].X, pdfY(points[0].Y)))
	for _, point := range points[1:] {
		p.content.WriteString(fmt.Sprintf("%.1f %.1f l ", point.X, pdfY(point.Y)))
	}
	p.content.WriteString("S Q\n")
}

func (d *simplePDF) bytes() []byte {
	if len(d.pages) == 0 {
		d.addPage()
	}
	fontID := 3
	nextID := 4
	pageIDs := make([]int, len(d.pages))
	contentIDs := make([]int, len(d.pages))
	for index := range d.pages {
		pageIDs[index] = nextID
		contentIDs[index] = nextID + 1
		nextID += 2
	}
	objects := map[int]string{}
	objects[1] = "<< /Type /Catalog /Pages 2 0 R >>"
	kids := make([]string, 0, len(pageIDs))
	for _, id := range pageIDs {
		kids = append(kids, fmt.Sprintf("%d 0 R", id))
	}
	objects[2] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pageIDs))
	objects[fontID] = "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"
	for index, page := range d.pages {
		content := page.content.String()
		objects[pageIDs[index]] = fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources << /Font << /F1 %d 0 R >> >> /Contents %d 0 R >>", pdfPageWidth, pdfPageHeight, fontID, contentIDs[index])
		objects[contentIDs[index]] = fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content)
	}
	maxID := nextID - 1
	var buffer bytes.Buffer
	buffer.WriteString("%PDF-1.4\n")
	offsets := make([]int, maxID+1)
	for id := 1; id <= maxID; id++ {
		offsets[id] = buffer.Len()
		buffer.WriteString(strconv.Itoa(id))
		buffer.WriteString(" 0 obj\n")
		buffer.WriteString(objects[id])
		buffer.WriteString("\nendobj\n")
	}
	xrefOffset := buffer.Len()
	buffer.WriteString(fmt.Sprintf("xref\n0 %d\n", maxID+1))
	buffer.WriteString("0000000000 65535 f \n")
	for id := 1; id <= maxID; id++ {
		buffer.WriteString(fmt.Sprintf("%010d 00000 n \n", offsets[id]))
	}
	buffer.WriteString(fmt.Sprintf("trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", maxID+1, xrefOffset))
	return buffer.Bytes()
}

func pdfY(topY float64) float64 {
	return pdfPageHeight - topY
}

func pdfEscape(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "(", "\\(")
	value = strings.ReplaceAll(value, ")", "\\)")
	return value
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
