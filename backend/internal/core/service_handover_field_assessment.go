package core

import (
	"context"
	"time"
)

// Native observation writes accept omission (preserve) and blank text (clear).
// Reject null and unknown fields so a mistyped payload cannot silently lose data.
func normalizeHandoverFieldAssessment(body Record, header, create bool) (Record, error) {
	limits := map[string]int{"requirement": 1000, "physical_result": 20, "defect": 1000, "due_date": 10}
	if header {
		limits = map[string]int{"handover_stage": 160, "handover_conclusion": 20}
	}
	if create {
		limits["title"] = 300
	}
	result := Record{}
	for key, raw := range body {
		limit, allowed := limits[key]
		if !allowed {
			return nil, validationError("Trường đánh giá bàn giao không hợp lệ", Record{"field": key})
		}
		text, valid := raw.(string)
		if !valid {
			return nil, validationError("Trường đánh giá bàn giao cần chuỗi", Record{"field": key})
		}
		value, err := nativeCaptureText(text, key, limit)
		if err != nil {
			return nil, err
		}
		result[key] = value
	}
	if value := cleanText(result["physical_result"]); value != "" && !statusIn(value, "pass", "fail", "not_assessed") {
		return nil, validationError("Đánh giá thực tế bàn giao không hợp lệ")
	}
	if value := cleanText(result["handover_conclusion"]); value != "" && !statusIn(value, "eligible", "not_eligible") {
		return nil, validationError("Kết luận bàn giao không hợp lệ")
	}
	if value := cleanText(result["due_date"]); value != "" {
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return nil, validationError("Thời hạn xử lý cần ngày hợp lệ")
		}
	}
	if create && cleanText(result["title"]) == "" {
		return nil, validationError("Hạng mục bàn giao cần tên")
	}
	if !create && len(result) == 0 {
		return nil, validationError("Cần ít nhất một trường đánh giá bàn giao")
	}
	return result, nil
}

func (s *Service) UpdateHandoverFieldAssessment(ctx context.Context, id string, body Record, actorID, actorName string) (Record, error) {
	normalized, err := normalizeHandoverFieldAssessment(body, true, false)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateHandoverFieldAssessment(ctx, cleanID(id), normalized, actorID, actorName)
}

func (s *Service) UpdateHandoverItemFieldAssessment(ctx context.Context, id string, body Record, actorID, actorName string) (Record, error) {
	normalized, err := normalizeHandoverFieldAssessment(body, false, false)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateHandoverItemFieldAssessment(ctx, cleanID(id), normalized, actorID, actorName)
}

func (s *Service) CreateHandoverPhysicalItem(ctx context.Context, id string, body Record, actorID, actorName string) (Record, error) {
	normalized, err := normalizeHandoverFieldAssessment(body, false, true)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateHandoverPhysicalItem(ctx, cleanID(id), normalized, actorID, actorName)
}
