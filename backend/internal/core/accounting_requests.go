package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/currency"
)

type AccountingBasisInput struct {
	SourceType  string   `json:"source_type"`
	SourceID    string   `json:"source_id"`
	Amount      string   `json:"amount"`
	Note        string   `json:"note"`
	EvidenceIDs []string `json:"evidence_ids"`
}
type AccountingRequestInput struct {
	ID               string   `json:"id"`
	BasisID          string   `json:"basis_id"`
	Title            string   `json:"title"`
	Kind             string   `json:"kind"`
	DueDate          string   `json:"due_date"`
	Gross            string   `json:"gross_amount"`
	Recovery         string   `json:"recovery_amount"`
	Retention        string   `json:"retention_amount"`
	AdvanceID        string   `json:"advance_request_id"`
	ReleaseID        string   `json:"release_request_id"`
	Note             string   `json:"note"`
	EvidenceIDs      []string `json:"evidence_ids"`
	StockIDs         []string `json:"stock_document_ids"`
	ExpectedRevision int      `json:"expected_revision"`
}
type AccountingRequestActionInput struct {
	ExpectedRevision int    `json:"expected_revision"`
	Reason           string `json:"reason"`
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
	// All proposal, recovery, import and allocation writers serialize per project.
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('accounting_operations:' || $1,0))`, project)
	return err
}
func accountingSourcePermission(kind string) string {
	if kind == "procurement_request" {
		return "procurement.view"
	}
	return "contract.view"
}

func (r *Repository) accountingSourceAccess(ctx context.Context, project, actor, permission string) error {
	allowed, err := r.CanAccessProjectPermission(ctx, actor, project, []string{permission})
	if err != nil {
		return err
	}
	if !allowed {
		return forbiddenError("Không có quyền xem nguồn đề nghị")
	}
	return nil
}
func accountingSourceTx(ctx context.Context, tx *sql.Tx, project, kind, id string) (Record, error) {
	var rows []Record
	var err error
	if kind == "procurement_request" {
		rows, err = queryTx(ctx, tx, `SELECT p.id,p.request_code AS code,p.title,p.supplier_partner_id AS partner_id,
  partner.name AS partner_name,'outbound'::text AS direction,'VND'::text AS currency,p.total_estimated_amount AS reference_amount,p.status
  FROM procurement_requests p JOIN partners partner ON partner.id=p.supplier_partner_id
  WHERE p.id=$1 AND p.project_id=$2 AND partner.active AND p.status IN ('ordered','partially_received','received') FOR SHARE OF p`, id, project)
	} else if kind == "contract_milestone" {
		rows, err = queryTx(ctx, tx, `SELECT m.id,m.milestone_code AS code,m.milestone_name AS title,c.counterparty_partner_id AS partner_id,
 c.counterparty_name AS partner_name,CASE c.direction WHEN 'receivable' THEN 'inbound' ELSE 'outbound' END AS direction,c.currency,
 m.net_amount AS reference_amount,m.status,c.contract_type,c.id AS contract_id,m.acceptance_id
 FROM contract_milestones m JOIN contracts c ON c.id=m.contract_id JOIN partners partner ON partner.id=c.counterparty_partner_id AND partner.active
 WHERE m.id=$1 AND c.project_id=$2 AND c.status IN ('active','completed') AND m.status IN ('active','eligible','obligation_opened','paid')
 AND ((c.contract_type='main_client' AND c.direction='receivable') OR (c.contract_type='subcontractor' AND c.direction='payable')) FOR SHARE OF m,c`, id, project)
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

func (s *Service) CreateAccountingBasis(ctx context.Context, project string, in AccountingBasisInput, actor, name string) (Record, error) {
	if err := s.accountingAccess(ctx, project, actor, "accounting_request.review"); err != nil {
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
	source, err := accountingSourceTx(ctx, tx, project, in.SourceType, in.SourceID)
	if err != nil {
		return nil, err
	}
	if err = accountingMoney(in.Amount, fmt.Sprint(source["currency"]), false); err != nil {
		return nil, err
	}
	var exists bool
	err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM accounting_request_bases WHERE project_id=$1 AND source_type=$2 AND source_id=$3)`, project, in.SourceType, in.SourceID).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, validationError("Nguồn đã có căn cứ xác nhận; không tạo hạn mức trùng")
	}
	if in.SourceType == "contract_milestone" {
		var exceeds bool
		err = tx.QueryRowContext(ctx, `SELECT $1::numeric > net_amount FROM contract_milestones WHERE id=$2`, in.Amount, in.SourceID).Scan(&exceeds)
		if err != nil {
			return nil, err
		}
		if exceeds {
			return nil, validationError("Căn cứ vượt giá trị ròng của mốc hợp đồng")
		}
	}
	id := newID()
	_, err = tx.ExecContext(ctx, `INSERT INTO accounting_request_bases(id,project_id,source_type,source_id,partner_id,direction,currency,amount,note,snapshot,evidence_ids,created_by) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11::jsonb,$12)`, id, project, in.SourceType, in.SourceID, source["partner_id"], source["direction"], source["currency"], in.Amount, in.Note, accountingJSON(source), accountingJSON(in.EvidenceIDs), actor)
	if err != nil {
		return nil, err
	}
	if err = addAuditTx(ctx, tx, project, actor, name, "accounting_basis_confirmed", "accounting_request_basis", id, "Xác nhận căn cứ đề nghị, không ghi nhận công nợ", Record{"source_type": in.SourceType, "source_id": in.SourceID, "amount": in.Amount}); err != nil {
		return nil, err
	}
	return Record{"id": id}, tx.Commit()
}

