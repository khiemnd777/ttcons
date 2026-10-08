package core

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"
)

const accountingMISAHeader = "external_id,line_key,kind,direction,partner_id,currency,amount,record_date,replaces_id"

type AccountingMISAImportInput struct {
	Entity   string `json:"entity_code"`
	Filename string `json:"filename"`
	Content  string `json:"content"`
}
type AccountingMISAAllocationInput struct {
	ID       string `json:"id"`
	BasisID  string `json:"basis_id"`
	RecordID string `json:"record_id"`
	Amount   string `json:"amount"`
}

type AccountingMISAReviewInput struct {
	BasisID     string `json:"basis_id"`
	Note        string `json:"note"`
	Fingerprint string `json:"fingerprint"`
}

type AccountingMISARevokeInput struct {
	Reason string `json:"reason"`
}

func parseAccountingMISA(content string) ([]Record, error) {
	if len(content) == 0 || len(content) > 2*1024*1024 {
		return nil, validationError("CSV phải có dữ liệu và không quá 2 MB")
	}
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(content, "\ufeff")))
	header, err := r.Read()
	if err != nil || strings.Join(header, ",") != accountingMISAHeader {
		return nil, validationError("Tiêu đề CSV không khớp mẫu Consflow", Record{"header": accountingMISAHeader})
	}
	rows := []Record{}
	seen := map[string]bool{}
	for {
		fields, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, validationError("CSV lỗi ở dòng dữ liệu", Record{"row": len(rows) + 2})
		}
		if len(rows) >= 2000 {
			return nil, validationError("Mỗi đợt nhập tối đa 2000 dòng")
		}
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		if len(fields) != 9 || fields[0] == "" || fields[1] == "" || !statusIn(fields[2], "transaction", "balance") || !statusIn(fields[3], "inbound", "outbound") || !validActivityUUID(fields[4]) || (fields[8] != "" && !validActivityUUID(fields[8])) {
			return nil, validationError("Dòng CSV thiếu mã hoặc không hợp lệ", Record{"row": len(rows) + 2})
		}
		if err = accountingMoney(fields[6], fields[5], fields[2] == "balance"); err != nil {
			return nil, validationError("Số tiền CSV không hợp lệ", Record{"row": len(rows) + 2})
		}
		if _, err = time.Parse("2006-01-02", fields[7]); err != nil {
			return nil, validationError("Ngày CSV phải là YYYY-MM-DD", Record{"row": len(rows) + 2})
		}
		key := fields[0] + "\x00" + fields[1] + "\x00" + fields[2]
		if seen[key] {
			return nil, validationError("CSV có dòng chứng từ trùng")
		}
		seen[key] = true
		row := Record{}
		for i, k := range header {
			row[k] = fields[i]
		}
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, validationError("CSV chưa có dòng dữ liệu")
	}
	return rows, nil
}

func (s *Service) PreviewAccountingMISA(ctx context.Context, project string, in AccountingMISAImportInput, actor string) (Record, error) {
	if err := s.accountingAccess(ctx, project, actor, "accounting_misa.reconcile"); err != nil {
		return nil, err
	}
	in.Entity = strings.TrimSpace(in.Entity)
	if in.Entity == "" || len(in.Entity) > 100 || strings.TrimSpace(in.Filename) == "" {
		return nil, validationError("Cần mã đơn vị kế toán và tên file")
	}
	rows, err := parseAccountingMISA(in.Content)
	if err != nil {
		return nil, err
	}
	return s.repo.previewAccountingMISA(ctx, project, in, actor, rows)
}

func (r *Repository) previewAccountingMISA(ctx context.Context, project string, in AccountingMISAImportInput, actor string, rows []Record) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := accountingLock(ctx, tx, project); err != nil {
		return nil, err
	}
	authorizer := &Service{repo: r.withReadTransaction(tx)}
	if err := authorizer.accountingAccess(ctx, project, actor, "accounting_misa.reconcile"); err != nil {
		return nil, err
	}
	for _, row := range rows {
		partners, err := queryTx(ctx, tx, `SELECT id FROM partners WHERE id=$1 AND active FOR SHARE`, row["partner_id"])
		if err != nil {
			return nil, err
		}
		if len(partners) != 1 {
			return nil, validationError("Mã đối tác chưa được ánh xạ tới đối tác hoạt động", Record{"external_id": row["external_id"]})
		}
	}
	sum := sha256.Sum256([]byte(in.Content))
	digest := hex.EncodeToString(sum[:])
	batches, err := queryTx(ctx, tx, `INSERT INTO accounting_misa_batches(project_id,entity_code,filename,file_content,digest,created_by) VALUES($1,$2,$3,$4,$5,$6)
 ON CONFLICT(project_id,entity_code,digest) DO UPDATE SET digest=EXCLUDED.digest RETURNING id,status,digest`, project, in.Entity, in.Filename, in.Content, digest, actor)
	if err != nil {
		return nil, err
	}
	if err := authorizer.accountingAccess(ctx, project, actor, "accounting_misa.reconcile"); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	batch := batches[0]
	batch["rows"] = rows
	return batch, nil
}

