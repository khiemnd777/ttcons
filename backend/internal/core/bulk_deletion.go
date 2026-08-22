package core

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
)

const bulkDeletionMaximumRows = 100

type partnerDeletionTarget struct {
	ID           string
	Code         string
	Name         string
	PartnerTypes []string
	Active       bool
	UpdatedAt    string
	Impact       Record
}

type projectDeletionTarget struct {
	ID        string
	Code      string
	Name      string
	Status    string
	UpdatedAt string
	Impact    Record
}

type partnerDeletionValidation struct {
	Preview Record
	Targets []partnerDeletionTarget
	Hash    string
}

type projectDeletionValidation struct {
	Preview Record
	Targets []projectDeletionTarget
	Hash    string
}

func (s *Service) PreviewPartnerDeletion(ctx context.Context, actorID string, req PartnerDeletionPreviewRequest) (Record, error) {
	validation, err := s.validatePartnerDeletion(ctx, actorID, req)
	if err != nil {
		return nil, err
	}
	return validation.Preview, nil
}

func (s *Service) ExecutePartnerDeletion(ctx context.Context, actorID, actorName string, req PartnerDeletionExecuteRequest) (Record, error) {
	idempotencyKey, previewHash, err := validateBulkDeletionExecution(req.IdempotencyKey, req.PreviewHash)
	if err != nil {
		return nil, err
	}
	if stored, found, err := s.repo.BulkDeletionResult(ctx, "partner", actorID, idempotencyKey, previewHash); err != nil {
		return nil, err
	} else if found {
		stored["idempotent_replay"] = true
		return stored, nil
	}
	validation, err := s.validatePartnerDeletion(ctx, actorID, req.PartnerDeletionPreviewRequest)
	if err != nil {
		return nil, err
	}
	if previewHash != validation.Hash {
		return nil, conflictError("Dữ liệu hoặc liên kết đối tác đã thay đổi sau bước kiểm tra. Vui lòng kiểm tra lại", Record{"field": "preview_hash"})
	}
	return s.repo.ExecutePartnerDeletion(ctx, actorID, actorName, idempotencyKey, previewHash, validation.Targets)
}

func (s *Service) validatePartnerDeletion(ctx context.Context, actorID string, req PartnerDeletionPreviewRequest) (partnerDeletionValidation, error) {
	partnerIDs, err := normalizedBulkDeletionIDs(req.PartnerIDs, "partner_ids", "đối tác")
	if err != nil {
		return partnerDeletionValidation{}, err
	}
	targets, err := s.repo.PartnerDeletionTargets(ctx, partnerIDs)
	if err != nil {
		return partnerDeletionValidation{}, err
	}
	if len(targets) != len(partnerIDs) {
		return partnerDeletionValidation{}, conflictError("Một hoặc nhiều đối tác không tồn tại hoặc đã bị vô hiệu hóa", Record{"partner_ids": missingBulkDeletionIDs(partnerIDs, partnerDeletionTargetIDs(targets))})
	}

	rows := make([]Record, 0, len(targets))
	warningCount := 0
	impactSummary := Record{"business_reference_count": 0, "shared_catalog_count": 0}
	for _, target := range targets {
		groups := partnerCatalogGroups(target.PartnerTypes)
		warnings := partnerDeletionWarnings(groups, target.Impact)
		warningCount += len(warnings)
		businessReferences := partnerImpactTotal(target.Impact)
		impactSummary["business_reference_count"] = number(impactSummary["business_reference_count"]) + float64(businessReferences)
		if len(groups) > 1 {
			impactSummary["shared_catalog_count"] = number(impactSummary["shared_catalog_count"]) + 1
		}
		rows = append(rows, Record{
			"partner_id": target.ID, "partner_code": target.Code, "name": target.Name,
			"partner_types": target.PartnerTypes, "catalog_groups": groups, "can_delete": true,
			"blockers": []Record{}, "warnings": warnings, "impact": target.Impact, "revision": target.UpdatedAt,
		})
	}
	hash, err := s.bulkDeletionPreviewHash(actorID, "partner", rows, impactSummary)
	if err != nil {
		return partnerDeletionValidation{}, err
	}
	preview := Record{
		"total_count": len(targets), "deletable_count": len(targets), "blocked_count": 0,
		"warning_count": warningCount, "impact_summary": impactSummary, "preview_hash": hash, "rows": rows,
	}
	return partnerDeletionValidation{Preview: preview, Targets: targets, Hash: hash}, nil
}

