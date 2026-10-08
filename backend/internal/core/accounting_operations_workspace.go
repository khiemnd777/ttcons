package core

import "context"

func (s *Service) AccountingOperationsWorkspace(ctx context.Context, project, actor string) (Record, error) {
	if err := s.accountingAccess(ctx, project, actor, "accounting_reconciliation.view"); err != nil {
		return nil, err
	}
	result := Record{}
	projectRow, err := s.repo.QueryOne(ctx, `SELECT id,code,name FROM projects WHERE id=$1 AND status<>'cancelled'`, project)
	if err != nil {
		return nil, err
	}
	result["project"] = projectRow
	queries := []struct{ key, sql string }{
		{"bases", `SELECT b.*,p.name AS partner_name,
 COALESCE(a.allocated,0)::text AS allocated_amount,(COALESCE(a.allocated,0)-b.amount)::text AS difference_amount,
 ` + accountingBasisFingerprintExpr + ` AS fingerprint,
 COALESCE((SELECT fingerprint=` + accountingBasisFingerprintExpr + ` FROM accounting_misa_reviews WHERE basis_id=b.id ORDER BY created_at DESC,id DESC LIMIT 1),false) AS reviewed
 FROM accounting_reconciliation_bases b JOIN partners p ON p.id=b.partner_id
 LEFT JOIN LATERAL (SELECT SUM(amount) AS allocated,string_agg(id::text||':'||amount::text||':'||record_id::text,',' ORDER BY id) AS keys FROM accounting_misa_allocations WHERE basis_id=b.id AND active) a ON true
 WHERE b.project_id=$1 ORDER BY b.created_at DESC,b.id`},
		{"records", `SELECT m.*,p.name AS partner_name,b.filename,b.confirmed_at,
 (m.amount-COALESCE((SELECT SUM(amount) FROM accounting_misa_allocations WHERE record_id=m.id AND active),0))::text AS available_amount
 FROM accounting_misa_records m JOIN partners p ON p.id=m.partner_id JOIN accounting_misa_batches b ON b.id=m.batch_id WHERE m.project_id=$1 ORDER BY m.record_date DESC,m.id`},
		{"batches", `SELECT id,entity_code,filename,digest,status,created_at,confirmed_at FROM accounting_misa_batches WHERE project_id=$1 ORDER BY created_at DESC,id`},
		{"allocations", `SELECT a.*,m.external_id,m.line_key,m.record_date,m.currency FROM accounting_misa_allocations a JOIN accounting_misa_records m ON m.id=a.record_id WHERE a.project_id=$1 ORDER BY a.created_at DESC,a.id`},
		{"events", `SELECT e.id,COALESCE(b.id,a.basis_id) AS basis_id,e.event_type AS action,e.created_at,
 CASE WHEN e.event_type='accounting_basis_confirmed' THEN to_jsonb(b) || jsonb_build_object('amount',b.amount::text,
 'snapshot',b.snapshot || jsonb_build_object('reference_amount',b.snapshot->>'reference_amount')) ELSE e.metadata_json END AS snapshot
 FROM audit_logs e
 LEFT JOIN accounting_reconciliation_bases b ON b.id=e.target_id AND e.target_type='accounting_reconciliation_basis'
 LEFT JOIN accounting_misa_allocations a ON a.id=e.target_id AND e.target_type='accounting_misa_allocation'
 WHERE e.project_id=$1 AND e.event_type IN ('accounting_basis_confirmed','accounting_misa_import_confirmed','accounting_misa_allocated','accounting_misa_allocation_revoked','accounting_misa_reviewed')
 ORDER BY e.created_at DESC,e.id`},
		{"reviews", `SELECT * FROM accounting_misa_reviews WHERE project_id=$1 ORDER BY created_at DESC,id`},
	}
	for _, q := range queries {
		rows, err := s.repo.Query(ctx, q.sql, project)
		if err != nil {
			return nil, err
		}
		result[q.key] = rows
	}
	for _, basis := range result["bases"].([]Record) {
		available, err := s.accountingBasisAvailable(ctx, project, cleanID(basis["id"]), actor)
		if err != nil {
			return nil, err
		}
		basis["source_available"] = available
		basis["source_unavailable_reason"] = ""
		basis["reviewed"] = boolValue(basis["reviewed"]) && available
		if !available {
			basis["source_unavailable_reason"] = "Hồ sơ nguồn hoặc minh chứng không còn đủ điều kiện truy cập và đối soát"
		}
	}
	caps := Record{}
	for _, p := range []string{"accounting_reconciliation.manage", "accounting_misa.reconcile", "procurement.view", "contract.view", "warehouse.view"} {
		ok, err := s.repo.CanAccessProjectPermission(ctx, actor, project, []string{p})
		if err != nil {
			return nil, err
		}
		caps[p] = ok
	}
	result["capabilities"] = caps
	sources := []Record{}
	if boolValue(caps["procurement.view"]) {
		rows, err := s.repo.Query(ctx, `SELECT id,'procurement_request'::text AS source_type,request_code AS code,title,'VND'::text AS currency,'outbound'::text AS direction,supplier_partner_id AS partner_id,total_estimated_amount::text AS reference_amount FROM procurement_requests WHERE project_id=$1 AND status IN ('ordered','partially_received','received') AND supplier_partner_id IN (SELECT id FROM partners WHERE active) ORDER BY request_code`, project)
		if err != nil {
			return nil, err
		}
		sources = append(sources, rows...)
	}
	if boolValue(caps["contract.view"]) {
		rows, err := s.repo.Query(ctx, `SELECT m.id,'contract_milestone'::text AS source_type,m.milestone_code AS code,m.milestone_name AS title,c.currency,c.counterparty_partner_id AS partner_id,CASE c.direction WHEN 'receivable' THEN 'inbound' ELSE 'outbound' END AS direction,m.net_amount::text AS reference_amount FROM contract_milestones m JOIN contracts c ON c.id=m.contract_id WHERE c.project_id=$1 AND c.status IN ('active','completed') AND m.status IN ('active','eligible') AND c.counterparty_partner_id IN (SELECT id FROM partners WHERE active) AND ((c.contract_type='main_client' AND c.direction='receivable') OR (c.contract_type='subcontractor' AND c.direction='payable')) ORDER BY c.contract_code,m.sequence_no`, project)
		if err != nil {
			return nil, err
		}
		sources = append(sources, rows...)
	}
	result["sources"] = sources
	result["stock_documents"] = []Record{}
	if boolValue(caps["warehouse.view"]) {
		rows, err := s.repo.Query(ctx, `SELECT id,document_code,document_date,note FROM stock_documents WHERE project_id=$1 AND status='posted' AND movement_type='issue' ORDER BY document_date DESC,id`, project)
		if err != nil {
			return nil, err
		}
		result["stock_documents"] = rows
	}
	evidence, err := s.repo.Query(ctx, `SELECT id FROM evidence_records WHERE project_id=$1 AND status IN ('draft','submitted','accepted') ORDER BY created_at DESC,id`, project)
	if err != nil {
		return nil, err
	}
	files := []Record{}
	for _, e := range evidence {
		file, err := s.EvidenceDetail(ctx, cleanID(e["id"]), actor)
		if err != nil && !handoffReferenceUnavailable(err) {
			return nil, err
		}
		if err == nil {
			files = append(files, Record{"id": e["id"], "name": file["name"], "work_item_id": file["work_item_id"]})
		}
	}
	result["evidence"] = files
	work, err := s.repo.Query(ctx, `SELECT id,title FROM work_items WHERE project_id=$1 AND status IN ('ready','in_progress','needs_changes') ORDER BY title,id`, project)
	if err != nil {
		return nil, err
	}
	uploadTargets := []Record{}
	for _, w := range work {
		allowed, accessErr := s.repo.CanActOnWorkItem(ctx, cleanID(w["id"]), actor)
		if accessErr != nil {
			return nil, accessErr
		}
		if allowed {
			uploadTargets = append(uploadTargets, w)
		}
	}
	result["upload_targets"] = uploadTargets
	return result, nil
}