func (s *Service) AccountingMISAFile(ctx context.Context, project, id, actor string) (Record, error) {
	if err := s.accountingAccess(ctx, project, actor, "accounting_reconciliation.view"); err != nil {
		return nil, err
	}
	if !validActivityUUID(id) {
		return nil, validationError("Đợt nhập không hợp lệ")
	}
	return s.repo.QueryOne(ctx, `SELECT filename,file_content FROM accounting_misa_batches WHERE id=$1 AND project_id=$2`, id, project)
}

func (s *Service) ConfirmAccountingMISA(ctx context.Context, project, id, actor, name string) (Record, error) {
	if err := s.accountingAccess(ctx, project, actor, "accounting_misa.reconcile"); err != nil {
		return nil, err
	}
	if !validActivityUUID(id) {
		return nil, validationError("Đợt nhập không hợp lệ")
	}
	return s.repo.confirmAccountingMISA(ctx, project, id, actor, name)
}

func (r *Repository) confirmAccountingMISA(ctx context.Context, project, id, actor, name string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = accountingLock(ctx, tx, project); err != nil {
		return nil, err
	}
	authorizer := &Service{repo: r.withReadTransaction(tx)}
	if err := authorizer.accountingAccess(ctx, project, actor, "accounting_misa.reconcile"); err != nil {
		return nil, err
	}
	batches, err := queryTx(ctx, tx, `SELECT * FROM accounting_misa_batches WHERE id=$1 AND project_id=$2 FOR UPDATE`, id, project)
	if err != nil {
		return nil, err
	}
	if len(batches) != 1 {
		return nil, sql.ErrNoRows
	}
	b := batches[0]
	if b["status"] == "confirmed" {
		return Record{"id": id, "status": "confirmed"}, nil
	}
	rows, err := parseAccountingMISA(fmt.Sprint(b["file_content"]))
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		partners, err := queryTx(ctx, tx, `SELECT id FROM partners WHERE id=$1 AND active FOR SHARE`, row["partner_id"])
		if err != nil {
			return nil, err
		}
		if len(partners) != 1 {
			return nil, validationError("Đối tác không còn hoạt động")
		}
		old, err := queryTx(ctx, tx, `SELECT * FROM accounting_misa_records WHERE project_id=$1 AND entity_code=$2 AND external_id=$3 AND line_key=$4 AND kind=$5 AND active FOR UPDATE`, project, b["entity_code"], row["external_id"], row["line_key"], row["kind"])
		if err != nil {
			return nil, err
		}
		replace := fmt.Sprint(row["replaces_id"])
		if len(old) > 0 {
			if replace != cleanID(old[0]["id"]) {
				return nil, validationError("Chứng từ đã tồn tại; phải chỉ rõ phiên bản thay thế", Record{"external_id": row["external_id"], "current_id": old[0]["id"]})
			}
			var allocated bool
			err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM accounting_misa_allocations WHERE record_id=$1 AND active)`, replace).Scan(&allocated)
			if err != nil {
				return nil, err
			}
			if allocated {
				return nil, validationError("Cần thu hồi phân bổ trước khi thay thế giao dịch")
			}
			if _, err = tx.ExecContext(ctx, `UPDATE accounting_misa_records SET active=false WHERE id=$1`, replace); err != nil {
				return nil, err
			}
		} else if replace != "" {
			return nil, validationError("Phiên bản thay thế không khớp chứng từ hiện hành")
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO accounting_misa_records(batch_id,project_id,entity_code,external_id,line_key,kind,direction,partner_id,currency,amount,record_date,replaces_id)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,NULLIF($12,'')::uuid)`, id, project, b["entity_code"], row["external_id"], row["line_key"], row["kind"], row["direction"], row["partner_id"], row["currency"], row["amount"], row["record_date"], replace)
		if err != nil {
			return nil, err
		}
	}
	if err := authorizer.accountingAccess(ctx, project, actor, "accounting_misa.reconcile"); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE accounting_misa_batches SET status='confirmed',confirmed_at=now() WHERE id=$1`, id); err != nil {
		return nil, err
	}
	if err = addAuditTx(ctx, tx, project, actor, name, "accounting_misa_import_confirmed", "accounting_misa_batch", id, "Nhập quan sát từ file MISA; không hạch toán", Record{"digest": b["digest"], "rows": len(rows)}); err != nil {
		return nil, err
	}
	return Record{"id": id, "status": "confirmed"}, tx.Commit()
}

const accountingBasisFingerprintExpr = `md5(to_jsonb(b)::text || ':' || COALESCE(a.keys,''))`
const accountingFingerprintSQL = `SELECT ` + accountingBasisFingerprintExpr + ` FROM accounting_reconciliation_bases b
 LEFT JOIN LATERAL (SELECT string_agg(id::text || ':' || amount::text || ':' || record_id::text,',' ORDER BY id) AS keys
 FROM accounting_misa_allocations WHERE basis_id=b.id AND active) a ON true WHERE b.id=$1`

func (s *Service) AllocateAccountingMISA(ctx context.Context, project string, in AccountingMISAAllocationInput, actor, name string) (Record, error) {
	if err := s.accountingAccess(ctx, project, actor, "accounting_misa.reconcile"); err != nil {
		return nil, err
	}
	if !validActivityUUID(in.ID) || !validActivityUUID(in.BasisID) || !validActivityUUID(in.RecordID) {
		return nil, validationError("Chọn căn cứ, giao dịch và mã thao tác")
	}
	if err := s.accountingBasisAccess(ctx, project, in.BasisID, actor); err != nil {
		return nil, err
	}
	return s.repo.allocateAccountingMISA(ctx, project, in, actor, name)
}

func (r *Repository) allocateAccountingMISA(ctx context.Context, project string, in AccountingMISAAllocationInput, actor, name string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = accountingLock(ctx, tx, project); err != nil {
		return nil, err
	}
	authorizer := &Service{repo: r.withReadTransaction(tx)}
	if err := authorizer.accountingAccess(ctx, project, actor, "accounting_misa.reconcile"); err != nil {
		return nil, err
	}
	basis, err := r.accountingBasisTx(ctx, tx, project, in.BasisID, actor)
	if err != nil {
		return nil, err
	}
	if err := accountingMoney(in.Amount, fmt.Sprint(basis["currency"]), false); err != nil {
		return nil, err
	}
	prior, err := queryTx(ctx, tx, `SELECT * FROM accounting_misa_allocations WHERE id=$1`, in.ID)
	if err != nil {
		return nil, err
	}
	if len(prior) > 0 {
		p := prior[0]
		if cleanID(p["project_id"]) == project && cleanID(p["basis_id"]) == in.BasisID && cleanID(p["record_id"]) == in.RecordID && boolValue(p["active"]) {
			var same bool
			err = tx.QueryRowContext(ctx, `SELECT amount=$2::numeric FROM accounting_misa_allocations WHERE id=$1`, in.ID, in.Amount).Scan(&same)
			if err != nil {
				return nil, err
			}
			if same {
				return Record{"id": in.ID}, nil
			}
		}
		return nil, validationError("Mã thao tác đã dùng với phân bổ khác; tải lại hồ sơ")
	}
	records, err := queryTx(ctx, tx, `SELECT m.* FROM accounting_misa_records m JOIN accounting_misa_batches b ON b.id=m.batch_id WHERE m.id=$1 AND m.project_id=$2 AND m.active AND m.kind='transaction' AND b.status='confirmed' FOR UPDATE OF m`, in.RecordID, project)
	if err != nil {
		return nil, err
	}
	if len(records) != 1 {
		return nil, validationError("Chỉ phân bổ giao dịch hiện hành của công trình")
	}
	record := records[0]
	if err = accountingMoney(in.Amount, fmt.Sprint(record["currency"]), false); err != nil {
		return nil, err
	}
	for _, key := range []string{"partner_id", "direction", "currency"} {
		if fmt.Sprint(basis[key]) != fmt.Sprint(record[key]) {
			return nil, validationError("Căn cứ và giao dịch khác đối tác, chiều giao dịch hoặc tiền tệ")
		}
	}
	var compatible bool
	err = tx.QueryRowContext(ctx, `SELECT amount >= $2::numeric+COALESCE((SELECT SUM(amount) FROM accounting_misa_allocations WHERE record_id=$1 AND active),0) FROM accounting_misa_records WHERE id=$1`, in.RecordID, in.Amount).Scan(&compatible)
	if err != nil {
		return nil, err
	}
	if !compatible {
		return nil, validationError("Phân bổ vượt giá trị giao dịch")
	}
	if err := authorizer.accountingAccess(ctx, project, actor, "accounting_misa.reconcile"); err != nil {
		return nil, err
	}
	if err := authorizer.accountingBasisAccess(ctx, project, in.BasisID, actor); err != nil {
		return nil, err
	}
	id := in.ID
	_, err = tx.ExecContext(ctx, `INSERT INTO accounting_misa_allocations(id,project_id,basis_id,record_id,amount,created_by) VALUES($1,$2,$3,$4,$5,$6)`, id, project, in.BasisID, in.RecordID, in.Amount, actor)
	if err != nil {
		return nil, err
	}
	if err = addAuditTx(ctx, tx, project, actor, name, "accounting_misa_allocated", "accounting_reconciliation_basis", in.BasisID, "Ghép giao dịch MISA với căn cứ đối soát", Record{"allocation_id": id, "record_id": in.RecordID, "amount": in.Amount}); err != nil {
		return nil, err
	}
	return Record{"id": id}, tx.Commit()
}

func (s *Service) RevokeAccountingAllocation(ctx context.Context, project, id, reason, actor, name string) (Record, error) {
	if err := s.accountingAccess(ctx, project, actor, "accounting_misa.reconcile"); err != nil {
		return nil, err
	}
	if !validActivityUUID(id) || strings.TrimSpace(reason) == "" {
		return nil, validationError("Cần phân bổ và lý do thu hồi")
	}
	return s.repo.revokeAccountingAllocation(ctx, project, id, reason, actor, name)
}

func (r *Repository) revokeAccountingAllocation(ctx context.Context, project, id, reason, actor, name string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = accountingLock(ctx, tx, project); err != nil {
		return nil, err
	}
	authorizer := &Service{repo: r.withReadTransaction(tx)}
	if err := authorizer.accountingAccess(ctx, project, actor, "accounting_misa.reconcile"); err != nil {
		return nil, err
	}
	rows, err := queryTx(ctx, tx, `SELECT id FROM accounting_misa_allocations WHERE id=$1 AND project_id=$2 AND active FOR UPDATE`, id, project)
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, validationError("Phân bổ không còn hiệu lực")
	}
	if err := authorizer.accountingAccess(ctx, project, actor, "accounting_misa.reconcile"); err != nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE accounting_misa_allocations SET active=false,revoked_by=$3,revoked_at=now(),revoke_reason=$4 WHERE id=$1 AND project_id=$2 AND active`, id, project, actor, reason)
	if err != nil {
		return nil, err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return nil, validationError("Phân bổ không còn hiệu lực")
	}
	if err = addAuditTx(ctx, tx, project, actor, name, "accounting_misa_allocation_revoked", "accounting_misa_allocation", id, "Thu hồi phân bổ", Record{"reason": reason}); err != nil {
		return nil, err
	}
	return Record{"id": id}, tx.Commit()
}

