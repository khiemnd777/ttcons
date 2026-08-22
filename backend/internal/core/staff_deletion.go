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

const staffDeletionMaximumRows = 100

type staffDeletionTarget struct {
	StaffID          string
	UserID           string
	FullName         string
	Email            string
	StaffStatus      string
	StaffUpdatedAt   string
	AccountStatus    string
	AccountDeleted   bool
	IsAdmin          bool
	LinkedStaffCount int
	WillDeleteUser   bool
	Blockers         []Record
	Warnings         []Record
	Impact           Record
}

type staffDeletionValidation struct {
	Preview Record
	Targets []staffDeletionTarget
	Hash    string
}

func (s *Service) PreviewStaffDeletion(ctx context.Context, actorID string, req StaffDeletionPreviewRequest) (Record, error) {
	validation, err := s.validateStaffDeletion(ctx, actorID, req)
	if err != nil {
		return nil, err
	}
	return validation.Preview, nil
}

func (s *Service) ExecuteStaffDeletion(ctx context.Context, actorID, actorName string, req StaffDeletionExecuteRequest) (Record, error) {
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	previewHash := strings.TrimSpace(req.PreviewHash)
	if _, err := uuid.Parse(idempotencyKey); err != nil {
		return nil, validationError("Mã chống xóa trùng không hợp lệ", Record{"field": "idempotency_key"})
	}
	if stored, found, err := s.repo.StaffDeletionResult(ctx, actorID, idempotencyKey, previewHash); err != nil {
		return nil, err
	} else if found {
		stored["idempotent_replay"] = true
		return stored, nil
	}

	validation, err := s.validateStaffDeletion(ctx, actorID, req.StaffDeletionPreviewRequest)
	if err != nil {
		return nil, err
	}
	if number(validation.Preview["blocked_count"]) > 0 {
		return nil, validationError("Danh sách xóa còn tài khoản bị chặn", validation.Preview)
	}
	if previewHash == "" || previewHash != validation.Hash {
		return nil, conflictError("Dữ liệu hoặc liên kết đã thay đổi sau bước kiểm tra. Vui lòng kiểm tra lại", Record{"field": "preview_hash"})
	}
	return s.repo.ExecuteStaffDeletion(ctx, actorID, actorName, idempotencyKey, previewHash, validation.Targets)
}

