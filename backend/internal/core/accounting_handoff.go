package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

func (r *Repository) AccountingHandoffCandidates(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		WITH procurement_sources AS (
			SELECT request.id,request.project_id,'payable'::text AS direction,
			       'procurement_request'::text AS source_type,request.request_code AS source_code,
			       request.title AS source_title,
			       COALESCE(partner.name,request.supplier_name,'') AS counterparty_name,
			       request.total_estimated_amount AS reference_amount,'VND'::text AS currency,
			       request.status AS source_status,request.order_reference,
			       (request.status IN ('ordered','partially_received','received')) AS status_ready,
			       (NULLIF(btrim(COALESCE(partner.name,request.supplier_name,'')),'') IS NOT NULL) AS counterparty_ready,
			       (NULLIF(btrim(COALESCE(request.order_reference,'')),'') IS NOT NULL) AS reference_ready,
			       (SELECT COUNT(*) FROM incoming_acceptance_records acceptance
			        WHERE acceptance.procurement_request_id=request.id AND acceptance.status='submitted'
			          AND acceptance.result IN ('pass','conditional_pass')) AS acceptance_count,
			       (SELECT COUNT(*) FROM stock_documents document
			        WHERE document.project_id=request.project_id AND document.status='posted'
			          AND (
			            (document.reference_type='procurement_request' AND document.reference_id=request.id)
			            OR document.reference_id IN (
			              SELECT acceptance.id FROM incoming_acceptance_records acceptance
			              WHERE acceptance.procurement_request_id=request.id
			            )
			          )) AS stock_document_count
			FROM procurement_requests request
			LEFT JOIN partners partner ON partner.id=request.supplier_partner_id
			WHERE request.project_id=$1
			  AND request.status IN ('approved','ordered','partially_received','received')
		), milestone_sources AS (
			SELECT milestone.id,contract.project_id,'receivable'::text AS direction,
			       'contract_milestone'::text AS source_type,milestone.milestone_code AS source_code,
			       milestone.milestone_name AS source_title,contract.counterparty_name,
			       milestone.net_amount AS reference_amount,contract.currency,
			       milestone.status AS source_status,NULL::text AS order_reference,
			       (milestone.status IN ('active','eligible')) AS status_ready,
			       (NULLIF(btrim(contract.counterparty_name),'') IS NOT NULL) AS counterparty_ready,
			       ((NOT milestone.acceptance_required) OR EXISTS (
			          SELECT 1 FROM acceptance_records acceptance
			          WHERE acceptance.id=milestone.acceptance_id AND acceptance.status='completed'
			            AND acceptance.result IN ('pass','conditional_pass')
			       )) AS reference_ready,
			       CASE WHEN milestone.acceptance_id IS NULL THEN 0 ELSE 1 END AS acceptance_count,
			       0::bigint AS stock_document_count
			FROM contract_milestones milestone
			JOIN contracts contract ON contract.id=milestone.contract_id
			WHERE contract.project_id=$1 AND contract.direction='receivable'
			  AND contract.contract_type='main_client' AND contract.status IN ('active','completed')
			  AND milestone.status IN ('active','eligible')
		), sources AS (
			SELECT * FROM procurement_sources
			UNION ALL
			SELECT * FROM milestone_sources
		)
		SELECT source.id AS source_id,source.project_id,source.direction,source.source_type,
		       source.source_code,source.source_title,source.counterparty_name,
		       source.reference_amount,source.currency,source.source_status,
		       source.acceptance_count,source.stock_document_count,
		       CASE source.source_type
		         WHEN 'procurement_request' THEN '/projects/' || source.project_id || '/procurement'
		         ELSE '/projects/' || source.project_id || '/contracts'
		       END AS source_route,
		       CASE WHEN source.status_ready AND source.counterparty_ready AND source.reference_ready
		         THEN 'ready' ELSE 'needs_attention' END AS readiness,
		       jsonb_build_array(
		         jsonb_build_object(
		           'key','source_status','label',CASE source.source_type WHEN 'procurement_request' THEN 'Đã đặt hàng' ELSE 'Mốc hợp đồng đã kích hoạt' END,
		           'met',source.status_ready,
		           'guidance',CASE source.source_type WHEN 'procurement_request' THEN 'Chuyển yêu cầu mua sang trạng thái đã đặt hàng.' ELSE 'Kích hoạt mốc thu trong hợp đồng.' END
		         ),
		         jsonb_build_object(
		           'key','counterparty','label','Đã xác định đối tác',
		           'met',source.counterparty_ready,
		           'guidance','Bổ sung đối tác tại hồ sơ nguồn.'
		         ),
		         jsonb_build_object(
		           'key','source_reference','label',CASE source.source_type WHEN 'procurement_request' THEN 'Có số đơn đặt hàng' ELSE 'Đủ điều kiện nghiệm thu' END,
		           'met',source.reference_ready,
		           'guidance',CASE source.source_type WHEN 'procurement_request' THEN 'Bổ sung số đơn đặt hàng tại Mua sắm.' ELSE 'Hoàn tất nghiệm thu được yêu cầu cho mốc hợp đồng.' END
		         )
		       ) AS requirements,
		       handoff.id AS handoff_id,handoff.status AS handoff_status,handoff.note,
		       handoff.external_reference,handoff.return_reason,
		       handoff.handed_off_at,handoff.acknowledged_at,handoff.returned_at,
		       handoff_user.full_name AS handed_off_by_name,
		       acknowledge_user.full_name AS acknowledged_by_name,
		       return_user.full_name AS returned_by_name
		FROM sources source
		LEFT JOIN accounting_handoffs handoff
		  ON handoff.project_id=source.project_id AND handoff.source_type=source.source_type AND handoff.source_id=source.id
		LEFT JOIN users handoff_user ON handoff_user.id=handoff.handed_off_by
		LEFT JOIN users acknowledge_user ON acknowledge_user.id=handoff.acknowledged_by
		LEFT JOIN users return_user ON return_user.id=handoff.returned_by
		ORDER BY
		  CASE COALESCE(handoff.status,CASE WHEN source.status_ready AND source.counterparty_ready AND source.reference_ready THEN 'ready' ELSE 'needs_attention' END)
		    WHEN 'returned' THEN 1 WHEN 'needs_attention' THEN 2 WHEN 'ready' THEN 3
		    WHEN 'handed_off' THEN 4 WHEN 'acknowledged' THEN 5 ELSE 6 END,
		  source.source_type,source.source_code,source.id
	`, projectID)
}

func (s *Service) AccountingHandoffWorkspace(ctx context.Context, projectID string) (Record, error) {
	items, err := s.repo.AccountingHandoffCandidates(ctx, projectID)
	if err != nil {
		return nil, err
	}
	summary := Record{
		"needs_attention_count": 0,
		"ready_count":           0,
		"handed_off_count":      0,
		"acknowledged_count":    0,
		"returned_count":        0,
	}
	for _, item := range items {
		status := cleanOptionalText(item["handoff_status"])
		if status == "" || status == "cancelled" {
			status = fmt.Sprint(item["readiness"])
		}
		item["status"] = status
		key := status + "_count"
		if current, ok := summary[key]; ok {
			summary[key] = int(number(current)) + 1
		}
	}
	return Record{
		"mode":    "handoff",
		"summary": summary,
		"items":   items,
	}, nil
}

func (r *Repository) MyAccountingWork(ctx context.Context, actorID string, filter MyAccountingWorkFilter, offset int) (Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, actorID, "project", 1)
	if err != nil {
		return nil, err
	}
	baseArgs := append([]any{}, scopeArgs...)
	baseConditions := []string{"project.status <> 'cancelled'", "handoff.status <> 'cancelled'", scopeSQL}
	if filter.ProjectID != "" {
		baseArgs = append(baseArgs, filter.ProjectID)
		baseConditions = append(baseConditions, fmt.Sprintf("handoff.project_id=$%d::uuid", len(baseArgs)))
	}
	if filter.Search != "" {
		baseArgs = append(baseArgs, "%"+filter.Search+"%")
		placeholder := len(baseArgs)
		baseConditions = append(baseConditions, fmt.Sprintf(`concat_ws(' ',
			project.code, project.name,
			COALESCE(handoff.source_snapshot_json->>'source_code',''),
			COALESCE(handoff.source_snapshot_json->>'source_title',''),
			COALESCE(handoff.source_snapshot_json->>'counterparty_name',''),
			COALESCE(handoff.external_reference,'')
		) ILIKE $%d`, placeholder))
	}
	baseWhere := strings.Join(baseConditions, " AND ")
	summary, err := r.QueryOne(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*) FILTER (WHERE handoff.status='handed_off') AS handed_off_count,
			COUNT(*) FILTER (WHERE handoff.status='returned') AS returned_count,
			COUNT(*) FILTER (WHERE handoff.status='acknowledged') AS acknowledged_count,
			COUNT(*) AS total
		FROM accounting_handoffs handoff
		JOIN projects project ON project.id=handoff.project_id
		WHERE %s
	`, baseWhere), baseArgs...)
	if err != nil {
		return nil, err
	}
	itemArgs := append([]any{}, baseArgs...)
	itemConditions := append([]string{}, baseConditions...)
	if filter.Status != "all" {
		itemArgs = append(itemArgs, filter.Status)
		itemConditions = append(itemConditions, fmt.Sprintf("handoff.status=$%d", len(itemArgs)))
	}
	itemWhere := strings.Join(itemConditions, " AND ")
	totalRow, err := r.QueryOne(ctx, fmt.Sprintf(`
		SELECT COUNT(*) AS total
		FROM accounting_handoffs handoff
		JOIN projects project ON project.id=handoff.project_id
		WHERE %s
	`, itemWhere), itemArgs...)
	if err != nil {
		return nil, err
	}
	total := int(number(totalRow["total"]))
	pageArgs := append([]any{}, itemArgs...)
	pageArgs = append(pageArgs, filter.Limit, offset)
	items, err := r.Query(ctx, accountingWorkSelectSQL()+fmt.Sprintf(`
		WHERE %s
		ORDER BY
			CASE handoff.status WHEN 'handed_off' THEN 1 WHEN 'returned' THEN 2 WHEN 'acknowledged' THEN 3 ELSE 4 END,
			handoff.updated_at DESC,
			handoff.id DESC
		LIMIT $%d OFFSET $%d
	`, itemWhere, len(pageArgs)-1, len(pageArgs)), pageArgs...)
	if err != nil {
		return nil, err
	}
	nextOffset := offset + len(items)
	hasMore := nextOffset < total
	nextCursor := ""
	if hasMore {
		nextCursor = encodeMyWorkItemsCursor(nextOffset)
	}
	return Record{
		"items":       items,
		"summary":     summary,
		"total":       total,
		"has_more":    hasMore,
		"next_cursor": nextCursor,
	}, nil
}