func (s *Service) ReviewAccountingMISA(ctx context.Context, project string, in AccountingMISAReviewInput, actor, name string) (Record, error) {
	if err := s.accountingAccess(ctx, project, actor, "accounting_misa.reconcile"); err != nil {
		return nil, err
	}
	if !validActivityUUID(in.BasisID) || strings.TrimSpace(in.Note) == "" {
		return nil, validationError("Cần căn cứ và ghi chú đối soát")
	}
	if err := s.accountingBasisAccess(ctx, project, in.BasisID, actor); err != nil {
		return nil, err
	}
	return s.repo.reviewAccountingMISA(ctx, project, in, actor, name)
}

func (r *Repository) reviewAccountingMISA(ctx context.Context, project string, in AccountingMISAReviewInput, actor, name string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err = accountingLock(ctx, tx, project); err != nil {
		return nil, err
	}
	authorizer := &Service{repo: r.withReadTransaction(tx)}
	if err := authorizer.accountingAccess(ctx, project, actor, "accounting_misa.reconcile"); err != nil {
		return nil, err
	}
	if _, err := r.accountingBasisTx(ctx, tx, project, in.BasisID, actor); err != nil {
		return nil, err
	}
	var fingerprint string
	err = tx.QueryRowContext(ctx, accountingFingerprintSQL, in.BasisID).Scan(&fingerprint)
	if err != nil {
		return nil, err
	}
	if fingerprint != in.Fingerprint {
		return nil, validationError("Phân bổ đã đổi; tải lại trước khi xác nhận")
	}
	if err := authorizer.accountingAccess(ctx, project, actor, "accounting_misa.reconcile"); err != nil {
		return nil, err
	}
	if err := authorizer.accountingBasisAccess(ctx, project, in.BasisID, actor); err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO accounting_misa_reviews(project_id,basis_id,fingerprint,note,actor_id) VALUES($1,$2,$3,$4,$5)`, project, in.BasisID, fingerprint, in.Note, actor)
	if err != nil {
		return nil, err
	}
	if err = addAuditTx(ctx, tx, project, actor, name, "accounting_misa_reviewed", "accounting_reconciliation_basis", in.BasisID, "Xác nhận đối soát dữ liệu đã nhập", Record{"fingerprint": fingerprint, "note": in.Note}); err != nil {
		return nil, err
	}
	return Record{"id": in.BasisID}, tx.Commit()
}