func (s *Service) PreviewProjectDeletion(ctx context.Context, actorID string, req ProjectDeletionPreviewRequest) (Record, error) {
	validation, err := s.validateProjectDeletion(ctx, actorID, req)
	if err != nil {
		return nil, err
	}
	return validation.Preview, nil
}

func (s *Service) ExecuteProjectDeletion(ctx context.Context, actorID, actorName string, req ProjectDeletionExecuteRequest) (Record, error) {
	idempotencyKey, previewHash, err := validateBulkDeletionExecution(req.IdempotencyKey, req.PreviewHash)
	if err != nil {
		return nil, err
	}
	if stored, found, err := s.repo.BulkDeletionResult(ctx, "project", actorID, idempotencyKey, previewHash); err != nil {
		return nil, err
	} else if found {
		stored["idempotent_replay"] = true
		return stored, nil
	}
	validation, err := s.validateProjectDeletion(ctx, actorID, req.ProjectDeletionPreviewRequest)
	if err != nil {
		return nil, err
	}
	if previewHash != validation.Hash {
		return nil, conflictError("Dữ liệu hoặc liên kết công trình đã thay đổi sau bước kiểm tra. Vui lòng kiểm tra lại", Record{"field": "preview_hash"})
	}
	result, err := s.repo.ExecuteProjectDeletion(ctx, actorID, actorName, idempotencyKey, previewHash, validation.Targets)
	if err != nil {
		return nil, err
	}
	for _, target := range validation.Targets {
		s.publishProjectProgressChanged(target.ID)
	}
	return result, nil
}

func (s *Service) validateProjectDeletion(ctx context.Context, actorID string, req ProjectDeletionPreviewRequest) (projectDeletionValidation, error) {
	projectIDs, err := normalizedBulkDeletionIDs(req.ProjectIDs, "project_ids", "công trình")
	if err != nil {
		return projectDeletionValidation{}, err
	}
	for _, projectID := range projectIDs {
		allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"project.delete"})
		if err != nil {
			return projectDeletionValidation{}, err
		}
		if !allowed {
			return projectDeletionValidation{}, forbiddenError("Bạn không có quyền xóa một hoặc nhiều công trình đã chọn", Record{"permission": "project.delete"})
		}
	}
	targets, err := s.repo.ProjectDeletionTargets(ctx, projectIDs)
	if err != nil {
		return projectDeletionValidation{}, err
	}
	if len(targets) != len(projectIDs) {
		return projectDeletionValidation{}, conflictError("Một hoặc nhiều công trình không tồn tại hoặc đã bị hủy", Record{"project_ids": missingBulkDeletionIDs(projectIDs, projectDeletionTargetIDs(targets))})
	}
	rows := make([]Record, 0, len(targets))
	warningCount := 0
	impactSummary := Record{
		"workflow_count": 0, "open_work_item_count": 0, "open_issue_count": 0,
		"finance_record_count": 0, "stock_movement_count": 0, "acceptance_count": 0,
	}
	for _, target := range targets {
		warnings := projectDeletionWarnings(target.Impact)
		warningCount += len(warnings)
		for key := range impactSummary {
			impactSummary[key] = number(impactSummary[key]) + number(target.Impact[key])
		}
		rows = append(rows, Record{
			"project_id": target.ID, "code": target.Code, "name": target.Name, "status": target.Status,
			"can_delete": true, "blockers": []Record{}, "warnings": warnings, "impact": target.Impact, "revision": target.UpdatedAt,
		})
	}
	hash, err := s.bulkDeletionPreviewHash(actorID, "project", rows, impactSummary)
	if err != nil {
		return projectDeletionValidation{}, err
	}
	preview := Record{
		"total_count": len(targets), "deletable_count": len(targets), "blocked_count": 0,
		"warning_count": warningCount, "impact_summary": impactSummary, "preview_hash": hash, "rows": rows,
	}
	return projectDeletionValidation{Preview: preview, Targets: targets, Hash: hash}, nil
}