func (s *Service) SaveAccountingRequest(ctx context.Context, project string, in AccountingRequestInput, actor, name string) (Record, error) {
	if in.EvidenceIDs == nil {
		in.EvidenceIDs = []string{}
	}
	if in.StockIDs == nil {
		in.StockIDs = []string{}
	}
	if err := s.accountingAccess(ctx, project, actor, "accounting_request.submit"); err != nil {
		return nil, err
	}
	if !validActivityUUID(in.ID) || !validActivityUUID(in.BasisID) || strings.TrimSpace(in.Title) == "" || !statusIn(in.Kind, "advance", "progress", "final", "retention_release") {
		return nil, validationError("Cần mã đề nghị, căn cứ, tên đợt và loại đề nghị")
	}
	if _, err := time.Parse("2006-01-02", in.DueDate); err != nil {
		return nil, validationError("Hạn đề nghị không hợp lệ")
	}
	if err := s.accountingEvidence(ctx, project, actor, in.EvidenceIDs); err != nil {
		return nil, err
	}
	if len(in.StockIDs) > 30 {
		return nil, validationError("Tối đa 30 phiếu kho")
	}
	if len(in.StockIDs) > 0 {
		if err := s.accountingAccess(ctx, project, actor, "warehouse.view"); err != nil {
			return nil, err
		}
	}
	return s.repo.saveAccountingRequest(ctx, project, in, actor, name)
}

