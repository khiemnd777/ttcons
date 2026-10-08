package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"golang.org/x/text/currency"
)

type AccountingBasisInput struct {
	SourceType  string   `json:"source_type"`
	SourceID    string   `json:"source_id"`
	Amount      string   `json:"amount"`
	Note        string   `json:"note"`
	EvidenceIDs []string `json:"evidence_ids"`
}

var accountingDecimal = regexp.MustCompile(`^\d{1,15}(\.\d{1,3})?$`)

// Decimal validation and SQL NUMERIC arithmetic deliberately avoid float64 money.
func accountingMoney(value, code string, zero bool) error {
	unit, err := currency.ParseISO(code)
	if err != nil || len(code) != 3 || code != strings.ToUpper(code) {
		return validationError("Mã tiền tệ không hợp lệ")
	}
	digits, _ := currency.Standard.Rounding(unit)
	parts := strings.Split(value, ".")
	if !accountingDecimal.MatchString(value) || (len(parts) == 2 && len(parts[1]) > digits) {
		return validationError("Số tiền không đúng độ chính xác của tiền tệ")
	}
	n, ok := new(big.Rat).SetString(value)
	if !ok || n.Sign() < 0 || (!zero && n.Sign() == 0) {
		return validationError("Số tiền phải lớn hơn 0")
	}
	return nil
}
func accountingJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

