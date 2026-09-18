package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// AccountingHandoffDetail is a read projection. It does not create obligations,
// infer links from names/materials, or alter the stored handoff snapshot.
func (s *Service) AccountingHandoffDetail(ctx context.Context, projectID, sourceType, sourceID, actorID string) (Record, error) {
	if !validActivityUUID(projectID) || !validActivityUUID(sourceID) || !statusIn(sourceType, "procurement_request", "contract_milestone") {
		return nil, validationError("Hồ sơ nguồn không hợp lệ")
	}
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"accounting_handoff.view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem hồ sơ kế toán của công trình")
	}
	project, err := s.repo.QueryOne(ctx, `SELECT id,code,name FROM projects WHERE id=$1 AND status <> 'cancelled'`, projectID)
	if err != nil {
		return nil, err
	}
	handoffs, err := s.repo.Query(ctx, `
		SELECT h.*, sender.full_name AS handed_off_by_name, receiver.full_name AS acknowledged_by_name,
		       returner.full_name AS returned_by_name
		FROM accounting_handoffs h
		LEFT JOIN users sender ON sender.id=h.handed_off_by
		LEFT JOIN users receiver ON receiver.id=h.acknowledged_by
		LEFT JOIN users returner ON returner.id=h.returned_by
		WHERE h.project_id=$1 AND h.source_type=$2 AND h.source_id=$3 AND h.status <> 'cancelled'
	`, projectID, sourceType, sourceID)
	if err != nil {
		return nil, err
	}
	var handoff Record
	if len(handoffs) > 0 {
		handoff = handoffs[0]
	}
	permission := "procurement.view"
	if sourceType == "contract_milestone" {
		permission = "contract.view"
	}
	canReadSource, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{permission})
	if err != nil {
		return nil, err
	}
	var current Record
	related := []Record{}
	files := []Record{}
	restricted := !canReadSource
	if canReadSource {
		current, err = s.repo.accountingHandoffCurrentSource(ctx, projectID, sourceType, sourceID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if current != nil {
			related, files, restricted, err = s.accountingHandoffRelated(ctx, projectID, sourceType, sourceID, actorID, current)
			if err != nil {
				return nil, err
			}
		}
	}
	if current == nil && handoff == nil {
		if !canReadSource {
			return nil, forbiddenError("Không thể mở hồ sơ nguồn theo quyền hiện tại")
		}
		return nil, sql.ErrNoRows
	}
	capabilities := Record{"handoff": false, "acknowledge": false, "return": false}
	status := "needs_attention"
	if handoff != nil {
		status = fmt.Sprint(handoff["status"])
	}
	// Reuse the canonical readiness rules; detail never adds a new completion gate.
	if current != nil {
		candidates, queryErr := s.repo.AccountingHandoffCandidates(ctx, projectID)
		if queryErr != nil {
			return nil, queryErr
		}
		for _, candidate := range candidates {
			if cleanID(candidate["source_id"]) != sourceID || fmt.Sprint(candidate["source_type"]) != sourceType {
				continue
			}
			current["requirements"] = candidate["requirements"]
			current["readiness"] = candidate["readiness"]
			if handoff == nil {
				status = fmt.Sprint(candidate["readiness"])
			}
			if (handoff == nil || status == "returned") && candidate["readiness"] == "ready" {
				capabilities["handoff"], err = s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"accounting_handoff.submit"})
				if err != nil {
					return nil, err
				}
			}
		}
	}
	if status == "handed_off" {
		canAct, accessErr := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"accounting_handoff.acknowledge"})
		if accessErr != nil {
			return nil, accessErr
		}
		capabilities["acknowledge"], capabilities["return"] = canAct, canAct
	}
	return Record{"project": project, "project_id": projectID, "source_type": sourceType, "source_id": sourceID, "current": current,
		"handoff": handoff, "related": related, "files": files, "restricted": restricted,
		"source_access": canReadSource, "status": status, "capabilities": capabilities}, nil
}

