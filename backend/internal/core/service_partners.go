package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (s *Service) ListPartners(ctx context.Context, filter PartnerFilter) ([]Record, error) {
	filter.Query = strings.TrimSpace(filter.Query)
	filter.Type = strings.TrimSpace(filter.Type)
	filter.Group = strings.TrimSpace(filter.Group)
	filter.Status = strings.TrimSpace(filter.Status)
	if filter.Type != "" && !validPartnerType(filter.Type) {
		return nil, validationError("Loại đối tác không hợp lệ", Record{"partner_type": filter.Type})
	}
	if filter.Group != "" && !statusIn(filter.Group, "input", "output") {
		return nil, validationError("Nhóm đối tác không hợp lệ", Record{"group": filter.Group})
	}
	if filter.Group == "input" && filter.Type != "" && filter.Type != "client" {
		return nil, validationError("Loại đối tác không thuộc nhóm Đầu vào", Record{"group": filter.Group, "partner_type": filter.Type})
	}
	if filter.Group == "output" && filter.Type == "client" {
		return nil, validationError("Loại đối tác không thuộc nhóm Đầu ra", Record{"group": filter.Group, "partner_type": filter.Type})
	}
	if filter.Status != "" && !statusIn(filter.Status, "active", "inactive", "all") {
		return nil, validationError("Trạng thái đối tác không hợp lệ", Record{"status": filter.Status})
	}
	return s.repo.ListPartners(ctx, filter)
}