func (s *Service) accountingAccess(ctx context.Context, project, actor, permission string) error {
	if !validActivityUUID(project) {
		return validationError("Công trình không hợp lệ")
	}
	if _, err := s.repo.QueryOne(ctx, `SELECT id FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL`, actor); errors.Is(err, sql.ErrNoRows) {
		return forbiddenError("Tài khoản không còn được phép xử lý hồ sơ kế toán")
	} else if err != nil {
		return err
	}
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actor, project, []string{permission})
	if err != nil {
		return err
	}
	if !allowed {
		return forbiddenError("Không có quyền xử lý hồ sơ kế toán công trình")
	}
	_, err = s.repo.QueryOne(ctx, `SELECT id FROM projects WHERE id=$1 AND status<>'cancelled'`, project)
	return err
}
func accountingLock(ctx context.Context, tx *sql.Tx, project string) error {
	// All source confirmation, import and allocation writers serialize per project.
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('accounting_operations:' || $1,0))`, project)
	if err != nil {
		return err
	}
	var id string
	return tx.QueryRowContext(ctx, `SELECT id FROM projects WHERE id=$1 AND status<>'cancelled' FOR UPDATE`, project).Scan(&id)
}
func accountingSourcePermission(kind string) string {
	if kind == "procurement_request" {
		return "procurement.view"
	}
	return "contract.view"
}

func accountingSourceTx(ctx context.Context, tx *sql.Tx, project, kind, id string) (Record, error) {
	var rows []Record
	var err error
	if kind == "procurement_request" {
		rows, err = queryTx(ctx, tx, `SELECT p.id,p.request_code AS code,p.title,p.supplier_partner_id AS partner_id,
  partner.name AS partner_name,'outbound'::text AS direction,'VND'::text AS currency,p.total_estimated_amount AS reference_amount,p.status
  FROM procurement_requests p JOIN partners partner ON partner.id=p.supplier_partner_id
  WHERE p.id=$1 AND p.project_id=$2 AND partner.active AND p.status IN ('ordered','partially_received','received') FOR SHARE OF p,partner`, id, project)
	} else if kind == "contract_milestone" {
		rows, err = queryTx(ctx, tx, `SELECT m.id,m.milestone_code AS code,m.milestone_name AS title,c.counterparty_partner_id AS partner_id,
 c.counterparty_name AS partner_name,CASE c.direction WHEN 'receivable' THEN 'inbound' ELSE 'outbound' END AS direction,c.currency,
 m.net_amount AS reference_amount,m.status,c.contract_type,c.id AS contract_id,m.acceptance_id
 FROM contract_milestones m JOIN contracts c ON c.id=m.contract_id JOIN partners partner ON partner.id=c.counterparty_partner_id AND partner.active
 WHERE m.id=$1 AND c.project_id=$2 AND c.status IN ('active','completed') AND m.status IN ('active','eligible')
 AND ((c.contract_type='main_client' AND c.direction='receivable') OR (c.contract_type='subcontractor' AND c.direction='payable')) FOR SHARE OF m,c,partner`, id, project)
	} else {
		return nil, validationError("Loại nguồn không hợp lệ")
	}
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, validationError("Nguồn chưa đủ điều kiện hoặc chưa liên kết đối tác")
	}
	return rows[0], nil
}

func (s *Service) accountingEvidence(ctx context.Context, project, actor string, ids []string) error {
	if len(ids) > 30 {
		return validationError("Tối đa 30 chứng từ")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !validActivityUUID(id) || seen[id] {
			return validationError("Chứng từ không hợp lệ hoặc bị lặp")
		}
		seen[id] = true
		row, err := s.EvidenceDetail(ctx, id, actor)
		if err != nil {
			return err
		}
		if cleanID(row["project_id"]) != project || !statusIn(fmt.Sprint(row["status"]), "draft", "submitted", "accepted") {
			return validationError("Chứng từ không thuộc công trình")
		}
	}
	return nil
}

// Canonical access is checked by the service; row locks protect eligibility until commit.
func accountingEvidenceStatusTx(ctx context.Context, tx *sql.Tx, project string, ids []string) error {
	if len(ids) == 0 {
		return validationError("Căn cứ chưa có chứng từ")
	}
	for _, id := range ids {
		rows, err := queryTx(ctx, tx, `SELECT e.id FROM evidence_records e JOIN work_items w ON w.id=e.work_item_id
			WHERE e.id=$1 AND e.project_id=$2 AND e.status IN ('draft','submitted','accepted') AND w.status<>'cancelled'
			FOR SHARE OF e,w`, id, project)
		if err != nil {
			return err
		}
		if len(rows) != 1 {
			return validationError("Chứng từ căn cứ đã thay đổi; kiểm tra lại hồ sơ")
		}
	}
	return nil
}

func (s *Service) accountingBasisAccess(ctx context.Context, project, id, actor string) error {
	row, err := s.repo.QueryOne(ctx, `SELECT source_type,evidence_ids FROM accounting_reconciliation_bases WHERE id=$1 AND project_id=$2`, id, project)
	if err != nil {
		return err
	}
	if err := s.accountingAccess(ctx, project, actor, accountingSourcePermission(fmt.Sprint(row["source_type"]))); err != nil {
		return err
	}
	return s.accountingEvidence(ctx, project, actor, stringSlice(row["evidence_ids"]))
}

func (s *Service) accountingBasisAvailable(ctx context.Context, project, id, actor string) (bool, error) {
	err := s.accountingBasisAccess(ctx, project, id, actor)
	if err == nil {
		var tx *sql.Tx
		tx, err = s.repo.db.BeginTx(ctx, nil)
		if err == nil {
			_, err = s.repo.accountingBasisTx(ctx, tx, project, id, actor)
			_ = tx.Rollback()
		}
	}
	if err == nil {
		return true, nil
	}
	var statusErr StatusError
	if errors.Is(err, sql.ErrNoRows) || (errors.As(err, &statusErr) && (statusErr.Status == 400 || statusErr.Status == 403 || statusErr.Status == 404)) {
		return false, nil
	}
	return false, err
}

func (r *Repository) accountingBasisTx(ctx context.Context, tx *sql.Tx, project, id, actor string) (Record, error) {
	rows, err := queryTx(ctx, tx, `SELECT * FROM accounting_reconciliation_bases WHERE id=$1 AND project_id=$2 FOR SHARE`, id, project)
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, validationError("Căn cứ không thuộc công trình")
	}
	basis := rows[0]
	kind := fmt.Sprint(basis["source_type"])
	authorizer := &Service{repo: r.withReadTransaction(tx)}
	if err := authorizer.accountingAccess(ctx, project, actor, accountingSourcePermission(kind)); err != nil {
		return nil, err
	}
	source, err := accountingSourceTx(ctx, tx, project, kind, cleanID(basis["source_id"]))
	if err != nil {
		return nil, err
	}
	for _, key := range []string{"partner_id", "direction", "currency"} {
		if fmt.Sprint(source[key]) != fmt.Sprint(basis[key]) {
			return nil, validationError("Đối tác, chiều giao dịch hoặc tiền tệ của nguồn đã thay đổi")
		}
	}
	if err := accountingEvidenceStatusTx(ctx, tx, project, stringSlice(basis["evidence_ids"])); err != nil {
		return nil, err
	}
	if err := authorizer.accountingAccess(ctx, project, actor, accountingSourcePermission(kind)); err != nil {
		return nil, err
	}
	if err := authorizer.accountingEvidence(ctx, project, actor, stringSlice(basis["evidence_ids"])); err != nil {
		return nil, err
	}
	return basis, nil
}

func (s *Service) CreateAccountingBasis(ctx context.Context, project string, in AccountingBasisInput, actor, name string) (Record, error) {
	if err := s.accountingAccess(ctx, project, actor, "accounting_reconciliation.manage"); err != nil {
		return nil, err
	}
	if !validActivityUUID(in.SourceID) || !statusIn(in.SourceType, "procurement_request", "contract_milestone") || strings.TrimSpace(in.Note) == "" || len(in.EvidenceIDs) == 0 {
		return nil, validationError("Cần nguồn, lý do xác nhận giá trị và chứng từ căn cứ")
	}
	if err := s.accountingAccess(ctx, project, actor, accountingSourcePermission(in.SourceType)); err != nil {
		return nil, err
	}
	if err := s.accountingEvidence(ctx, project, actor, in.EvidenceIDs); err != nil {
		return nil, err
	}
	return s.repo.createAccountingBasis(ctx, project, in, actor, name)
}

func (r *Repository) createAccountingBasis(ctx context.Context, project string, in AccountingBasisInput, actor, name string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = accountingLock(ctx, tx, project); err != nil {
		return nil, err
	}
	authorizer := &Service{repo: r.withReadTransaction(tx)}
	if err := authorizer.accountingAccess(ctx, project, actor, "accounting_reconciliation.manage"); err != nil {
		return nil, err
	}
	if err := authorizer.accountingAccess(ctx, project, actor, accountingSourcePermission(in.SourceType)); err != nil {
		return nil, err
	}
	if err := authorizer.accountingEvidence(ctx, project, actor, in.EvidenceIDs); err != nil {
		return nil, err
	}
	source, err := accountingSourceTx(ctx, tx, project, in.SourceType, in.SourceID)
	if err != nil {
		return nil, err
	}
	if err = accountingMoney(in.Amount, fmt.Sprint(source["currency"]), false); err != nil {
		return nil, err
	}
	var exists bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM accounting_reconciliation_bases WHERE project_id=$1 AND source_type=$2 AND source_id=$3)`, project, in.SourceType, in.SourceID).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, validationError("Nguồn đã có căn cứ đối soát xác nhận")
	}
	if err := accountingEvidenceStatusTx(ctx, tx, project, in.EvidenceIDs); err != nil {
		return nil, err
	}
	if err := authorizer.accountingAccess(ctx, project, actor, "accounting_reconciliation.manage"); err != nil {
		return nil, err
	}
	if err := authorizer.accountingAccess(ctx, project, actor, accountingSourcePermission(in.SourceType)); err != nil {
		return nil, err
	}
	if err := authorizer.accountingEvidence(ctx, project, actor, in.EvidenceIDs); err != nil {
		return nil, err
	}
	id := newID()
	_, err = tx.ExecContext(ctx, `INSERT INTO accounting_reconciliation_bases(id,project_id,source_type,source_id,partner_id,direction,currency,amount,note,snapshot,evidence_ids,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11::jsonb,$12)`, id, project, in.SourceType, in.SourceID, source["partner_id"], source["direction"], source["currency"], in.Amount, in.Note, accountingJSON(source), accountingJSON(in.EvidenceIDs), actor)
	if err != nil {
		return nil, err
	}
	if err = addAuditTx(ctx, tx, project, actor, name, "accounting_basis_confirmed", "accounting_reconciliation_basis", id, "Xác nhận căn cứ đối soát với hồ sơ nguồn", Record{"source_type": in.SourceType, "source_id": in.SourceID, "amount": in.Amount}); err != nil {
		return nil, err
	}
	return Record{"id": id}, tx.Commit()
}