func (r *Repository) saveAccountingRequest(ctx context.Context, project string, in AccountingRequestInput, actor, name string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = accountingLock(ctx, tx, project); err != nil {
		return nil, err
	}
	bases, err := queryTx(ctx, tx, `SELECT * FROM accounting_request_bases WHERE id=$1 AND project_id=$2`, in.BasisID, project)
	if err != nil {
		return nil, err
	}
	if len(bases) != 1 {
		return nil, validationError("Căn cứ không thuộc công trình")
	}
	basis := bases[0]
	for _, v := range []struct {
		amount string
		zero   bool
	}{{in.Gross, false}, {in.Recovery, true}, {in.Retention, true}} {
		if err = accountingMoney(v.amount, fmt.Sprint(basis["currency"]), v.zero); err != nil {
			return nil, err
		}
	}
	gross, _ := new(big.Rat).SetString(in.Gross)
	recovery, _ := new(big.Rat).SetString(in.Recovery)
	retention, _ := new(big.Rat).SetString(in.Retention)
	if gross.Cmp(new(big.Rat).Add(recovery, retention)) <= 0 || (in.Kind == "advance" && (recovery.Sign() > 0 || retention.Sign() > 0)) {
		return nil, validationError("Số tiền ròng phải dương; tạm ứng không có thu hồi hoặc giữ lại")
	}
	if (recovery.Sign() > 0 && !validActivityUUID(in.AdvanceID)) || (recovery.Sign() == 0 && in.AdvanceID != "") || in.AdvanceID == in.ID {
		return nil, validationError("Phải chọn đúng đề nghị tạm ứng để thu hồi")
	}
	if (in.Kind == "retention_release" && (!validActivityUUID(in.ReleaseID) || in.ReleaseID == in.ID || recovery.Sign() > 0 || retention.Sign() > 0)) || (in.Kind != "retention_release" && in.ReleaseID != "") {
		return nil, validationError("Giải phóng giữ lại phải liên kết đợt giữ lại, không có thu hồi hoặc giữ lại mới")
	}
	for _, ref := range []struct{ id, kind string }{{in.AdvanceID, "advance"}, {in.ReleaseID, ""}} {
		if ref.id != "" {
			var ok bool
			err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM accounting_requests WHERE id=$1 AND basis_id=$2 AND status='acknowledged' AND ($3='' OR kind=$3))`, ref.id, in.BasisID, ref.kind).Scan(&ok)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, validationError("Đợt tham chiếu không thuộc cùng căn cứ hoặc chưa tiếp nhận")
			}
		}
	}
	seen := map[string]bool{}
	for _, id := range in.StockIDs {
		if !validActivityUUID(id) || seen[id] {
			return nil, validationError("Phiếu kho không hợp lệ hoặc bị lặp")
		}
		seen[id] = true
		var ok bool
		err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM stock_documents WHERE id=$1 AND project_id=$2 AND movement_type='issue' AND status='posted')`, id, project).Scan(&ok)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, validationError("Chỉ liên kết phiếu xuất kho đã ghi nhận của công trình")
		}
	}
	old, err := queryTx(ctx, tx, `SELECT * FROM accounting_requests WHERE id=$1 FOR UPDATE`, in.ID)
	if err != nil {
		return nil, err
	}
	revision := 1
	if len(old) > 0 {
		row := old[0]
		if cleanID(row["project_id"]) != project || cleanID(row["basis_id"]) != in.BasisID || !statusIn(fmt.Sprint(row["status"]), "draft", "returned") || int(number(row["revision"])) != in.ExpectedRevision {
			return nil, validationError("Đề nghị đã thay đổi hoặc không còn cho phép sửa; tải lại hồ sơ")
		}
		revision = in.ExpectedRevision + 1
	} else if in.ExpectedRevision != 0 {
		return nil, validationError("Không tìm thấy phiên bản đề nghị")
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO accounting_requests(id,project_id,basis_id,title,kind,due_date,gross_amount,recovery_amount,retention_amount,advance_request_id,note,evidence_ids,stock_document_ids,created_by,updated_by,release_request_id)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,NULLIF($10,'')::uuid,$11,$12::jsonb,$13::jsonb,$14,$14,NULLIF($16,'')::uuid)
 ON CONFLICT(id) DO UPDATE SET title=$4,kind=$5,due_date=$6,gross_amount=$7,recovery_amount=$8,retention_amount=$9,advance_request_id=NULLIF($10,'')::uuid,note=$11,evidence_ids=$12::jsonb,stock_document_ids=$13::jsonb,revision=$15,updated_by=$14,release_request_id=NULLIF($16,'')::uuid,updated_at=now()`, in.ID, project, in.BasisID, in.Title, in.Kind, in.DueDate, in.Gross, in.Recovery, in.Retention, in.AdvanceID, in.Note, accountingJSON(in.EvidenceIDs), accountingJSON(in.StockIDs), actor, revision, in.ReleaseID)
	if err != nil {
		return nil, err
	}
	if err = accountingRequestEvent(ctx, tx, in.ID, actor, "save"); err != nil {
		return nil, err
	}
	if err = addAuditTx(ctx, tx, project, actor, name, "accounting_request_saved", "accounting_request", in.ID, "Lưu đề nghị kế toán", Record{"revision": revision}); err != nil {
		return nil, err
	}
	return Record{"id": in.ID, "revision": revision}, tx.Commit()
}

func accountingRequestEvent(ctx context.Context, tx *sql.Tx, id, actor, action string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO accounting_request_events(request_id,revision,action,snapshot,actor_id)
 SELECT r.id,r.revision,$2,to_jsonb(r)||jsonb_build_object('basis',to_jsonb(b)),$3 FROM accounting_requests r JOIN accounting_request_bases b ON b.id=r.basis_id WHERE r.id=$1`, id, action, actor)
	return err
}