func (s *Service) CreateAccountingHandoff(ctx context.Context, projectID string, req AccountingHandoffRequest, actorID, actorName string) (Record, error) {
	req.SourceType = strings.TrimSpace(req.SourceType)
	req.SourceID = strings.TrimSpace(req.SourceID)
	req.Note = strings.TrimSpace(req.Note)
	req.ExternalReference = strings.TrimSpace(req.ExternalReference)
	if req.SourceType == "" || req.SourceID == "" {
		return nil, validationError("Cần chọn hồ sơ nguồn để bàn giao", Record{"fields": []string{"source_type", "source_id"}})
	}
	if req.SourceType != "procurement_request" && req.SourceType != "contract_milestone" {
		return nil, validationError("Loại hồ sơ nguồn không hợp lệ", Record{"source_type": req.SourceType})
	}
	return s.repo.CreateAccountingHandoff(ctx, projectID, req, actorID, actorName)
}

func (s *Service) AccountingHandoffAction(ctx context.Context, id, action string, req AccountingHandoffActionRequest, actorID, actorName string) (Record, error) {
	req.Note = strings.TrimSpace(req.Note)
	req.ReturnReason = strings.TrimSpace(req.ReturnReason)
	req.ExternalReference = strings.TrimSpace(req.ExternalReference)
	if action == "return" && req.ReturnReason == "" {
		return nil, validationError("Cần ghi rõ nội dung hồ sơ cần bổ sung", Record{"field": "return_reason"})
	}
	return s.repo.AccountingHandoffAction(ctx, id, action, req, actorID, actorName)
}