func (s *Service) validateStaffDeletion(ctx context.Context, actorID string, req StaffDeletionPreviewRequest) (staffDeletionValidation, error) {
	staffIDs, err := normalizedStaffDeletionIDs(req.StaffIDs)
	if err != nil {
		return staffDeletionValidation{}, err
	}
	targets, err := s.repo.StaffDeletionTargets(ctx, staffIDs)
	if err != nil {
		return staffDeletionValidation{}, err
	}
	if len(targets) != len(staffIDs) {
		found := make(map[string]bool, len(targets))
		for _, target := range targets {
			found[target.StaffID] = true
		}
		missing := make([]string, 0)
		for _, staffID := range staffIDs {
			if !found[staffID] {
				missing = append(missing, staffID)
			}
		}
		return staffDeletionValidation{}, conflictError("Một hoặc nhiều nhân sự không tồn tại hoặc đã được xóa", Record{"staff_ids": missing})
	}

	selectedByUser := map[string]int{}
	for _, target := range targets {
		if target.UserID != "" {
			selectedByUser[target.UserID]++
		}
	}
	deletingUserIDs := make([]string, 0)
	for index := range targets {
		if targets[index].UserID != "" && selectedByUser[targets[index].UserID] >= targets[index].LinkedStaffCount {
			targets[index].WillDeleteUser = true
			deletingUserIDs = append(deletingUserIDs, targets[index].UserID)
		}
	}
	deletingUserIDs = uniqueSortedStrings(deletingUserIDs)
	remainingActiveAdmins, err := s.repo.RemainingActiveAdminCount(ctx, deletingUserIDs)
	if err != nil {
		return staffDeletionValidation{}, err
	}

	impacts, err := s.repo.StaffDeletionImpacts(ctx, staffIDs)
	if err != nil {
		return staffDeletionValidation{}, err
	}
	previewRows := make([]Record, 0, len(targets))
	warningCount := 0
	blockedCount := 0
	impactTotals := Record{
		"project_manager_count":      0,
		"project_membership_count":   0,
		"work_item_count":            0,
		"other_responsibility_count": 0,
	}
	for index := range targets {
		target := &targets[index]
		target.Blockers = []Record{}
		if target.WillDeleteUser && target.UserID == actorID {
			target.Blockers = append(target.Blockers, Record{"code": "self_delete", "message": "Không thể xóa tài khoản đang đăng nhập"})
		}
		if target.WillDeleteUser && target.IsAdmin && target.AccountStatus == "active" && remainingActiveAdmins == 0 {
			target.Blockers = append(target.Blockers, Record{"code": "last_active_admin", "message": "Việc xóa sẽ khiến hệ thống không còn Admin hoạt động"})
		}
		target.Impact = impacts[target.StaffID]
		if target.Impact == nil {
			target.Impact = emptyStaffDeletionImpact()
		}
		target.Warnings = staffDeletionWarnings(target.Impact)
		warningCount += len(target.Warnings)
		if len(target.Blockers) > 0 {
			blockedCount++
		}
		for key := range impactTotals {
			impactTotals[key] = number(impactTotals[key]) + float64(len(recordSlice(target.Impact[impactFieldForTotal(key)])))
		}
		previewRows = append(previewRows, Record{
			"staff_id": target.StaffID, "user_id": target.UserID, "full_name": target.FullName, "email": target.Email,
			"staff_status": target.StaffStatus, "account_status": target.AccountStatus, "account_deleted": target.AccountDeleted,
			"will_delete_account": target.WillDeleteUser, "can_delete": len(target.Blockers) == 0,
			"blockers": target.Blockers, "warnings": target.Warnings, "impact": target.Impact,
		})
	}

	hashPayload := Record{"actor_id": actorID, "rows": previewRows, "impact_summary": impactTotals}
	payload, err := json.Marshal(hashPayload)
	if err != nil {
		return staffDeletionValidation{}, err
	}
	mac := hmac.New(sha256.New, []byte(s.jwtSecret))
	if _, err := mac.Write(payload); err != nil {
		return staffDeletionValidation{}, err
	}
	hash := hex.EncodeToString(mac.Sum(nil))
	preview := Record{
		"total_count": len(targets), "deletable_count": len(targets) - blockedCount, "blocked_count": blockedCount,
		"warning_count": warningCount, "impact_summary": impactTotals, "preview_hash": hash, "rows": previewRows,
	}
	return staffDeletionValidation{Preview: preview, Targets: targets, Hash: hash}, nil
}

