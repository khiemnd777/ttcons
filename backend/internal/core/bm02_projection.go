package core

import (
	"fmt"
	"strconv"
	"strings"
)

// Daily criteria describe supplied observations, never infer an assessment.
// Source sections and native tables retain the full rows behind bounded notes.
func nativeBMGSProjectDailyCriteria(sheet *BMGSSheet, row Record) {
	structured := recordFromAny(row["structured_json"])
	manual := map[string]Record{}
	for _, assessment := range workflowAnswerRecordList(structured["supervision_assessments"]) {
		manual[cleanText(assessment["code"])] = assessment
	}
	notes := map[string]string{}
	for _, section := range sheet.SourceSections {
		code := map[string]string{"Tiến độ được người lập ghi nhận": "progress", "Công việc và khối lượng đã ghi nhận": "completed_quantity", "Kế hoạch tiếp theo đã ghi nhận": "instructions", "Vướng mắc đã ghi nhận": "risk"}[section.Title]
		if code != "" {
			notes[code] = nativeBMGSCriterionSummary(section.Lines, section.Title)
		}
	}
	crew := []string{}
	for _, item := range workflowAnswerRecordList(structured["crew_items"]) {
		if line := nativeBMGSDailyFactLine(item, "crew_name", [][2]string{{"worker_count", "Số người"}, {"work_hours", "Giờ làm việc"}, {"representative", "Đại diện"}, {"contact", "Liên hệ"}, {"note", "Ghi chú"}}); line != "" {
			crew = append(crew, line)
		}
	}
	nativeBMGSAddLines(sheet, "Nhân lực đã ghi nhận", crew)
	notes["workforce"] = nativeBMGSCriterionSummary(crew, "Nhân lực đã ghi nhận")
	deliveries := []string{}
	for _, item := range workflowAnswerRecordList(structured["material_arrival_items"]) {
		if line := nativeBMGSDailyFactLine(item, "material_name", [][2]string{{"quantity", "Khối lượng nhập"}, {"unit", "Đơn vị"}, {"specification", "Quy cách"}, {"origin", "Nguồn gốc"}, {"note", "Ghi chú"}}); line != "" {
			deliveries = append(deliveries, line)
		}
	}
	nativeBMGSAddLines(sheet, "Vật tư nhập trường đã ghi nhận", deliveries)
	notes["deliveries"] = nativeBMGSCriterionSummary(deliveries, "Vật tư nhập trường đã ghi nhận")
	criteria := [][2]string{{"progress", "tiến độ"}, {"workforce", "nhân lực"}, {"deliveries", "vật tư nhập trường"}, {"quality", "chất lượng"}, {"completed_quantity", "khối lượng thực hiện"}, {"variation", "phát sinh"}, {"mep", "MEP"}, {"safety", "an toàn"}, {"acceptance", "nghiệm thu"}, {"risk", "rủi ro"}, {"images", "ảnh đã liên kết"}, {"instructions", "chỉ dẫn hiện trường"}}
	sheet.CriteriaRows = nil
	for _, criterion := range criteria {
		assessment := bmgsCopy(manual[criterion[0]])
		assessment["code"] = criterion[0]
		if bmgsBlank(assessment["assessment"]) {
			assessment["assessment"] = "not_assessed"
		}
		if bmgsBlank(assessment["note"]) {
			assessment["note"] = notes[criterion[0]]
			if bmgsBlank(assessment["note"]) {
				assessment["note"] = "Chưa có ghi nhận " + criterion[1] + " trong nguồn báo cáo."
			}
		}
		sheet.CriteriaRows = append(sheet.CriteriaRows, assessment)
	}
}

// Only the service's protected membership result may supply photo facts.
func nativeBMGSProjectDailyPhotos(sheet *BMGSSheet, row Record, photos []Record) {
	for _, assessment := range workflowAnswerRecordList(recordFromAny(row["structured_json"])["supervision_assessments"]) {
		if cleanText(assessment["code"]) == "images" && !bmgsBlank(assessment["note"]) {
			return
		}
	}
	lines := []string{}
	for index, photo := range photos {
		facts := bmgsCopy(photo)
		// The protected resolver has already selected the exact frozen/live
		// source metadata. Never substitute a downloaded live filename here.
		if bmgsBlank(facts["original_filename"]) {
			facts["original_filename"] = photo["name"]
		}
		if cleanText(facts["original_filename"]) == cleanText(photo["id"]) {
			// Legacy frozen sources use the ID as their filename fallback.
			facts["original_filename"] = nil
		}
		if bmgsBlank(facts["caption"]) {
			facts["caption"] = photo["description"]
		}
		capture := recordFromAny(recordFromAny(photo["payload_json"])["capture_metadata"])
		for _, key := range []string{"location", "captured_at", "photo_phase"} {
			facts[key] = capture[key]
		}
		line := nativeBMGSDailyFactLine(facts, "", [][2]string{{"original_filename", "Tên tệp"}, {"caption", "Chú thích"}, {"location", "Vị trí"}, {"captured_at", "Thời điểm chụp đã ghi nhận"}, {"photo_phase", "Giai đoạn ảnh"}})
		lines = append(lines, bmgsJoin(fmt.Sprintf("Ảnh %d", index+1), line))
	}
	nativeBMGSAddLines(sheet, "Ảnh đã liên kết với nguồn báo cáo", lines)
	for _, criterion := range sheet.CriteriaRows {
		if criterion["code"] == "images" && len(lines) > 0 {
			criterion["note"] = nativeBMGSCriterionSummary(lines, "Ảnh đã liên kết với nguồn báo cáo")
		}
	}
}

func nativeBMGSCriterionSummary(lines []string, reference string) string {
	if len(lines) == 0 {
		return ""
	}
	// Reserve room for the explicit full-source reference. Keep row order and
	// duplicates; do not combine quantities across different units.
	text := strings.Join(lines, "\n")
	runes := []rune(text)
	if len(lines) <= 2 && len(runes) <= 700 {
		return text
	}
	if len(lines) > 2 {
		runes = []rune(strings.Join(lines[:2], "\n"))
	}
	if len(runes) > 700 {
		runes = runes[:700]
	}
	return fmt.Sprintf("%s…\n%s ghi nhận; xem đầy đủ tại %s.", string(runes), strconv.Itoa(len(lines)), reference)
}