func (r *Repository) CreateAccountingHandoff(ctx context.Context, projectID string, req AccountingHandoffRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	source, err := accountingHandoffSourceTx(ctx, tx, projectID, req.SourceType, req.SourceID)
	if err != nil {
		return nil, err
	}
	if !boolValue(source["ready"]) {
		return nil, validationError("Hồ sơ nguồn chưa đủ điều kiện bàn giao", Record{"source_type": req.SourceType, "source_id": req.SourceID})
	}
	existing, err := queryTx(ctx, tx, `
		SELECT * FROM accounting_handoffs
		WHERE project_id=$1 AND source_type=$2 AND source_id=$3
		FOR UPDATE
	`, projectID, req.SourceType, req.SourceID)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return nil, validationError("Hồ sơ nguồn đã có lịch sử bàn giao", Record{"status": existing[0]["status"], "handoff_id": existing[0]["id"]})
	}
	snapshot, err := json.Marshal(source["snapshot"])
	if err != nil {
		return nil, err
	}
	rows, err := queryTx(ctx, tx, `
		INSERT INTO accounting_handoffs (
			project_id,direction,source_type,source_id,source_snapshot_json,status,note,external_reference,
			handed_off_by,handed_off_at
		) VALUES ($1,$2,$3,$4,$5::jsonb,'handed_off',NULLIF($6,''),NULLIF($7,''),$8,now())
		RETURNING *
	`, projectID, source["direction"], req.SourceType, req.SourceID, string(snapshot), req.Note, req.ExternalReference, actorID)
	if err != nil {
		return nil, err
	}
	row := rows[0]
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "accounting_handoff_created", "accounting_handoff", cleanID(row["id"]), "Bàn giao hồ sơ nghiệp vụ cho Kế toán", Record{"source_type": req.SourceType, "source_id": req.SourceID}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, projectID, req.WorkItemContext, projectFinanceWorkItemPolicy, "accounting_handoff", cleanID(row["id"]), actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return attachWorkItemTransition(row, transition), nil
}