func normalizedStaffDeletionIDs(rawIDs []string) ([]string, error) {
	if len(rawIDs) == 0 || len(rawIDs) > staffDeletionMaximumRows {
		return nil, validationError("Mỗi lần xóa phải chọn từ 1 đến 100 nhân sự", Record{"field": "staff_ids", "maximum": staffDeletionMaximumRows})
	}
	seen := map[string]bool{}
	ids := make([]string, 0, len(rawIDs))
	for _, rawID := range rawIDs {
		parsed, err := uuid.Parse(strings.TrimSpace(rawID))
		if err != nil {
			return nil, validationError("Mã nhân sự không hợp lệ", Record{"field": "staff_ids", "value": rawID})
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

func uniqueSortedStrings(values []string) []string {
	set := map[string]bool{}
	for _, value := range values {
		if value != "" {
			set[value] = true
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func emptyStaffDeletionImpact() Record {
	return Record{
		"project_manager":        []Record{},
		"project_memberships":    []Record{},
		"work_items":             []Record{},
		"other_responsibilities": []Record{},
	}
}

func staffDeletionWarnings(impact Record) []Record {
	warnings := []Record{}
	for _, item := range []struct {
		field   string
		code    string
		message string
	}{
		{"project_manager", "project_manager", "Đang phụ trách công trình; cần phân công lại sau khi xóa"},
		{"project_memberships", "project_membership", "Đang là thành viên công trình; membership sẽ ngừng hoạt động"},
		{"work_items", "work_item", "Đang được giao công việc chưa hoàn thành; cần phân công lại"},
		{"other_responsibilities", "other_responsibility", "Đang được tham chiếu ở trách nhiệm nghiệp vụ chưa kết thúc"},
	} {
		items := recordSlice(impact[item.field])
		if len(items) > 0 {
			warnings = append(warnings, Record{"code": item.code, "message": item.message, "count": len(items), "items": items})
		}
	}
	return warnings
}

func impactFieldForTotal(totalKey string) string {
	switch totalKey {
	case "project_manager_count":
		return "project_manager"
	case "project_membership_count":
		return "project_memberships"
	case "work_item_count":
		return "work_items"
	default:
		return "other_responsibilities"
	}
}

func recordSlice(value any) []Record {
	if values, ok := value.([]Record); ok {
		return values
	}
	return []Record{}
}

func (r *Repository) StaffDeletionTargets(ctx context.Context, staffIDs []string) ([]staffDeletionTarget, error) {
	query, args := staffDeletionPlaceholders(staffIDs, 1)
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT person.id::text,COALESCE(person.user_id::text,''),person.full_name,person.email,person.status,
			person.updated_at::text,COALESCE(account.status,''),account.deleted_at IS NOT NULL,
			EXISTS (
				SELECT 1 FROM user_roles assignment
				JOIN roles role ON role.id=assignment.role_id
				WHERE assignment.user_id=person.user_id AND assignment.project_id IS NULL AND role.name='admin'
			),
			CASE WHEN person.user_id IS NULL THEN 0 ELSE (
				SELECT COUNT(*) FROM staff linked WHERE linked.user_id=person.user_id AND linked.status<>'deleted'
			) END
		FROM staff person
		LEFT JOIN users account ON account.id=person.user_id
		WHERE person.id IN (%s) AND person.status<>'deleted'
		ORDER BY person.id
	`, query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	targets := []staffDeletionTarget{}
	for rows.Next() {
		var target staffDeletionTarget
		if err := rows.Scan(
			&target.StaffID, &target.UserID, &target.FullName, &target.Email, &target.StaffStatus,
			&target.StaffUpdatedAt, &target.AccountStatus, &target.AccountDeleted, &target.IsAdmin, &target.LinkedStaffCount,
		); err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, rows.Err()
}

func (r *Repository) RemainingActiveAdminCount(ctx context.Context, excludedUserIDs []string) (int, error) {
	args := []any{}
	exclusion := ""
	if len(excludedUserIDs) > 0 {
		placeholders, values := staffDeletionPlaceholders(excludedUserIDs, 1)
		args = values
		exclusion = "AND account.id NOT IN (" + placeholders + ")"
	}
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT account.id)
		FROM users account
		JOIN user_roles assignment ON assignment.user_id=account.id AND assignment.project_id IS NULL
		JOIN roles role ON role.id=assignment.role_id AND role.name='admin'
		WHERE account.status='active' AND account.deleted_at IS NULL `+exclusion, args...).Scan(&count)
	return count, err
}

func (r *Repository) StaffDeletionImpacts(ctx context.Context, staffIDs []string) (map[string]Record, error) {
	result := make(map[string]Record, len(staffIDs))
	for _, staffID := range staffIDs {
		result[staffID] = emptyStaffDeletionImpact()
	}
	placeholders, args := staffDeletionPlaceholders(staffIDs, 1)
	queries := []struct {
		field string
		sql   string
	}{
		{"project_manager", `
			SELECT project.pm_staff_id::text AS staff_id,project.id::text AS reference_id,
				project.code,project.name AS label,project.status,'project' AS reference_type
			FROM projects project
			WHERE project.pm_staff_id IN (` + placeholders + `) AND project.status<>'cancelled'
			ORDER BY project.code,project.id`},
		{"project_memberships", `
			SELECT member.staff_id::text AS staff_id,member.id::text AS reference_id,
				project.code,project.name AS label,project.status,'project_membership' AS reference_type
			FROM project_members member
			JOIN projects project ON project.id=member.project_id AND project.status<>'cancelled'
			WHERE member.staff_id IN (` + placeholders + `) AND member.active
			ORDER BY project.code,member.id`},
		{"work_items", `
			SELECT DISTINCT source.staff_id,source.reference_id,source.code,source.label,source.status,source.reference_type
			FROM (
				SELECT item.assignee_staff_id::text AS staff_id,item.id::text AS reference_id,
					project.code,item.title AS label,item.status,'assignee' AS reference_type
				FROM work_items item
				JOIN projects project ON project.id=item.project_id
				WHERE item.assignee_staff_id IN (` + placeholders + `)
				  AND item.status NOT IN ('completed','done','cancelled')
				UNION ALL
				SELECT assignment.staff_id::text,item.id::text,project.code,item.title,item.status,assignment.assignment_type
				FROM work_item_assignments assignment
				JOIN work_items item ON item.id=assignment.work_item_id
				JOIN projects project ON project.id=item.project_id
				WHERE assignment.staff_id IN (` + placeholders + `)
				  AND item.status NOT IN ('completed','done','cancelled')
			) source
			ORDER BY source.code,source.label,source.reference_id`},
		{"other_responsibilities", `
			SELECT source.staff_id,source.reference_id,source.code,source.label,source.status,source.reference_type
			FROM (
				SELECT issue.assigned_to::text AS staff_id,issue.id::text AS reference_id,project.code,
					issue.title AS label,issue.status,'issue' AS reference_type
				FROM issues issue JOIN projects project ON project.id=issue.project_id
				WHERE issue.assigned_to IN (` + placeholders + `) AND issue.status NOT IN ('resolved','closed','cancelled','deleted')
				UNION ALL
				SELECT rework.assigned_to::text,rework.id::text,project.code,rework.title,rework.status,'rework'
				FROM rework_requests rework JOIN projects project ON project.id=rework.project_id
				WHERE rework.assigned_to IN (` + placeholders + `) AND rework.status NOT IN ('done','completed','closed','cancelled','deleted')
				UNION ALL
				SELECT acceptance.inspected_by::text,acceptance.id::text,project.code,acceptance.title,acceptance.status,'acceptance'
				FROM acceptance_records acceptance JOIN projects project ON project.id=acceptance.project_id
				WHERE acceptance.inspected_by IN (` + placeholders + `) AND acceptance.status NOT IN ('approved','completed','closed','cancelled','deleted')
				UNION ALL
				SELECT person.id::text,action.id::text,project.code,action.title,action.status,'construction_action'
				FROM construction_action_items action
				JOIN projects project ON project.id=action.project_id
				JOIN staff person ON person.user_id=action.assigned_user_id AND person.id IN (` + placeholders + `)
				WHERE action.status IN ('open','waiting_confirmation')
				UNION ALL
				SELECT person.id::text,plan.id::text,project.code,plan.title,plan.status,'recurring_plan'
				FROM construction_recurring_plans plan
				JOIN projects project ON project.id=plan.project_id
				JOIN staff person ON person.user_id=plan.assigned_user_id AND person.id IN (` + placeholders + `)
				WHERE plan.status='active'
				UNION ALL
				SELECT person.id::text,occurrence.id::text,project.code,plan.title,occurrence.status,'recurring_occurrence'
				FROM construction_recurring_occurrences occurrence
				JOIN construction_recurring_plans plan ON plan.id=occurrence.plan_id
				JOIN projects project ON project.id=occurrence.project_id
				JOIN staff person ON person.user_id=occurrence.assigned_user_id AND person.id IN (` + placeholders + `)
				WHERE occurrence.status IN ('pending','in_progress','overdue')
				UNION ALL
				SELECT person.id::text,item.id::text,project.code,item.title,item.status,'construction_handoff_item'
				FROM construction_handoff_items item
				JOIN projects project ON project.id=item.project_id
				JOIN staff person ON person.user_id=item.assigned_user_id AND person.id IN (` + placeholders + `)
				WHERE item.status='activated'
			) source
			ORDER BY source.code,source.label,source.reference_id`},
	}
	for _, spec := range queries {
		rows, err := r.Query(ctx, spec.sql, args...)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			staffID := cleanID(row["staff_id"])
			delete(row, "staff_id")
			impact := result[staffID]
			items := recordSlice(impact[spec.field])
			impact[spec.field] = append(items, row)
			result[staffID] = impact
		}
	}
	return result, nil
}

func staffDeletionPlaceholders(ids []string, start int) (string, []any) {
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for index, id := range ids {
		placeholders[index] = fmt.Sprintf("$%d", start+index)
		args[index] = id
	}
	return strings.Join(placeholders, ","), args
}

func (r *Repository) StaffDeletionResult(ctx context.Context, actorID, idempotencyKey, previewHash string) (Record, bool, error) {
	var storedActorID sql.NullString
	var storedHash string
	var storedJSON []byte
	err := r.db.QueryRowContext(ctx, `
		SELECT actor_id::text,preview_hash,result_json
		FROM staff_deletion_batches
		WHERE idempotency_key=$1
	`, idempotencyKey).Scan(&storedActorID, &storedHash, &storedJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !storedActorID.Valid || storedActorID.String != actorID || previewHash == "" || storedHash != previewHash {
		return nil, false, conflictError("Mã chống xóa trùng đã được dùng cho một danh sách khác", Record{"field": "idempotency_key"})
	}
	var result Record
	if err := json.Unmarshal(storedJSON, &result); err != nil {
		return nil, false, err
	}
	return result, true, nil
}

func (r *Repository) ExecuteStaffDeletion(
	ctx context.Context,
	actorID string,
	actorName string,
	idempotencyKey string,
	previewHash string,
	targets []staffDeletionTarget,
) (result Record, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, idempotencyKey); err != nil {
		return nil, err
	}
	var storedHash string
	var storedActorID sql.NullString
	var storedJSON []byte
	if err := tx.QueryRowContext(ctx, `
		SELECT actor_id::text,preview_hash,result_json
		FROM staff_deletion_batches WHERE idempotency_key=$1
	`, idempotencyKey).Scan(&storedActorID, &storedHash, &storedJSON); err == nil {
		if !storedActorID.Valid || storedActorID.String != actorID || storedHash != previewHash {
			return nil, conflictError("Mã chống xóa trùng đã được dùng cho một danh sách khác", Record{"field": "idempotency_key"})
		}
		if err := json.Unmarshal(storedJSON, &result); err != nil {
			return nil, err
		}
		result["idempotent_replay"] = true
		return result, tx.Commit()
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	staffIDs := make([]string, 0, len(targets))
	for _, target := range targets {
		staffIDs = append(staffIDs, target.StaffID)
	}
	staffIDs = uniqueSortedStrings(staffIDs)
	placeholders, args := staffDeletionPlaceholders(staffIDs, 1)
	lockedRows, err := tx.QueryContext(ctx, `
		SELECT id::text,COALESCE(user_id::text,''),full_name,email,status
		FROM staff
		WHERE id IN (`+placeholders+`)
		ORDER BY id
		FOR UPDATE
	`, args...)
	if err != nil {
		return nil, err
	}
	locked := []staffDeletionTarget{}
	for lockedRows.Next() {
		var target staffDeletionTarget
		if err := lockedRows.Scan(&target.StaffID, &target.UserID, &target.FullName, &target.Email, &target.StaffStatus); err != nil {
			lockedRows.Close()
			return nil, err
		}
		locked = append(locked, target)
	}
	if err := lockedRows.Close(); err != nil {
		return nil, err
	}
	if len(locked) != len(staffIDs) {
		return nil, conflictError("Danh sách nhân sự đã thay đổi. Vui lòng kiểm tra lại", Record{"field": "staff_ids"})
	}
	for _, target := range locked {
		if target.StaffStatus == "deleted" {
			return nil, conflictError("Nhân sự đã được xóa trước đó", Record{"staff_id": target.StaffID})
		}
	}

	selectedByUser := map[string]int{}
	for _, target := range locked {
		if target.UserID != "" {
			selectedByUser[target.UserID]++
		}
	}
	deletingUserIDs := []string{}
	for userID, selectedCount := range selectedByUser {
		var linkedCount int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM staff WHERE user_id=$1 AND status<>'deleted'`, userID).Scan(&linkedCount); err != nil {
			return nil, err
		}
		if selectedCount >= linkedCount {
			deletingUserIDs = append(deletingUserIDs, userID)
		}
	}
	deletingUserIDs = uniqueSortedStrings(deletingUserIDs)
	for _, userID := range deletingUserIDs {
		if userID == actorID {
			return nil, validationError("Không thể xóa tài khoản đang đăng nhập", Record{"code": "self_delete"})
		}
	}
	if len(deletingUserIDs) > 0 {
		userPlaceholders, userArgs := staffDeletionPlaceholders(deletingUserIDs, 1)
		userRows, err := tx.QueryContext(ctx, `SELECT id FROM users WHERE id IN (`+userPlaceholders+`) ORDER BY id FOR UPDATE`, userArgs...)
		if err != nil {
			return nil, err
		}
		lockedUserCount := 0
		for userRows.Next() {
			var userID string
			if err := userRows.Scan(&userID); err != nil {
				userRows.Close()
				return nil, err
			}
			lockedUserCount++
		}
		if err := userRows.Close(); err != nil {
			return nil, err
		}
		if lockedUserCount != len(deletingUserIDs) {
			return nil, conflictError("Tài khoản liên kết đã thay đổi. Vui lòng kiểm tra lại", Record{"field": "staff_ids"})
		}
		var deletingActiveAdmins int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(DISTINCT account.id)
			FROM users account
			JOIN user_roles assignment ON assignment.user_id=account.id AND assignment.project_id IS NULL
			JOIN roles role ON role.id=assignment.role_id AND role.name='admin'
			WHERE account.id IN (`+userPlaceholders+`) AND account.status='active' AND account.deleted_at IS NULL
		`, userArgs...).Scan(&deletingActiveAdmins); err != nil {
			return nil, err
		}
		if deletingActiveAdmins > 0 {
			var remainingActiveAdmins int
			if err := tx.QueryRowContext(ctx, `
				SELECT COUNT(DISTINCT account.id)
				FROM users account
				JOIN user_roles assignment ON assignment.user_id=account.id AND assignment.project_id IS NULL
				JOIN roles role ON role.id=assignment.role_id AND role.name='admin'
				WHERE account.status='active' AND account.deleted_at IS NULL
				  AND account.id NOT IN (`+userPlaceholders+`)
			`, userArgs...).Scan(&remainingActiveAdmins); err != nil {
				return nil, err
			}
			if remainingActiveAdmins == 0 {
				return nil, validationError("Không thể xóa Admin hoạt động cuối cùng", Record{"code": "last_active_admin"})
			}
		}
	}

	targetByStaffID := map[string]staffDeletionTarget{}
	for _, target := range targets {
		targetByStaffID[target.StaffID] = target
	}
	deletingUsers := map[string]bool{}
	for _, userID := range deletingUserIDs {
		deletingUsers[userID] = true
	}
	deletedItems := make([]Record, 0, len(locked))
	totalMembershipsDeactivated := int64(0)
	totalSessionsRevoked := int64(0)
	totalTokensInvalidated := int64(0)
	totalRolesRemoved := int64(0)
	for _, current := range locked {
		previewTarget := targetByStaffID[current.StaffID]
		membershipResult, err := tx.ExecContext(ctx, `
			UPDATE project_members SET active=false,updated_at=now()
			WHERE staff_id=$1 AND active
		`, current.StaffID)
		if err != nil {
			return nil, err
		}
		membershipsDeactivated, _ := membershipResult.RowsAffected()
		totalMembershipsDeactivated += membershipsDeactivated
		if _, err := tx.ExecContext(ctx, `
			UPDATE staff
			SET status='deleted',deleted_at=now(),deleted_by=$2,updated_at=now()
			WHERE id=$1 AND status<>'deleted'
		`, current.StaffID, actorID); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE projects SET updated_at=now() WHERE pm_staff_id=$1`, current.StaffID); err != nil {
			return nil, err
		}

		sessionsRevoked := int64(0)
		tokensInvalidated := int64(0)
		rolesRemoved := int64(0)
		if current.UserID != "" && deletingUsers[current.UserID] {
			sessionResult, err := tx.ExecContext(ctx, `
				UPDATE auth_sessions SET revoked_at=COALESCE(revoked_at,now())
				WHERE user_id=$1 AND revoked_at IS NULL
			`, current.UserID)
			if err != nil {
				return nil, err
			}
			sessionsRevoked, _ = sessionResult.RowsAffected()
			tokenResult, err := tx.ExecContext(ctx, `
				UPDATE auth_email_tokens SET invalidated_at=now()
				WHERE user_id=$1 AND consumed_at IS NULL AND invalidated_at IS NULL
			`, current.UserID)
			if err != nil {
				return nil, err
			}
			tokensInvalidated, _ = tokenResult.RowsAffected()
			roleResult, err := tx.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, current.UserID)
			if err != nil {
				return nil, err
			}
			rolesRemoved, _ = roleResult.RowsAffected()
			if _, err := tx.ExecContext(ctx, `
				UPDATE users
				SET status='inactive',deleted_at=now(),deleted_by=$2,
					recovery_email=NULL,recovery_email_verified_at=NULL,must_change_password=false,updated_at=now()
				WHERE id=$1 AND deleted_at IS NULL
			`, current.UserID, actorID); err != nil {
				return nil, err
			}
			delete(deletingUsers, current.UserID)
		}
		totalSessionsRevoked += sessionsRevoked
		totalTokensInvalidated += tokensInvalidated
		totalRolesRemoved += rolesRemoved
		metadata := Record{
			"target_user_id": current.UserID, "account_deleted": current.UserID != "" && previewTarget.WillDeleteUser,
			"memberships_deactivated": membershipsDeactivated, "sessions_revoked": sessionsRevoked,
			"tokens_invalidated": tokensInvalidated, "roles_removed": rolesRemoved,
			"impact_counts": Record{
				"project_manager":        len(recordSlice(previewTarget.Impact["project_manager"])),
				"project_memberships":    len(recordSlice(previewTarget.Impact["project_memberships"])),
				"work_items":             len(recordSlice(previewTarget.Impact["work_items"])),
				"other_responsibilities": len(recordSlice(previewTarget.Impact["other_responsibilities"])),
			},
		}
		if err := addAuditTx(ctx, tx, "", actorID, actorName, "staff_soft_deleted", "staff", current.StaffID, "Soft-delete tài khoản nhân sự và giữ nguyên liên kết nghiệp vụ", metadata); err != nil {
			return nil, err
		}
		deletedItems = append(deletedItems, Record{
			"staff_id": current.StaffID, "user_id": current.UserID, "full_name": current.FullName,
			"account_deleted": current.UserID != "" && previewTarget.WillDeleteUser,
		})
	}

	batchID := uuid.NewString()
	result = Record{
		"deletion_id": batchID, "deleted_count": len(locked), "items": deletedItems,
		"memberships_deactivated": totalMembershipsDeactivated, "sessions_revoked": totalSessionsRevoked,
		"tokens_invalidated": totalTokensInvalidated, "roles_removed": totalRolesRemoved,
		"idempotent_replay": false,
	}
	storedResult := Record{
		"deletion_id": batchID, "deleted_count": len(locked), "items": deletedItems,
		"memberships_deactivated": totalMembershipsDeactivated, "sessions_revoked": totalSessionsRevoked,
		"tokens_invalidated": totalTokensInvalidated, "roles_removed": totalRolesRemoved,
	}
	resultJSON, err := json.Marshal(storedResult)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO staff_deletion_batches (id,idempotency_key,actor_id,preview_hash,staff_count,deleted_count,result_json)
		VALUES ($1,$2,$3,$4,$5,$5,$6::jsonb)
	`, batchID, idempotencyKey, actorID, previewHash, len(locked), resultJSON); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "staff_deletion_completed", "staff_deletion_batch", batchID, "Hoàn tất soft-delete tài khoản nhân sự", Record{
		"staff_count": len(locked), "memberships_deactivated": totalMembershipsDeactivated,
		"sessions_revoked": totalSessionsRevoked, "tokens_invalidated": totalTokensInvalidated, "roles_removed": totalRolesRemoved,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