func (s *Service) AccountingRequestAction(ctx context.Context, project, id, action string, in AccountingRequestActionInput, actor, name string) (Record, error) {
	permission := "accounting_request.review"
	if action == "submit" || action == "cancel" {
		permission = "accounting_request.submit"
	}
	if err := s.accountingAccess(ctx, project, actor, permission); err != nil {
		return nil, err
	}
	if !validActivityUUID(id) || !statusIn(action, "submit", "acknowledge", "return", "cancel") {
		return nil, validationError("Thao tác không hợp lệ")
	}
	if action == "submit" {
		row, err := s.repo.QueryOne(ctx, `SELECT r.evidence_ids,b.evidence_ids AS basis_evidence FROM accounting_requests r JOIN accounting_request_bases b ON b.id=r.basis_id WHERE r.id=$1 AND r.project_id=$2`, id, project)
		if err != nil {
			return nil, err
		}
		for _, key := range []string{"evidence_ids", "basis_evidence"} {
			if err = s.accountingEvidence(ctx, project, actor, stringSlice(row[key])); err != nil {
				return nil, err
			}
		}
	}
	return s.repo.accountingRequestAction(ctx, project, id, action, in, actor, name)
}

func (r *Repository) accountingRequestAction(ctx context.Context, project, id, action string, in AccountingRequestActionInput, actor, name string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = accountingLock(ctx, tx, project); err != nil {
		return nil, err
	}
	rows, err := queryTx(ctx, tx, `SELECT r.*,b.amount AS basis_amount,b.source_type,b.source_id,b.currency FROM accounting_requests r JOIN accounting_request_bases b ON b.id=r.basis_id WHERE r.id=$1 AND r.project_id=$2 FOR UPDATE OF r`, id, project)
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, sql.ErrNoRows
	}
	row := rows[0]
	if int(number(row["revision"])) != in.ExpectedRevision {
		return nil, validationError("Hồ sơ đã đổi; tải lại trước khi thao tác")
	}
	status := fmt.Sprint(row["status"])
	next := ""
	switch action {
	case "submit":
		if !statusIn(status, "draft", "returned") {
			return nil, validationError("Chỉ gửi bản nháp hoặc hồ sơ trả bổ sung")
		}
		if err = r.accountingSourceAccess(ctx, project, actor, accountingSourcePermission(fmt.Sprint(row["source_type"]))); err != nil {
			return nil, err
		}
		if _, err = accountingSourceTx(ctx, tx, project, fmt.Sprint(row["source_type"]), cleanID(row["source_id"])); err != nil {
			return nil, err
		}
		var referencesValid bool
		err = tx.QueryRowContext(ctx, `SELECT NOT EXISTS(SELECT 1 FROM jsonb_array_elements_text(r.stock_document_ids) x WHERE NOT EXISTS(SELECT 1 FROM stock_documents d WHERE d.id=x.value::uuid AND d.project_id=r.project_id AND d.status='posted' AND d.movement_type='issue')) AND NOT EXISTS(SELECT 1 FROM jsonb_array_elements_text(r.evidence_ids||b.evidence_ids) x WHERE NOT EXISTS(SELECT 1 FROM evidence_records e WHERE e.id=x.value::uuid AND e.project_id=r.project_id AND e.status IN ('draft','submitted','accepted'))) FROM accounting_requests r JOIN accounting_request_bases b ON b.id=r.basis_id WHERE r.id=$1`, id).Scan(&referencesValid)
		if err != nil {
			return nil, err
		}
		if !referencesValid {
			return nil, validationError("Chứng từ hoặc phiếu kho đã thay đổi; kiểm tra lại căn cứ trước khi gửi")
		}
		var valid bool
		// Net requested + held value is reserved once. Recovered advances already reserve the earlier request.
		err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(gross_amount-recovery_amount),0)+CASE WHEN $6='retention_release' THEN 0 ELSE $3::numeric-$4::numeric END <= $5::numeric FROM accounting_requests WHERE basis_id=$1 AND id<>$2 AND status IN ('submitted','acknowledged') AND kind<>'retention_release'`, row["basis_id"], id, row["gross_amount"], row["recovery_amount"], row["basis_amount"], row["kind"]).Scan(&valid)
		if err != nil {
			return nil, err
		}
		if !valid {
			return nil, validationError("Tổng đề nghị vượt căn cứ đã xác nhận")
		}
		if row["kind"] == "retention_release" {
			err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM accounting_requests a WHERE a.id=$1 AND a.basis_id=$2 AND a.status='acknowledged' AND a.retention_amount >= $3::numeric+COALESCE((SELECT SUM(r.gross_amount) FROM accounting_requests r WHERE r.release_request_id=a.id AND r.id<>$4 AND r.status IN ('submitted','acknowledged')),0))`, row["release_request_id"], row["basis_id"], row["gross_amount"], id).Scan(&valid)
			if err != nil {
				return nil, err
			}
			if !valid {
				return nil, validationError("Giải phóng vượt giá trị giữ lại đã tiếp nhận")
			}
		}
		if cleanID(row["advance_request_id"]) != "" {
			err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM accounting_requests a WHERE a.id=$1 AND a.basis_id=$2 AND a.kind='advance' AND a.status='acknowledged' AND a.amount >= $3::numeric + COALESCE((SELECT SUM(r.recovery_amount) FROM accounting_requests r WHERE r.advance_request_id=a.id AND r.id<>$4 AND r.status IN ('submitted','acknowledged')),0))`, row["advance_request_id"], row["basis_id"], row["recovery_amount"], id).Scan(&valid)
			if err != nil {
				return nil, err
			}
			if !valid {
				return nil, validationError("Thu hồi vượt tạm ứng đã tiếp nhận hoặc sai căn cứ")
			}
		}
		next = "submitted"
	case "acknowledge":
		if status != "submitted" {
			return nil, validationError("Hồ sơ không chờ tiếp nhận")
		}
		next = "acknowledged"
	case "return":
		if status != "submitted" || strings.TrimSpace(in.Reason) == "" {
			return nil, validationError("Cần hồ sơ chờ tiếp nhận và lý do trả bổ sung")
		}
		next = "returned"
	case "cancel":
		if !statusIn(status, "draft", "returned") || strings.TrimSpace(in.Reason) == "" {
			return nil, validationError("Chỉ hủy bản nháp/trả bổ sung và phải ghi lý do")
		}
		next = "cancelled"
	}
	_, err = tx.ExecContext(ctx, `UPDATE accounting_requests SET status=$2,reason=$3,revision=revision+1,updated_by=$4,updated_at=now() WHERE id=$1`, id, next, in.Reason, actor)
	if err != nil {
		return nil, err
	}
	if err = accountingRequestEvent(ctx, tx, id, actor, action); err != nil {
		return nil, err
	}
	if err = addAuditTx(ctx, tx, project, actor, name, "accounting_request_"+action, "accounting_request", id, "Thao tác hồ sơ đề nghị; không xác nhận thu/chi", Record{"reason": in.Reason}); err != nil {
		return nil, err
	}
	return Record{"id": id, "status": next}, tx.Commit()
}