func (r *Repository) AccountingHandoffAction(ctx context.Context, id, action string, req AccountingHandoffActionRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT * FROM accounting_handoffs WHERE id=$1 FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	var updated []Record
	switch action {
	case "acknowledge":
		if fromStatus != "handed_off" {
			return nil, validationError("Chỉ xác nhận hồ sơ đã bàn giao", Record{"status": fromStatus})
		}
		updated, err = queryTx(ctx, tx, `
			UPDATE accounting_handoffs
			SET status='acknowledged',acknowledged_by=$2,acknowledged_at=now(),
			    external_reference=COALESCE(NULLIF($3,''),external_reference),
			    note=COALESCE(NULLIF($4,''),note),return_reason=NULL,updated_at=now()
			WHERE id=$1 RETURNING *
		`, id, actorID, req.ExternalReference, req.Note)
	case "return":
		if fromStatus != "handed_off" {
			return nil, validationError("Chỉ trả lại hồ sơ đã bàn giao", Record{"status": fromStatus})
		}
		updated, err = queryTx(ctx, tx, `
			UPDATE accounting_handoffs
			SET status='returned',returned_by=$2,returned_at=now(),return_reason=$3,
			    note=COALESCE(NULLIF($4,''),note),updated_at=now()
			WHERE id=$1 RETURNING *
		`, id, actorID, req.ReturnReason, req.Note)
	case "handoff":
		if fromStatus != "returned" {
			return nil, validationError("Chỉ bàn giao lại hồ sơ đã bị trả lại", Record{"status": fromStatus})
		}
		source, sourceErr := accountingHandoffSourceTx(ctx, tx, cleanID(current["project_id"]), fmt.Sprint(current["source_type"]), cleanID(current["source_id"]))
		if sourceErr != nil {
			return nil, sourceErr
		}
		if !boolValue(source["ready"]) {
			return nil, validationError("Hồ sơ nguồn chưa đủ điều kiện bàn giao lại")
		}
		snapshot, marshalErr := json.Marshal(source["snapshot"])
		if marshalErr != nil {
			return nil, marshalErr
		}
		updated, err = queryTx(ctx, tx, `
			UPDATE accounting_handoffs
			SET status='handed_off',source_snapshot_json=$2::jsonb,handed_off_by=$3,handed_off_at=now(),
			    acknowledged_by=NULL,acknowledged_at=NULL,returned_by=NULL,returned_at=NULL,return_reason=NULL,
			    external_reference=COALESCE(NULLIF($4,''),external_reference),
			    note=COALESCE(NULLIF($5,''),note),updated_at=now()
			WHERE id=$1 RETURNING *
		`, id, string(snapshot), actorID, req.ExternalReference, req.Note)
	default:
		return nil, validationError("Hành động bàn giao kế toán không hợp lệ", Record{"action": action})
	}
	if err != nil {
		return nil, err
	}
	row := updated[0]
	projectID := cleanID(current["project_id"])
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "accounting_handoff_"+action, "accounting_handoff", id, "Cập nhật bàn giao hồ sơ kế toán", Record{"from_status": fromStatus, "to_status": row["status"], "return_reason": req.ReturnReason}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, projectID, req.WorkItemContext, projectFinanceWorkItemPolicy, "accounting_handoff", id, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return attachWorkItemTransition(row, transition), nil
}

