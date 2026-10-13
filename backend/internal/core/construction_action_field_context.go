package core

import (
	"context"
	"database/sql"
	"strings"
)

func normalizeConstructionActionFieldContext(code, location, severity, unit, representative, contact *string) (Record, error) {
	row := Record{}
	for _, field := range []struct {
		key   string
		value *string
		limit int
	}{{"fault_code", code, 80}, {"location", location, 200}, {"severity", severity, 20}, {"responsible_unit", unit, 160}, {"representative", representative, 120}, {"contact", contact, 100}} {
		if field.value == nil {
			continue
		}
		value, err := nativeCaptureText(*field.value, field.key, field.limit)
		if err != nil {
			return nil, err
		}
		row[field.key] = value
	}
	if value := strings.TrimSpace(cleanText(row["severity"])); value != "" && !statusIn(value, "low", "medium", "high", "critical") {
		return nil, validationError("Mức độ lỗi không hợp lệ", nil)
	}
	return row, nil
}

func saveConstructionActionFieldContextTx(ctx context.Context, tx *sql.Tx, action, fields Record, actorID, actorName string) (Record, error) {
	if len(fields) == 0 {
		return action, nil
	}
	updated, err := queryOneRecord(ctx, tx, `UPDATE construction_action_items SET fault_code=CASE WHEN $2::text IS NULL THEN fault_code ELSE NULLIF($2,'') END,location=CASE WHEN $3::text IS NULL THEN location ELSE NULLIF($3,'') END,severity=CASE WHEN $4::text IS NULL THEN severity ELSE NULLIF($4,'') END,responsible_unit=CASE WHEN $5::text IS NULL THEN responsible_unit ELSE NULLIF($5,'') END,representative=CASE WHEN $6::text IS NULL THEN representative ELSE NULLIF($6,'') END,contact=CASE WHEN $7::text IS NULL THEN contact ELSE NULLIF($7,'') END WHERE id=$1 RETURNING *`,
		cleanID(action["id"]), fields["fault_code"], fields["location"], fields["severity"], fields["responsible_unit"], fields["representative"], fields["contact"])
	if err != nil {
		return nil, err
	}
	before := Record{}
	for key := range fields {
		before[key] = action[key]
	}
	if err = addAuditTx(ctx, tx, cleanID(action["project_id"]), actorID, actorName, "construction_action_field_context_updated", "construction_action", cleanID(action["id"]), "Ghi nhận thông tin lỗi và đơn vị xử lý", Record{"before": before, "after": fields}); err != nil {
		return nil, err
	}
	return updated, nil
}