func (r *Repository) accountingHandoffCurrentSource(ctx context.Context, projectID, sourceType, sourceID string) (Record, error) {
	if sourceType == "procurement_request" {
		return r.QueryOne(ctx, `SELECT request.id,request.request_code AS source_code,request.title AS source_title,
			request.supplier_partner_id AS partner_id,COALESCE(partner.name,request.supplier_name,'') AS counterparty_name,request.total_estimated_amount AS reference_amount,
			'VND'::text AS currency,request.status AS source_status,request.order_reference
			FROM procurement_requests request LEFT JOIN partners partner ON partner.id=request.supplier_partner_id
			WHERE request.id=$1 AND request.project_id=$2 AND request.status <> 'cancelled'`, sourceID, projectID)
	}
	return r.QueryOne(ctx, `SELECT m.id,m.milestone_code AS source_code,m.milestone_name AS source_title,
		c.counterparty_partner_id AS partner_id,c.counterparty_name,m.net_amount AS reference_amount,c.currency,
		m.status AS source_status,c.id AS contract_id,c.contract_code,m.acceptance_id
		FROM contract_milestones m JOIN contracts c ON c.id=m.contract_id
		WHERE m.id=$1 AND c.project_id=$2 AND c.contract_type='main_client' AND c.direction='receivable'
		AND m.status <> 'cancelled' AND c.status NOT IN ('cancelled','terminated')`, sourceID, projectID)
}

// Source permissions and work-item visibility are evaluated before returning
// related identities or file metadata. Omitted items expose no IDs or counts.
func (s *Service) accountingHandoffRelated(ctx context.Context, projectID, sourceType, sourceID, actorID string, current Record) ([]Record, []Record, bool, error) {
	related, files := []Record{}, []Record{}
	restricted := false
	seenFiles := map[string]bool{}
	addEvidence := func(id string) error {
		if id == "" || seenFiles["evidence:"+id] {
			return nil
		}
		seenFiles["evidence:"+id] = true
		evidence, err := s.EvidenceDetail(ctx, id, actorID)
		if handoffReferenceUnavailable(err) {
			restricted = true
			return nil
		}
		if err != nil {
			return err
		}
		if cleanID(evidence["project_id"]) != projectID || statusIn(fmt.Sprint(evidence["status"]), "deleted", "cancelled") {
			return nil
		}
		files = append(files, Record{"kind": "evidence", "id": id, "name": evidence["name"], "mime_type": evidence["mime_type"]})
		return nil
	}
	addAcceptance := func(id string, incoming bool) error {
		var row Record
		var err error
		kind := "acceptance"
		if incoming {
			kind = "incoming_acceptance"
			row, err = s.GetIncomingAcceptance(ctx, id, actorID)
		} else {
			row, err = s.GetAcceptance(ctx, id, actorID)
		}
		if handoffReferenceUnavailable(err) {
			restricted = true
			return nil
		}
		if err != nil {
			return err
		}
		if cleanID(row["project_id"]) != projectID || statusIn(fmt.Sprint(row["status"]), "deleted", "cancelled") {
			return nil
		}
		related = append(related, Record{"kind": kind, "id": id, "label": row["acceptance_code"], "status": row["status"]})
		query := `SELECT evidence_id FROM acceptance_checklist_items WHERE acceptance_id=$1 AND evidence_id IS NOT NULL
			UNION SELECT link.evidence_id FROM acceptance_checklist_item_evidence link
			JOIN acceptance_checklist_items item ON item.id=link.checklist_item_id WHERE item.acceptance_id=$1`
		if incoming {
			query = `SELECT evidence_id FROM incoming_acceptance_items WHERE incoming_acceptance_id=$1 AND project_id=$2 AND evidence_id IS NOT NULL`
		}
		args := []any{id}
		if incoming {
			args = append(args, projectID)
		}
		evidenceRows, err := s.repo.Query(ctx, query, args...)
		if err != nil {
			return err
		}
		for _, e := range evidenceRows {
			if err := addEvidence(cleanID(e["evidence_id"])); err != nil {
				return err
			}
		}
		if pdfID := cleanID(row["pdf_export_id"]); !incoming && pdfID != "" {
			pdf, pdfErr := s.repo.GetWorkflowFormPDFExportDetail(ctx, pdfID)
			if handoffReferenceUnavailable(pdfErr) {
				restricted = true
				return nil
			}
			if pdfErr != nil {
				return pdfErr
			}
			if cleanID(pdf["project_id"]) != projectID {
				return nil
			}
			allowed, accessErr := s.repo.CanViewWorkItem(ctx, cleanID(pdf["work_item_id"]), actorID)
			if accessErr != nil {
				return accessErr
			}
			if !allowed {
				restricted = true
				return nil
			}
			if !seenFiles["workflow_pdf:"+pdfID] {
				seenFiles["workflow_pdf:"+pdfID] = true
				files = append(files, Record{"kind": "workflow_pdf", "id": pdfID, "name": pdf["file_name"], "mime_type": "application/pdf"})
			}
		}
		return nil
	}
	if sourceType == "procurement_request" {
		related = append(related, Record{"kind": "procurement_request", "id": sourceID, "label": current["source_code"], "status": current["source_status"]})
		acceptances, err := s.repo.Query(ctx, `SELECT id FROM incoming_acceptance_records WHERE project_id=$1 AND procurement_request_id=$2 AND status <> 'cancelled' ORDER BY created_at,id`, projectID, sourceID)
		if err != nil {
			return nil, nil, false, err
		}
		for _, a := range acceptances {
			if err := addAcceptance(cleanID(a["id"]), true); err != nil {
				return nil, nil, false, err
			}
		}
		canViewStock, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"warehouse.view"})
		if err != nil {
			return nil, nil, false, err
		}
		if canViewStock {
			stock, err := s.repo.accountingHandoffStockReferences(ctx, projectID, sourceID)
			if err != nil {
				return nil, nil, false, err
			}
			related = append(related, stock...)
		} else {
			restricted = true
		}
	} else {
		related = append(related, Record{"kind": "contract", "id": current["contract_id"], "label": current["contract_code"], "status": ""})
		if id := cleanID(current["acceptance_id"]); id != "" {
			if err := addAcceptance(id, false); err != nil {
				return nil, nil, false, err
			}
		}
		// Exact contract/dispatch documents only, never every file in the project.
		documents, err := s.repo.Query(ctx, `SELECT d.id,d.name,d.mime_type,d.file_size,d.created_at
			FROM documents d WHERE d.project_id=$1 AND d.status='active'
			AND d.storage_provider IN ('local','s3') AND NULLIF(btrim(d.storage_key),'') IS NOT NULL
			AND ((d.target_type='contract' AND d.target_id=$2) OR
			(d.target_type IN ('contract_dispatch','contract_returned_scan') AND EXISTS
			(SELECT 1 FROM contract_dispatches dispatch WHERE dispatch.id=d.target_id AND dispatch.contract_id=$2)))
			ORDER BY d.created_at,d.id`, projectID, current["contract_id"])
		if err != nil {
			return nil, nil, false, err
		}
		for _, d := range documents {
			d["kind"] = "document"
			files = append(files, d)
		}
	}
	return related, files, restricted, nil
}