func (s *Service) CreatePartner(ctx context.Context, body Record, actorID, actorName string) (Record, error) {
	normalized, err := normalizePartnerBody(body, true)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.insertWithAudit(ctx, "partners", partnerColumns(), normalized, func(row Record) auditEvent {
		return auditEvent{
			ActorID: actorID, ActorName: actorName, EventType: "partner_created", TargetType: "partner",
			Description: fmt.Sprintf("Create partner %s", row["name"]),
			Metadata:    Record{"partner_code": row["partner_code"], "partner_types": row["partner_types"]},
		}
	})
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (s *Service) UpdatePartner(ctx context.Context, id string, body Record, actorID, actorName string) (Record, error) {
	normalized, err := normalizePartnerBody(body, false)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.updateWithAudit(ctx, "partners", partnerColumns(), id, normalized, func(row Record) auditEvent {
		return auditEvent{
			ActorID: actorID, ActorName: actorName, EventType: "partner_updated", TargetType: "partner",
			Description: fmt.Sprintf("Update partner %s", row["name"]),
			Metadata:    Record{"partner_code": row["partner_code"], "partner_types": row["partner_types"]},
		}
	})
	if err != nil {
		return nil, err
	}
	return row, nil
}

func normalizePartnerBody(body Record, create bool) (Record, error) {
	if body == nil {
		body = Record{}
	}
	normalized := Record{}
	for key, value := range body {
		normalized[key] = value
	}
	if create && cleanOptionalText(normalized["partner_code"]) == "" {
		normalized["partner_code"] = fmt.Sprintf("PT-%d", time.Now().Unix())
	}
	for _, field := range []string{"partner_code", "name", "legal_name", "tax_code", "phone", "email", "address", "contact_name", "notes"} {
		if _, ok := normalized[field]; ok {
			normalized[field] = strings.TrimSpace(fmt.Sprint(normalized[field]))
		}
	}
	if create || normalized["name"] != nil {
		if cleanOptionalText(normalized["name"]) == "" {
			return nil, validationError("Tên đối tác là bắt buộc")
		}
	}
	for _, field := range []string{"partner_code", "name", "legal_name", "tax_code", "phone", "email", "contact_name"} {
		if len([]rune(cleanOptionalText(normalized[field]))) > 240 {
			return nil, validationError("Thông tin đối tác không được vượt quá 240 ký tự", Record{"field": field})
		}
	}
	if len([]rune(cleanOptionalText(normalized["address"]))) > 1000 {
		return nil, validationError("Địa chỉ đối tác không được vượt quá 1000 ký tự")
	}
	if len([]rune(cleanOptionalText(normalized["notes"]))) > 2000 {
		return nil, validationError("Ghi chú đối tác không được vượt quá 2000 ký tự")
	}
	if create || normalized["partner_types"] != nil {
		types, err := normalizePartnerTypes(normalized["partner_types"])
		if err != nil {
			return nil, err
		}
		if len(types) == 0 {
			return nil, validationError("Cần chọn ít nhất một loại đối tác")
		}
		normalized["partner_types"] = types
	}
	if create || normalized["payment_terms_days"] != nil {
		days := int(number(normalized["payment_terms_days"]))
		if days < 0 {
			return nil, validationError("Điều khoản thanh toán không được âm")
		}
		normalized["payment_terms_days"] = days
	}
	if create && normalized["active"] == nil {
		normalized["active"] = true
	}
	return normalized, nil
}

func normalizePartnerTypes(value any) ([]string, error) {
	out := make([]string, 0)
	seen := map[string]bool{}
	add := func(raw any) error {
		for _, part := range strings.Split(fmt.Sprint(raw), ",") {
			value := strings.TrimSpace(part)
			if value == "" || value == "<nil>" {
				continue
			}
			if !validPartnerType(value) {
				return validationError("Loại đối tác không hợp lệ", Record{"partner_type": value})
			}
			if !seen[value] {
				seen[value] = true
				out = append(out, value)
			}
		}
		return nil
	}
	switch typed := value.(type) {
	case nil:
		return out, nil
	case []string:
		for _, item := range typed {
			if err := add(item); err != nil {
				return nil, err
			}
		}
	case []any:
		for _, item := range typed {
			if err := add(item); err != nil {
				return nil, err
			}
		}
	case string:
		text := strings.TrimSpace(typed)
		if strings.HasPrefix(text, "[") {
			var items []string
			if err := json.Unmarshal([]byte(text), &items); err == nil {
				for _, item := range items {
					if err := add(item); err != nil {
						return nil, err
					}
				}
				return out, nil
			}
		}
		if err := add(typed); err != nil {
			return nil, err
		}
	default:
		if err := add(typed); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func validPartnerType(value string) bool {
	return statusIn(value, "client", "supplier", "subcontractor", "labor_team", "transport", "equipment_rental", "service_provider", "other")
}

func vendorPartnerTypes() []string {
	return []string{"supplier", "subcontractor", "labor_team", "transport", "equipment_rental", "service_provider", "other"}
}

func (s *Service) applyPartnerSnapshot(ctx context.Context, body Record, idField, nameField, taxCodeField string, allowedTypes []string) error {
	if body == nil {
		return nil
	}
	rawID, exists := body[idField]
	if !exists {
		return nil
	}
	partnerID := cleanID(rawID)
	if partnerID == "" {
		body[idField] = nil
		return nil
	}
	partner, err := s.requireActivePartner(ctx, partnerID, allowedTypes)
	if err != nil {
		return err
	}
	body[idField] = partnerID
	body[nameField] = cleanOptionalText(partner["name"])
	if taxCodeField != "" {
		body[taxCodeField] = cleanOptionalText(partner["tax_code"])
	}
	return nil
}

func (s *Service) requireActivePartner(ctx context.Context, partnerID string, allowedTypes []string) (Record, error) {
	if !validActivityUUID(partnerID) {
		return nil, validationError("partner_id không hợp lệ", Record{"partner_id": partnerID})
	}
	partner, err := s.repo.GetByID(ctx, "partners", partnerID)
	if err != nil {
		return nil, validationError("Đối tác không tồn tại", Record{"partner_id": partnerID})
	}
	if active, ok := partner["active"].(bool); ok && !active {
		return nil, validationError("Đối tác đang không hoạt động", Record{"partner_id": partnerID})
	}
	if len(allowedTypes) > 0 && !partnerHasAnyType(partner, allowedTypes) {
		return nil, validationError("Loại đối tác không phù hợp nghiệp vụ", Record{"partner_id": partnerID, "allowed_types": allowedTypes, "partner_types": partner["partner_types"]})
	}
	return partner, nil
}

func partnerHasAnyType(partner Record, allowedTypes []string) bool {
	types := map[string]bool{}
	switch raw := partner["partner_types"].(type) {
	case []any:
		for _, item := range raw {
			types[fmt.Sprint(item)] = true
		}
	case []string:
		for _, item := range raw {
			types[item] = true
		}
	case string:
		parsed, _ := normalizePartnerTypes(raw)
		for _, item := range parsed {
			types[item] = true
		}
	}
	for _, allowed := range allowedTypes {
		if types[allowed] {
			return true
		}
	}
	return false
}