func (s *Service) bulkDeletionPreviewHash(actorID, entityType string, rows []Record, impactSummary Record) (string, error) {
	payload, err := json.Marshal(Record{"actor_id": actorID, "entity_type": entityType, "rows": rows, "impact_summary": impactSummary})
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, []byte(s.jwtSecret))
	if _, err := mac.Write(payload); err != nil {
		return "", err
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func validateBulkDeletionExecution(rawKey, rawHash string) (string, string, error) {
	idempotencyKey := strings.TrimSpace(rawKey)
	previewHash := strings.TrimSpace(rawHash)
	if _, err := uuid.Parse(idempotencyKey); err != nil {
		return "", "", validationError("Mã chống xóa trùng không hợp lệ", Record{"field": "idempotency_key"})
	}
	if len(previewHash) != sha256.Size*2 {
		return "", "", validationError("Mã kiểm tra xem trước không hợp lệ", Record{"field": "preview_hash"})
	}
	return idempotencyKey, previewHash, nil
}

func normalizedBulkDeletionIDs(rawIDs []string, field, label string) ([]string, error) {
	if len(rawIDs) == 0 || len(rawIDs) > bulkDeletionMaximumRows {
		return nil, validationError(fmt.Sprintf("Mỗi lần xóa phải chọn từ 1 đến %d %s", bulkDeletionMaximumRows, label), Record{"field": field, "maximum": bulkDeletionMaximumRows})
	}
	seen := map[string]bool{}
	ids := make([]string, 0, len(rawIDs))
	for _, rawID := range rawIDs {
		parsed, err := uuid.Parse(strings.TrimSpace(rawID))
		if err != nil {
			return nil, validationError(fmt.Sprintf("Mã %s không hợp lệ", label), Record{"field": field, "value": rawID})
		}
		id := parsed.String()
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func missingBulkDeletionIDs(requested, found []string) []string {
	set := make(map[string]bool, len(found))
	for _, id := range found {
		set[id] = true
	}
	missing := []string{}
	for _, id := range requested {
		if !set[id] {
			missing = append(missing, id)
		}
	}
	return missing
}

func partnerDeletionTargetIDs(targets []partnerDeletionTarget) []string {
	ids := make([]string, 0, len(targets))
	for _, target := range targets {
		ids = append(ids, target.ID)
	}
	return ids
}

func projectDeletionTargetIDs(targets []projectDeletionTarget) []string {
	ids := make([]string, 0, len(targets))
	for _, target := range targets {
		ids = append(ids, target.ID)
	}
	return ids
}

func partnerCatalogGroups(types []string) []string {
	input := false
	output := false
	for _, partnerType := range types {
		if partnerType == "client" {
			input = true
		} else {
			output = true
		}
	}
	groups := []string{}
	if input {
		groups = append(groups, "input")
	}
	if output {
		groups = append(groups, "output")
	}
	return groups
}

func partnerImpactTotal(impact Record) int {
	total := 0
	for _, key := range []string{"project_count", "catalog_reference_count", "procurement_reference_count", "contract_reference_count", "finance_reference_count", "acceptance_reference_count"} {
		total += int(number(impact[key]))
	}
	return total
}

func partnerDeletionWarnings(groups []string, impact Record) []Record {
	warnings := []Record{}
	if len(groups) > 1 {
		warnings = append(warnings, Record{"code": "shared_catalog", "message": "Đối tác xuất hiện ở cả Đầu vào và Đầu ra; thao tác sẽ ẩn khỏi cả hai bảng"})
	}
	if count := partnerImpactTotal(impact); count > 0 {
		warnings = append(warnings, Record{"code": "business_references", "message": "Đang được tham chiếu trong dữ liệu nghiệp vụ; lịch sử và snapshot sẽ được giữ nguyên", "count": count})
	}
	return warnings
}

func projectDeletionWarnings(impact Record) []Record {
	warnings := []Record{}
	for _, item := range []struct {
		field   string
		code    string
		message string
	}{
		{"workflow_count", "workflow", "Có quy trình đang lưu trong công trình"},
		{"open_work_item_count", "open_work_items", "Có công việc chưa hoàn thành"},
		{"open_issue_count", "open_issues", "Có vướng mắc chưa đóng"},
		{"finance_record_count", "finance", "Có hồ sơ tài chính cần được giữ nguyên"},
		{"stock_movement_count", "stock", "Có lịch sử biến động kho cần được giữ nguyên"},
		{"acceptance_count", "acceptance", "Có hồ sơ nghiệm thu cần được giữ nguyên"},
	} {
		if count := int(number(impact[item.field])); count > 0 {
			warnings = append(warnings, Record{"code": item.code, "message": item.message, "count": count})
		}
	}
	return warnings
}

func (r *Repository) PartnerDeletionTargets(ctx context.Context, partnerIDs []string) ([]partnerDeletionTarget, error) {
	placeholders, args := staffDeletionPlaceholders(partnerIDs, 1)
	rows, err := r.db.QueryContext(ctx, `
		SELECT partner.id::text,partner.partner_code,partner.name,partner.partner_types::text,partner.active,partner.updated_at::text,
			(SELECT COUNT(*) FROM projects item WHERE item.partner_id=partner.id),
			((SELECT COUNT(*) FROM materials item WHERE item.default_supplier_partner_id=partner.id)),
			((SELECT COUNT(*) FROM procurement_request_items item WHERE item.supplier_partner_id=partner.id)
			 +(SELECT COUNT(*) FROM procurement_requests item WHERE item.supplier_partner_id=partner.id)
			 +(SELECT COUNT(*) FROM supplier_quotations item WHERE item.supplier_partner_id=partner.id)),
			(SELECT COUNT(*) FROM contracts item WHERE item.counterparty_partner_id=partner.id),
			((SELECT COUNT(*) FROM payables item WHERE item.vendor_partner_id=partner.id)
			 +(SELECT COUNT(*) FROM receivables item WHERE item.client_partner_id=partner.id)
			 +(SELECT COUNT(*) FROM input_invoices item WHERE item.vendor_partner_id=partner.id)
			 +(SELECT COUNT(*) FROM output_invoices item WHERE item.client_partner_id=partner.id)
			 +(SELECT COUNT(*) FROM journal_entry_lines item WHERE item.partner_id=partner.id)
			 +(SELECT COUNT(*) FROM project_material_costs item WHERE item.vendor_partner_id=partner.id)),
			(SELECT COUNT(*) FROM incoming_acceptance_records item WHERE item.supplier_partner_id=partner.id OR item.subcontractor_partner_id=partner.id)
		FROM partners partner
		WHERE partner.id IN (`+placeholders+`) AND partner.active
		ORDER BY partner.id
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	targets := []partnerDeletionTarget{}
	for rows.Next() {
		var target partnerDeletionTarget
		var partnerTypesJSON string
		var projectCount, catalogCount, procurementCount, contractCount, financeCount, acceptanceCount int
		if err := rows.Scan(
			&target.ID, &target.Code, &target.Name, &partnerTypesJSON, &target.Active, &target.UpdatedAt,
			&projectCount, &catalogCount, &procurementCount, &contractCount, &financeCount, &acceptanceCount,
		); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(partnerTypesJSON), &target.PartnerTypes); err != nil {
			return nil, err
		}
		target.Impact = Record{
			"project_count": projectCount, "catalog_reference_count": catalogCount,
			"procurement_reference_count": procurementCount, "contract_reference_count": contractCount,
			"finance_reference_count": financeCount, "acceptance_reference_count": acceptanceCount,
		}
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

func (r *Repository) ProjectDeletionTargets(ctx context.Context, projectIDs []string) ([]projectDeletionTarget, error) {
	placeholders, args := staffDeletionPlaceholders(projectIDs, 1)
	rows, err := r.db.QueryContext(ctx, `
		SELECT project.id::text,project.code,project.name,project.status,project.updated_at::text,
			(SELECT COUNT(*) FROM pipeline_runs item WHERE item.project_id=project.id),
			(SELECT COUNT(*) FROM work_items item WHERE item.project_id=project.id AND item.status NOT IN ('completed','done','cancelled')),
			(SELECT COUNT(*) FROM issues item WHERE item.project_id=project.id AND item.status NOT IN ('resolved','closed','cancelled','deleted')),
			((SELECT COUNT(*) FROM payables item WHERE item.project_id=project.id)
			 +(SELECT COUNT(*) FROM receivables item WHERE item.project_id=project.id)
			 +(SELECT COUNT(*) FROM payments item WHERE item.project_id=project.id)
			 +(SELECT COUNT(*) FROM input_invoices item WHERE item.project_id=project.id)
			 +(SELECT COUNT(*) FROM output_invoices item WHERE item.project_id=project.id)
			 +(SELECT COUNT(*) FROM project_cash_requests item WHERE item.project_id=project.id)),
			(SELECT COUNT(*) FROM stock_movements item WHERE item.project_id=project.id),
			((SELECT COUNT(*) FROM acceptance_records item WHERE item.project_id=project.id)
			 +(SELECT COUNT(*) FROM incoming_acceptance_records item WHERE item.project_id=project.id))
		FROM projects project
		WHERE project.id IN (`+placeholders+`) AND project.status<>'cancelled'
		ORDER BY project.id
	`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	targets := []projectDeletionTarget{}
	for rows.Next() {
		var target projectDeletionTarget
		var workflowCount, workItemCount, issueCount, financeCount, stockCount, acceptanceCount int
		if err := rows.Scan(
			&target.ID, &target.Code, &target.Name, &target.Status, &target.UpdatedAt,
			&workflowCount, &workItemCount, &issueCount, &financeCount, &stockCount, &acceptanceCount,
		); err != nil {
			return nil, err
		}
		target.Impact = Record{
			"workflow_count": workflowCount, "open_work_item_count": workItemCount, "open_issue_count": issueCount,
			"finance_record_count": financeCount, "stock_movement_count": stockCount, "acceptance_count": acceptanceCount,
		}
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

func (r *Repository) BulkDeletionResult(ctx context.Context, entityType, actorID, idempotencyKey, previewHash string) (Record, bool, error) {
	var storedActorID sql.NullString
	var storedEntityType, storedHash string
	var storedJSON []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT entity_type,actor_id::text,preview_hash,result_json
		FROM bulk_deletion_batches WHERE idempotency_key=$1
	`, idempotencyKey).Scan(&storedEntityType, &storedActorID, &storedHash, &storedJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if storedEntityType != entityType || !storedActorID.Valid || storedActorID.String != actorID || previewHash == "" || storedHash != previewHash {
		return nil, false, conflictError("Mã chống xóa trùng đã được dùng cho một danh sách khác", Record{"field": "idempotency_key"})
	}
	var result Record
	if err := json.Unmarshal(storedJSON, &result); err != nil {
		return nil, false, err
	}
	return result, true, nil
}

func (r *Repository) ExecutePartnerDeletion(ctx context.Context, actorID, actorName, idempotencyKey, previewHash string, targets []partnerDeletionTarget) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if result, found, err := bulkDeletionResultTx(ctx, tx, "partner", actorID, idempotencyKey, previewHash); err != nil {
		return nil, err
	} else if found {
		return result, tx.Commit()
	}
	ids := partnerDeletionTargetIDs(targets)
	placeholders, args := staffDeletionPlaceholders(ids, 1)
	rows, err := tx.QueryContext(ctx, `SELECT id::text,active,updated_at::text FROM partners WHERE id IN (`+placeholders+`) ORDER BY id FOR UPDATE`, args...)
	if err != nil {
		return nil, err
	}
	current := map[string]struct {
		Active    bool
		UpdatedAt string
	}{}
	for rows.Next() {
		var id, updatedAt string
		var active bool
		if err := rows.Scan(&id, &active, &updatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		current[id] = struct {
			Active    bool
			UpdatedAt string
		}{active, updatedAt}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(current) != len(targets) {
		return nil, conflictError("Danh sách đối tác đã thay đổi. Vui lòng kiểm tra lại", Record{"field": "partner_ids"})
	}
	for _, target := range targets {
		state := current[target.ID]
		if !state.Active || state.UpdatedAt != target.UpdatedAt {
			return nil, conflictError("Đối tác đã thay đổi sau bước kiểm tra. Vui lòng kiểm tra lại", Record{"partner_id": target.ID})
		}
	}
	items := make([]Record, 0, len(targets))
	for _, target := range targets {
		if _, err := tx.ExecContext(ctx, `UPDATE partners SET active=false,updated_at=now() WHERE id=$1 AND active`, target.ID); err != nil {
			return nil, err
		}
		if err := addAuditTx(ctx, tx, "", actorID, actorName, "partner_deactivated", "partner", target.ID, "Vô hiệu hóa đối tác và giữ nguyên liên kết nghiệp vụ", Record{
			"partner_code": target.Code, "partner_types": target.PartnerTypes, "catalog_groups": partnerCatalogGroups(target.PartnerTypes), "impact": target.Impact,
		}); err != nil {
			return nil, err
		}
		items = append(items, Record{"partner_id": target.ID, "partner_code": target.Code, "name": target.Name})
	}
	return insertBulkDeletionBatchTx(ctx, tx, "partner", actorID, actorName, idempotencyKey, previewHash, items)
}

func (r *Repository) ExecuteProjectDeletion(ctx context.Context, actorID, actorName, idempotencyKey, previewHash string, targets []projectDeletionTarget) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if result, found, err := bulkDeletionResultTx(ctx, tx, "project", actorID, idempotencyKey, previewHash); err != nil {
		return nil, err
	} else if found {
		return result, tx.Commit()
	}
	ids := projectDeletionTargetIDs(targets)
	placeholders, args := staffDeletionPlaceholders(ids, 1)
	rows, err := tx.QueryContext(ctx, `SELECT id::text,status,updated_at::text FROM projects WHERE id IN (`+placeholders+`) ORDER BY id FOR UPDATE`, args...)
	if err != nil {
		return nil, err
	}
	current := map[string]struct{ Status, UpdatedAt string }{}
	for rows.Next() {
		var id, status, updatedAt string
		if err := rows.Scan(&id, &status, &updatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		current[id] = struct{ Status, UpdatedAt string }{status, updatedAt}
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(current) != len(targets) {
		return nil, conflictError("Danh sách công trình đã thay đổi. Vui lòng kiểm tra lại", Record{"field": "project_ids"})
	}
	for _, target := range targets {
		state := current[target.ID]
		if state.Status == "cancelled" || state.UpdatedAt != target.UpdatedAt {
			return nil, conflictError("Công trình đã thay đổi sau bước kiểm tra. Vui lòng kiểm tra lại", Record{"project_id": target.ID})
		}
	}
	items := make([]Record, 0, len(targets))
	for _, target := range targets {
		if _, err := tx.ExecContext(ctx, `UPDATE projects SET status='cancelled',updated_at=now() WHERE id=$1 AND status<>'cancelled'`, target.ID); err != nil {
			return nil, err
		}
		if err := addAuditTx(ctx, tx, target.ID, actorID, actorName, "project_cancelled", "project", target.ID, "Hủy công trình và giữ nguyên lịch sử nghiệp vụ", Record{
			"project_code": target.Code, "previous_status": target.Status, "impact": target.Impact,
		}); err != nil {
			return nil, err
		}
		items = append(items, Record{"project_id": target.ID, "code": target.Code, "name": target.Name})
	}
	return insertBulkDeletionBatchTx(ctx, tx, "project", actorID, actorName, idempotencyKey, previewHash, items)
}

func bulkDeletionResultTx(ctx context.Context, tx *sql.Tx, entityType, actorID, idempotencyKey, previewHash string) (Record, bool, error) {
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, entityType+":"+idempotencyKey); err != nil {
		return nil, false, err
	}
	var storedActorID sql.NullString
	var storedEntityType, storedHash string
	var storedJSON []byte
	err := tx.QueryRowContext(ctx, `SELECT entity_type,actor_id::text,preview_hash,result_json FROM bulk_deletion_batches WHERE idempotency_key=$1`, idempotencyKey).
		Scan(&storedEntityType, &storedActorID, &storedHash, &storedJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if storedEntityType != entityType || !storedActorID.Valid || storedActorID.String != actorID || storedHash != previewHash {
		return nil, false, conflictError("Mã chống xóa trùng đã được dùng cho một danh sách khác", Record{"field": "idempotency_key"})
	}
	var result Record
	if err := json.Unmarshal(storedJSON, &result); err != nil {
		return nil, false, err
	}
	result["idempotent_replay"] = true
	return result, true, nil
}

func insertBulkDeletionBatchTx(ctx context.Context, tx *sql.Tx, entityType, actorID, actorName, idempotencyKey, previewHash string, items []Record) (Record, error) {
	batchID := uuid.NewString()
	storedResult := Record{"deletion_id": batchID, "deleted_count": len(items), "items": items}
	resultJSON, err := json.Marshal(storedResult)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO bulk_deletion_batches (id,entity_type,idempotency_key,actor_id,preview_hash,selected_count,deleted_count,result_json)
		VALUES ($1,$2,$3,$4,$5,$6,$6,$7::jsonb)
	`, batchID, entityType, idempotencyKey, actorID, previewHash, len(items), resultJSON); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, entityType+"_deletion_completed", "bulk_deletion_batch", batchID, "Hoàn tất thao tác xóa hàng loạt", Record{
		"entity_type": entityType, "selected_count": len(items), "deleted_count": len(items),
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return Record{"deletion_id": batchID, "deleted_count": len(items), "items": items, "idempotent_replay": false}, nil
}

func (s *Service) DeleteProject(ctx context.Context, id, actorID, actorName string) error {
	if err := s.repo.DeleteProject(ctx, id, actorID, actorName); err != nil {
		return err
	}
	s.publishProjectProgressChanged(cleanID(id))
	return nil
}

func (r *Repository) DeleteProject(ctx context.Context, id, actorID, actorName string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var code, name, status string
	if err := tx.QueryRowContext(ctx, `SELECT code,name,status FROM projects WHERE id=$1 FOR UPDATE`, id).Scan(&code, &name, &status); err != nil {
		return err
	}
	if status == "cancelled" {
		return conflictError("Công trình đã được hủy", Record{"project_id": id})
	}
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET status='cancelled',updated_at=now() WHERE id=$1`, id); err != nil {
		return err
	}
	if err := addAuditTx(ctx, tx, id, actorID, actorName, "project_cancelled", "project", id, "Hủy công trình và giữ nguyên lịch sử nghiệp vụ", Record{
		"project_code": code, "project_name": name, "previous_status": status, "source": "single_delete_api",
	}); err != nil {
		return err
	}
	return tx.Commit()
}