func handoffReferenceUnavailable(err error) bool {
	var statusErr StatusError
	return errors.Is(err, sql.ErrNoRows) || (errors.As(err, &statusErr) && (statusErr.Status == 403 || statusErr.Status == 404))
}

func (r *Repository) accountingHandoffStockReferences(ctx context.Context, projectID, procurementID string) ([]Record, error) {
	// EXISTS deduplicates a multi-line document. Each path checks its source type
	// and project; matching a material or an untyped UUID is never sufficient.
	return r.Query(ctx, `SELECT 'stock_document'::text AS kind,d.id,d.document_code AS label,d.status
		FROM stock_documents d WHERE d.project_id=$1 AND d.movement_type='receive' AND d.status='posted'
		AND (
		 (d.reference_type='procurement_request' AND d.reference_id=$2)
		 OR (d.reference_type='incoming_acceptance' AND EXISTS (
		  SELECT 1 FROM incoming_acceptance_records a WHERE a.id=d.reference_id AND a.project_id=$1 AND a.procurement_request_id=$2 AND a.status <> 'cancelled'))
		 OR EXISTS (SELECT 1 FROM stock_document_items line
		  JOIN incoming_acceptance_items item ON item.id=line.source_line_id AND item.project_id=$1
		  JOIN incoming_acceptance_records a ON a.id=item.incoming_acceptance_id AND a.project_id=$1
		  WHERE line.document_id=d.id AND line.source_line_type='incoming_acceptance_item' AND a.procurement_request_id=$2 AND a.status <> 'cancelled')
		 OR EXISTS (SELECT 1 FROM stock_movements movement
		  JOIN procurement_request_items item ON item.boq_item_id=movement.reference_id AND item.request_id=$2
		  WHERE movement.stock_document_id=d.id AND movement.project_id=$1 AND movement.movement_type='receive' AND movement.reference_type='boq_item')
		) ORDER BY d.document_date,d.id`, projectID, procurementID)
}