func accountingHandoffSourceTx(ctx context.Context, tx *sql.Tx, projectID, sourceType, sourceID string) (Record, error) {
	var rows []Record
	var err error
	switch sourceType {
	case "procurement_request":
		rows, err = queryTx(ctx, tx, `
			SELECT request.id,request.project_id,'payable'::text AS direction,request.request_code AS source_code,
			       request.title AS source_title,COALESCE(partner.name,request.supplier_name,'') AS counterparty_name,
			       request.total_estimated_amount AS reference_amount,'VND'::text AS currency,
			       request.status AS source_status,request.order_reference,
			       (request.status IN ('ordered','partially_received','received')
			        AND NULLIF(btrim(COALESCE(partner.name,request.supplier_name,'')),'') IS NOT NULL
			        AND NULLIF(btrim(COALESCE(request.order_reference,'')),'') IS NOT NULL) AS ready
			FROM procurement_requests request
			LEFT JOIN partners partner ON partner.id=request.supplier_partner_id
			WHERE request.id=$1 AND request.project_id=$2 AND request.status <> 'cancelled'
			FOR UPDATE OF request
		`, sourceID, projectID)
	case "contract_milestone":
		rows, err = queryTx(ctx, tx, `
			SELECT milestone.id,contract.project_id,'receivable'::text AS direction,
			       milestone.milestone_code AS source_code,milestone.milestone_name AS source_title,
			       contract.counterparty_name,milestone.net_amount AS reference_amount,contract.currency,
			       milestone.status AS source_status,milestone.acceptance_required,milestone.acceptance_id,
			       (milestone.status IN ('active','eligible')
			        AND NULLIF(btrim(contract.counterparty_name),'') IS NOT NULL
			        AND ((NOT milestone.acceptance_required) OR EXISTS (
			          SELECT 1 FROM acceptance_records acceptance
			          WHERE acceptance.id=milestone.acceptance_id AND acceptance.status='completed'
			            AND acceptance.result IN ('pass','conditional_pass')
			        ))) AS ready
			FROM contract_milestones milestone
			JOIN contracts contract ON contract.id=milestone.contract_id
			WHERE milestone.id=$1 AND contract.project_id=$2 AND contract.direction='receivable'
			  AND contract.contract_type='main_client' AND contract.status IN ('active','completed')
			FOR UPDATE OF milestone
		`, sourceID, projectID)
	default:
		return nil, validationError("Loại hồ sơ nguồn không hợp lệ", Record{"source_type": sourceType})
	}
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, validationError("Hồ sơ nguồn không thuộc công trình hoặc không còn hiệu lực")
	}
	row := rows[0]
	snapshot := Record{}
	for _, key := range []string{"id", "project_id", "direction", "source_code", "source_title", "counterparty_name", "reference_amount", "currency", "source_status", "order_reference", "acceptance_required", "acceptance_id"} {
		if value, ok := row[key]; ok {
			snapshot[key] = value
		}
	}
	row["snapshot"] = snapshot
	return row, nil
}

func (r *Repository) AccountingHandoffCloseReadiness(ctx context.Context, projectID string) (Record, error) {
	items, err := r.AccountingHandoffCandidates(ctx, projectID)
	if err != nil {
		return nil, err
	}
	pending := 0
	acknowledged := 0
	for _, item := range items {
		if fmt.Sprint(item["handoff_status"]) == "acknowledged" {
			acknowledged++
		} else {
			pending++
		}
	}
	handover, err := r.QueryOne(ctx, `
		SELECT COUNT(*) AS total,MAX(id::text) AS target_id
		FROM handover_records
		WHERE project_id=$1 AND status='handed_over' AND completed_at IS NOT NULL
	`, projectID)
	if err != nil {
		return nil, err
	}
	return Record{
		"ready":              number(handover["total"]) > 0 && pending == 0,
		"physical_handover":  number(handover["total"]) > 0,
		"pending_handoffs":   pending,
		"acknowledged_count": acknowledged,
		"target_id":          handover["target_id"],
	}, nil
}
