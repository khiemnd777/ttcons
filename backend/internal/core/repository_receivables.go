package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (r *Repository) ListReceivablePaymentSchedules(ctx context.Context, receivableID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			rps.*,
			p.name AS project_name,
			rec.receivable_code,
			rec.client_name,
			COALESCE(created.full_name, '') AS created_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM receivable_payment_schedules rps
		JOIN projects p ON p.id=rps.project_id
		JOIN receivables rec ON rec.id=rps.receivable_id
		LEFT JOIN users created ON created.id=rps.created_by
		LEFT JOIN users approved ON approved.id=rps.approved_by
		WHERE rps.receivable_id=$1
		  AND rps.status NOT IN ('cancelled','deleted')
		  AND rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		ORDER BY rps.due_date NULLS LAST, rps.created_at DESC
	`, receivableID)
}

func (r *Repository) GetReceivablePaymentScheduleDetail(ctx context.Context, scheduleID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			rps.*,
			p.name AS project_name,
			rec.receivable_code,
			rec.client_name,
			COALESCE(created.full_name, '') AS created_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM receivable_payment_schedules rps
		JOIN projects p ON p.id=rps.project_id
		JOIN receivables rec ON rec.id=rps.receivable_id
		LEFT JOIN users created ON created.id=rps.created_by
		LEFT JOIN users approved ON approved.id=rps.approved_by
		WHERE rps.id=$1
		  AND rps.status NOT IN ('cancelled','deleted')
		  AND rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
	`, scheduleID)
}

func (r *Repository) CreateReceivablePaymentSchedule(ctx context.Context, receivable Record, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	scheduleID := newID()
	projectID := cleanID(receivable["project_id"])
	receivableID := cleanID(receivable["id"])
	rows, err := queryTx(ctx, tx, `
		INSERT INTO receivable_payment_schedules (
			id, project_id, receivable_id, schedule_code, milestone_name, schedule_type,
			planned_amount, received_amount, remaining_amount, due_date, status, notes, created_by
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,0,$7,NULLIF($8,'')::date,'draft',$9,NULLIF($10,'')::uuid)
		RETURNING *
	`, scheduleID, projectID, receivableID, body["schedule_code"], body["milestone_name"], body["schedule_type"], number(body["planned_amount"]), fmt.Sprint(body["due_date"]), body["notes"], actorID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "receivable_payment_schedule_created", "receivable_payment_schedule", scheduleID, "Tạo kỳ thu đầu ra", Record{
		"receivable_id":  receivableID,
		"schedule_code":  body["schedule_code"],
		"planned_amount": body["planned_amount"],
		"schedule_type":  body["schedule_type"],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetReceivablePaymentScheduleDetail(ctx, scheduleID)
}

func (r *Repository) ReceivablePaymentScheduleAction(ctx context.Context, scheduleID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	if action != "approve" {
		return nil, validationError("Hành động kỳ thu không hợp lệ", Record{"action": action})
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM receivable_payment_schedules WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, scheduleID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	if !statusIn(fromStatus, "draft", "overdue") {
		return nil, validationError("Trạng thái kỳ thu không cho phép duyệt", Record{"status": fromStatus})
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE receivable_payment_schedules
		SET status='approved',
			approved_by=NULLIF($2,'')::uuid,
			updated_at=now()
		WHERE id=$1 AND status=$3
		RETURNING *
	`, scheduleID, actorID, fromStatus)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái kỳ thu đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	comment := actionComment(req)
	metadata := Record{"action": action, "from_status": fromStatus, "to_status": "approved", "receivable_id": current["receivable_id"]}
	if comment != "" {
		metadata["comment"] = comment
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(current["project_id"]), actorID, actorName, "receivable_payment_schedule_approved", "receivable_payment_schedule", scheduleID, "Duyệt kỳ thu đầu ra", metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetReceivablePaymentScheduleDetail(ctx, scheduleID)
}

func (r *Repository) ListReceivableAdvanceReceipts(ctx context.Context, receivableID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			car.*,
			p.name AS project_name,
			rec.receivable_code,
			rec.client_name AS receivable_client_name,
			pmt.payment_code,
			pmt.payment_type,
			COALESCE(requested.full_name, '') AS requested_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM client_advance_receipts car
		JOIN projects p ON p.id=car.project_id
		JOIN receivables rec ON rec.id=car.receivable_id
		LEFT JOIN payments pmt ON pmt.id=car.payment_id
		LEFT JOIN users requested ON requested.id=car.requested_by
		LEFT JOIN users approved ON approved.id=car.approved_by
		WHERE car.receivable_id=$1
		  AND car.status NOT IN ('cancelled','deleted')
		  AND rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		ORDER BY car.due_date NULLS LAST, car.created_at DESC
	`, receivableID)
}

func (r *Repository) GetReceivableAdvanceReceiptDetail(ctx context.Context, advanceID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			car.*,
			p.name AS project_name,
			rec.receivable_code,
			rec.client_name AS receivable_client_name,
			pmt.payment_code,
			pmt.payment_type,
			COALESCE(requested.full_name, '') AS requested_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM client_advance_receipts car
		JOIN projects p ON p.id=car.project_id
		JOIN receivables rec ON rec.id=car.receivable_id
		LEFT JOIN payments pmt ON pmt.id=car.payment_id
		LEFT JOIN users requested ON requested.id=car.requested_by
		LEFT JOIN users approved ON approved.id=car.approved_by
		WHERE car.id=$1
		  AND car.status NOT IN ('cancelled','deleted')
		  AND rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
	`, advanceID)
}

func (r *Repository) CreateReceivableAdvanceReceipt(ctx context.Context, receivable Record, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	advanceID := newID()
	projectID := cleanID(receivable["project_id"])
	receivableID := cleanID(receivable["id"])
	rows, err := queryTx(ctx, tx, `
		INSERT INTO client_advance_receipts (
			id, project_id, receivable_id, receipt_code, requested_by, amount, reason, status,
			client_name, due_date, notes
		)
		VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid,$6,$7,'draft',$8,NULLIF($9,'')::date,$10)
		RETURNING *
	`, advanceID, projectID, receivableID, body["receipt_code"], actorID, number(body["amount"]), body["reason"], body["client_name"], fmt.Sprint(body["due_date"]), body["notes"])
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "receivable_advance_receipt_created", "client_advance_receipt", advanceID, "Tạo ứng trước đầu ra", Record{
		"receivable_id": receivableID,
		"receipt_code":  body["receipt_code"],
		"amount":        body["amount"],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetReceivableAdvanceReceiptDetail(ctx, advanceID)
}

func (r *Repository) ReceivableAdvanceReceiptAction(ctx context.Context, advanceID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	if action != "approve" {
		return nil, validationError("Hành động ứng trước đầu ra không hợp lệ", Record{"action": action})
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	advanceRows, err := queryTx(ctx, tx, `SELECT * FROM client_advance_receipts WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, advanceID)
	if err != nil {
		return nil, err
	}
	if len(advanceRows) == 0 {
		return nil, sql.ErrNoRows
	}
	advance := advanceRows[0]
	fromStatus := fmt.Sprint(advance["status"])
	if !statusIn(fromStatus, "draft", "submitted") {
		return nil, validationError("Trạng thái ứng trước đầu ra không cho phép duyệt", Record{"status": fromStatus})
	}
	receivableRows, err := queryTx(ctx, tx, `SELECT * FROM receivables WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, advance["receivable_id"])
	if err != nil {
		return nil, err
	}
	if len(receivableRows) == 0 {
		return nil, sql.ErrNoRows
	}
	receivable := receivableRows[0]
	if !statusIn(fmt.Sprint(receivable["status"]), "approved", "partially_received", "overdue") {
		return nil, validationError("Phải thu cần được duyệt trước khi duyệt ứng trước", Record{"status": receivable["status"]})
	}
	amount := number(advance["amount"])
	if amount <= 0 || amount > number(receivable["remaining_amount"]) {
		return nil, validationError("Số tiền ứng trước vượt số còn phải thu", Record{"amount": amount, "remaining_amount": receivable["remaining_amount"]})
	}
	updatedAdvance, err := queryTx(ctx, tx, `
		UPDATE client_advance_receipts
		SET status='approved',
			approved_by=NULLIF($2,'')::uuid,
			updated_at=now()
		WHERE id=$1 AND status=$3
		RETURNING *
	`, advanceID, actorID, fromStatus)
	if err != nil {
		return nil, err
	}
	if len(updatedAdvance) == 0 {
		return nil, validationError("Trạng thái ứng trước đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	newRemaining := number(receivable["remaining_amount"]) - amount
	newReceived := number(receivable["received_amount"]) + amount
	if _, err := tx.ExecContext(ctx, `
		UPDATE receivables
		SET received_amount=received_amount + $2,
			remaining_amount=GREATEST(remaining_amount - $2, 0),
			status=$3,
			approved_by=COALESCE(approved_by,NULLIF($4,'')::uuid),
			updated_at=now()
		WHERE id=$1
	`, advance["receivable_id"], amount, receivableSettlementStatus(newReceived, newRemaining), actorID); err != nil {
		return nil, err
	}
	comment := actionComment(req)
	metadata := Record{"action": action, "from_status": fromStatus, "to_status": "approved", "receivable_id": advance["receivable_id"], "amount": amount}
	if comment != "" {
		metadata["comment"] = comment
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(advance["project_id"]), actorID, actorName, "receivable_advance_receipt_approved", "client_advance_receipt", advanceID, "Duyệt ứng trước đầu ra", metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetReceivableAdvanceReceiptDetail(ctx, advanceID)
}

func (r *Repository) SettleReceivableAdvanceReceipt(ctx context.Context, advanceID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM client_advance_receipts WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, advanceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	if !statusIn(fromStatus, "approved", "partially_received", "received", "partially_settled") {
		return nil, validationError("Trạng thái ứng trước không cho phép quyết toán", Record{"status": fromStatus})
	}
	amount := number(body["amount"])
	remaining := number(current["amount"]) - number(current["settled_amount"])
	if amount <= 0 || amount > remaining {
		return nil, validationError("Số tiền quyết toán vượt số ứng trước còn lại", Record{"amount": amount, "remaining": remaining})
	}
	toStatus := "partially_settled"
	if amount >= remaining {
		toStatus = "settled"
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE client_advance_receipts
		SET settled_amount=settled_amount + $2,
			status=$3,
			settled_at=CASE WHEN $3='settled' THEN now() ELSE settled_at END,
			updated_at=now()
		WHERE id=$1 AND status=$4
		RETURNING *
	`, advanceID, amount, toStatus, fromStatus)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái ứng trước đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	metadata := Record{
		"from_status":       fromStatus,
		"to_status":         toStatus,
		"receivable_id":     current["receivable_id"],
		"amount":            amount,
		"output_invoice_id": cleanID(body["output_invoice_id"]),
		"schedule_id":       cleanID(body["schedule_id"]),
	}
	if comment := strings.TrimSpace(fmt.Sprint(body["comment"])); comment != "" {
		metadata["comment"] = comment
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(current["project_id"]), actorID, actorName, "receivable_advance_receipt_settled", "client_advance_receipt", advanceID, "Quyết toán ứng trước đầu ra", metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetReceivableAdvanceReceiptDetail(ctx, advanceID)
}

func (r *Repository) ListReceivablePayments(ctx context.Context, receivableID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			pmt.*,
			p.name AS project_name,
			ra.id AS allocation_id,
			ra.schedule_id,
			ra.advance_receipt_id,
			ra.output_invoice_id,
			ra.allocation_type,
			ra.amount AS allocation_amount,
			ra.notes AS allocation_notes,
			rps.schedule_code,
			rps.milestone_name,
			car.receipt_code AS advance_receipt_code,
			oi.invoice_number AS output_invoice_number,
			rec.receivable_code,
			rec.client_name AS receivable_client_name,
			COALESCE(created.full_name, '') AS created_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM payments pmt
		JOIN projects p ON p.id=pmt.project_id
		JOIN receipt_allocations ra ON ra.payment_id=pmt.id
		JOIN receivables rec ON rec.id=ra.receivable_id
		LEFT JOIN receivable_payment_schedules rps ON rps.id=ra.schedule_id
		LEFT JOIN client_advance_receipts car ON car.id=ra.advance_receipt_id
		LEFT JOIN output_invoices oi ON oi.id=ra.output_invoice_id
		LEFT JOIN users created ON created.id=pmt.created_by
		LEFT JOIN users approved ON approved.id=pmt.approved_by
		WHERE ra.receivable_id=$1
		  AND pmt.status <> 'cancelled'
		  AND rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		ORDER BY pmt.created_at DESC
	`, receivableID)
}

func (r *Repository) GetReceivablePaymentDetail(ctx context.Context, paymentID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			pmt.*,
			p.name AS project_name,
			ra.id AS allocation_id,
			ra.receivable_id,
			ra.schedule_id,
			ra.advance_receipt_id,
			ra.output_invoice_id,
			ra.allocation_type,
			ra.amount AS allocation_amount,
			ra.notes AS allocation_notes,
			rps.schedule_code,
			rps.milestone_name,
			car.receipt_code AS advance_receipt_code,
			oi.invoice_number AS output_invoice_number,
			rec.receivable_code,
			rec.client_name AS receivable_client_name,
			COALESCE(created.full_name, '') AS created_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM payments pmt
		JOIN projects p ON p.id=pmt.project_id
		JOIN receipt_allocations ra ON ra.payment_id=pmt.id
		JOIN receivables rec ON rec.id=ra.receivable_id
		LEFT JOIN receivable_payment_schedules rps ON rps.id=ra.schedule_id
		LEFT JOIN client_advance_receipts car ON car.id=ra.advance_receipt_id
		LEFT JOIN output_invoices oi ON oi.id=ra.output_invoice_id
		LEFT JOIN users created ON created.id=pmt.created_by
		LEFT JOIN users approved ON approved.id=pmt.approved_by
		WHERE pmt.id=$1
		  AND pmt.status <> 'cancelled'
		  AND rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
	`, paymentID)
}

func (r *Repository) CreateReceivablePayment(ctx context.Context, receivable Record, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	projectID := cleanID(receivable["project_id"])
	receivableID := cleanID(receivable["id"])
	receivableRows, err := queryTx(ctx, tx, `SELECT * FROM receivables WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, receivableID)
	if err != nil {
		return nil, err
	}
	if len(receivableRows) == 0 {
		return nil, sql.ErrNoRows
	}
	currentReceivable := receivableRows[0]
	amount := number(body["amount"])
	allocationType := fmt.Sprint(body["allocation_type"])
	scheduleID := cleanID(body["schedule_id"])
	advanceID := cleanID(body["advance_receipt_id"])
	if allocationType == "receivable_receipt" && scheduleID == "" && amount > number(currentReceivable["remaining_amount"]) {
		return nil, validationError("Số tiền thu vượt số còn phải thu", Record{"amount": amount, "remaining_amount": currentReceivable["remaining_amount"]})
	}
	if scheduleID != "" {
		scheduleRows, err := queryTx(ctx, tx, `SELECT * FROM receivable_payment_schedules WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, scheduleID)
		if err != nil {
			return nil, err
		}
		if len(scheduleRows) == 0 {
			return nil, sql.ErrNoRows
		}
		schedule := scheduleRows[0]
		if cleanID(schedule["receivable_id"]) != receivableID || !statusIn(fmt.Sprint(schedule["status"]), "approved", "partially_received", "overdue") {
			return nil, validationError("Kỳ thu không thuộc phải thu hoặc chưa được duyệt", Record{"schedule_id": scheduleID})
		}
		if amount > number(schedule["remaining_amount"]) {
			return nil, validationError("Số tiền thu vượt số còn lại của kỳ", Record{"amount": amount, "schedule_remaining": schedule["remaining_amount"]})
		}
	}
	if advanceID != "" {
		advanceRows, err := queryTx(ctx, tx, `SELECT * FROM client_advance_receipts WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, advanceID)
		if err != nil {
			return nil, err
		}
		if len(advanceRows) == 0 {
			return nil, sql.ErrNoRows
		}
		advance := advanceRows[0]
		if cleanID(advance["receivable_id"]) != receivableID || !statusIn(fmt.Sprint(advance["status"]), "approved", "partially_received", "received", "partially_settled") {
			return nil, validationError("Ứng trước không thuộc phải thu hoặc chưa được duyệt", Record{"advance_receipt_id": advanceID})
		}
		if amount > number(advance["amount"])-number(advance["received_amount"]) {
			return nil, validationError("Số tiền thu vượt số ứng trước chưa ghi nhận tiền", Record{"amount": amount})
		}
	}
	paymentID := newID()
	rows, err := queryTx(ctx, tx, `
		INSERT INTO payments (
			id, project_id, payment_code, payment_type, target_type, target_id,
			amount, payment_method, status, paid_at, created_by,
			accounting_book_id, cash_account_id
		)
		VALUES (
			$1,$2,$3,$4,'receivable',$5,$6,$7,'draft',NULLIF($8,'')::timestamptz,NULLIF($9,'')::uuid,
			NULLIF($10,'')::uuid,NULLIF($11,'')::uuid
		)
		RETURNING *
	`, paymentID, projectID, body["payment_code"], body["payment_type"], receivableID, amount, body["payment_method"], fmt.Sprint(body["paid_at"]), actorID,
		body["accounting_book_id"], body["cash_account_id"])
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO receipt_allocations (
			project_id, payment_id, receivable_id, schedule_id, advance_receipt_id, output_invoice_id,
			allocation_type, amount, notes, created_by
		)
		VALUES ($1,$2,$3,NULLIF($4,'')::uuid,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,$7,$8,$9,NULLIF($10,'')::uuid)
	`, projectID, paymentID, receivableID, scheduleID, advanceID, cleanID(body["output_invoice_id"]), allocationType, amount, body["notes"], actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "receivable_payment_created", "payment", paymentID, "Tạo phiếu thu đầu ra", Record{
		"receivable_id":      receivableID,
		"amount":             amount,
		"allocation_type":    allocationType,
		"schedule_id":        scheduleID,
		"advance_receipt_id": advanceID,
		"output_invoice_id":  cleanID(body["output_invoice_id"]),
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetReceivablePaymentDetail(ctx, paymentID)
}

func (r *Repository) GetReceivableDetail(ctx context.Context, receivableID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT rec.*, p.name AS project_name
		FROM receivables rec
		JOIN projects p ON p.id=rec.project_id
		WHERE rec.id=$1
		  AND rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
	`, receivableID)
}

func (r *Repository) ReceivableReconciliation(ctx context.Context, receivableID string) (Record, error) {
	summary, err := r.QueryOne(ctx, `
		WITH rec AS (
			SELECT rec.*, p.name AS project_name
			FROM receivables rec
			JOIN projects p ON p.id=rec.project_id
			WHERE rec.id=$1
			  AND rec.status <> 'cancelled'
			  AND p.status <> 'cancelled'
		),
		inv AS (
			SELECT
				receivable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('deleted','cancelled')) AS invoice_count,
				COUNT(*) FILTER (WHERE status='issued') AS issued_invoice_count,
				COUNT(*) FILTER (WHERE status='voided') AS voided_invoice_count,
				COALESCE(SUM(total_amount) FILTER (WHERE status NOT IN ('deleted','cancelled')), 0) AS invoice_total,
				COALESCE(SUM(total_amount) FILTER (WHERE status='issued'), 0) AS issued_invoice_total,
				MAX(invoice_date) FILTER (WHERE status NOT IN ('deleted','cancelled')) AS latest_invoice_date
			FROM output_invoices
			WHERE receivable_id=$1
			GROUP BY receivable_id
		),
		sch AS (
			SELECT
				receivable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('cancelled','deleted')) AS schedule_count,
				COUNT(*) FILTER (WHERE status IN ('approved','partially_received','overdue') AND remaining_amount > 0 AND due_date < CURRENT_DATE) AS overdue_schedule_count,
				COALESCE(SUM(planned_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS scheduled_total,
				COALESCE(SUM(received_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS scheduled_received_total,
				COALESCE(SUM(remaining_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS scheduled_remaining,
				MIN(due_date) FILTER (WHERE status IN ('approved','partially_received','overdue') AND remaining_amount > 0) AS next_schedule_due_date
			FROM receivable_payment_schedules
			WHERE receivable_id=$1
			GROUP BY receivable_id
		),
		adv AS (
			SELECT
				receivable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('cancelled','deleted')) AS advance_receipt_count,
				COALESCE(SUM(amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS advance_requested_total,
				COALESCE(SUM(amount) FILTER (WHERE status IN ('approved','partially_received','received','partially_settled','settled')), 0) AS advance_approved_total,
				COALESCE(SUM(received_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS advance_received_total,
				COALESCE(SUM(settled_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS advance_settled_total
			FROM client_advance_receipts
			WHERE receivable_id=$1
			GROUP BY receivable_id
		),
		alloc AS (
			SELECT
				ra.receivable_id,
				COALESCE(SUM(ra.amount) FILTER (WHERE pmt.status='approved' AND ra.allocation_type='receivable_receipt'), 0) AS approved_receipt_total,
				COALESCE(SUM(ra.amount) FILTER (WHERE pmt.status='approved' AND ra.allocation_type='advance_receipt'), 0) AS approved_advance_receipt_total,
				COALESCE(SUM(ra.amount) FILTER (WHERE pmt.status <> 'cancelled'), 0) AS receipt_allocation_total,
				COUNT(*) FILTER (WHERE pmt.status NOT IN ('approved','cancelled')) AS pending_receipt_count
			FROM receipt_allocations ra
			JOIN payments pmt ON pmt.id=ra.payment_id
			WHERE ra.receivable_id=$1
			GROUP BY ra.receivable_id
		),
		calc AS (
			SELECT
				rec.*,
				COALESCE(inv.invoice_count, 0) AS invoice_count,
				COALESCE(inv.issued_invoice_count, 0) AS issued_invoice_count,
				COALESCE(inv.voided_invoice_count, 0) AS voided_invoice_count,
				COALESCE(inv.invoice_total, 0) AS invoice_total,
				COALESCE(inv.issued_invoice_total, 0) AS issued_invoice_total,
				inv.latest_invoice_date,
				COALESCE(sch.schedule_count, 0) AS schedule_count,
				COALESCE(sch.overdue_schedule_count, 0) AS overdue_schedule_count,
				COALESCE(sch.scheduled_total, 0) AS scheduled_total,
				COALESCE(sch.scheduled_received_total, 0) AS scheduled_received_total,
				COALESCE(sch.scheduled_remaining, 0) AS scheduled_remaining,
				sch.next_schedule_due_date,
				COALESCE(adv.advance_receipt_count, 0) AS advance_receipt_count,
				COALESCE(adv.advance_requested_total, 0) AS advance_requested_total,
				COALESCE(adv.advance_approved_total, 0) AS advance_approved_total,
				COALESCE(adv.advance_received_total, 0) AS advance_received_total,
				COALESCE(adv.advance_settled_total, 0) AS advance_settled_total,
				COALESCE(alloc.approved_receipt_total, 0) AS approved_receipt_total,
				COALESCE(alloc.approved_advance_receipt_total, 0) AS approved_advance_receipt_total,
				COALESCE(alloc.receipt_allocation_total, 0) AS receipt_allocation_total,
				COALESCE(alloc.pending_receipt_count, 0) AS pending_receipt_count,
				COALESCE(rec.due_date, sch.next_schedule_due_date, inv.latest_invoice_date) AS effective_due_date,
				GREATEST(COALESCE(rec.gross_amount,0) - COALESCE(rec.received_amount,0), 0) AS computed_remaining
			FROM rec
			LEFT JOIN inv ON inv.receivable_id=rec.id
			LEFT JOIN sch ON sch.receivable_id=rec.id
			LEFT JOIN adv ON adv.receivable_id=rec.id
			LEFT JOIN alloc ON alloc.receivable_id=rec.id
		)
		SELECT
			calc.*,
			CASE
				WHEN COALESCE(remaining_amount,0) <= 0 THEN 0
				WHEN effective_due_date IS NULL THEN 0
				WHEN effective_due_date >= CURRENT_DATE THEN 0
				ELSE CURRENT_DATE - effective_due_date
			END AS overdue_days,
			CASE
				WHEN COALESCE(remaining_amount,0) <= 0 THEN 'settled'
				WHEN effective_due_date IS NULL THEN 'no_due_date'
				WHEN effective_due_date >= CURRENT_DATE THEN 'not_due'
				WHEN CURRENT_DATE - effective_due_date <= 30 THEN '1_30'
				WHEN CURRENT_DATE - effective_due_date <= 60 THEN '31_60'
				WHEN CURRENT_DATE - effective_due_date <= 90 THEN '61_90'
				ELSE '90_plus'
			END AS aging_bucket,
			ABS(COALESCE(remaining_amount,0) - COALESCE(computed_remaining,0)) AS balance_difference,
			GREATEST(COALESCE(gross_amount,0) - COALESCE(issued_invoice_total,0), 0) AS unissued_invoice_gap,
			GREATEST(COALESCE(advance_approved_total,0) - COALESCE(advance_received_total,0), 0) AS advance_unreceived_total,
			GREATEST(COALESCE(advance_received_total,0) - COALESCE(advance_settled_total,0), 0) AS advance_unsettled_total
		FROM calc
	`, receivableID)
	if err != nil {
		return nil, err
	}
	warnings := receivableReconciliationWarnings(summary)
	summary["warning_count"] = len(warnings)
	if len(warnings) > 0 {
		summary["reconciliation_status"] = "needs_review"
	} else {
		summary["reconciliation_status"] = "ok"
	}

	ledger, err := r.Query(ctx, `
		WITH entries AS (
			SELECT
				rec.created_at AS occurred_at,
				0 AS sort_order,
				'receivable_recorded'::text AS entry_type,
				rec.id::text AS source_id,
				rec.receivable_code::text AS source_code,
				rec.status::text AS status,
				'Ghi nhận phải thu'::text AS description,
				COALESCE(rec.gross_amount,0)::numeric AS debit_amount,
				0::numeric AS credit_amount,
				0::numeric AS document_amount,
				0::numeric AS cash_amount,
				COALESCE(created.full_name, '') AS actor_name
			FROM receivables rec
			LEFT JOIN users created ON created.id=rec.created_by
			WHERE rec.id=$1 AND rec.status <> 'cancelled'

			UNION ALL

			SELECT
				COALESCE(oi.issued_at, oi.voided_at, oi.created_at) AS occurred_at,
				10 AS sort_order,
				('output_invoice_' || oi.status)::text AS entry_type,
				oi.id::text AS source_id,
				oi.invoice_number::text AS source_code,
				oi.status::text AS status,
				CASE
					WHEN oi.status='issued' THEN 'Hoá đơn đầu ra đã phát hành'
					WHEN oi.status='voided' THEN 'Hoá đơn đầu ra đã huỷ'
					ELSE 'Tải lên hoá đơn đầu ra'
				END AS description,
				0::numeric AS debit_amount,
				0::numeric AS credit_amount,
				COALESCE(oi.total_amount,0)::numeric AS document_amount,
				0::numeric AS cash_amount,
				COALESCE(actor.full_name, '') AS actor_name
			FROM output_invoices oi
			LEFT JOIN users actor ON actor.id=COALESCE(oi.issued_by, oi.voided_by, oi.uploaded_by)
			WHERE oi.receivable_id=$1 AND oi.status NOT IN ('deleted','cancelled')

			UNION ALL

			SELECT
				car.updated_at AS occurred_at,
				20 AS sort_order,
				'client_advance_approved'::text AS entry_type,
				car.id::text AS source_id,
				car.receipt_code::text AS source_code,
				car.status::text AS status,
				'Ứng trước khách hàng được duyệt ghi giảm phải thu'::text AS description,
				0::numeric AS debit_amount,
				COALESCE(car.amount,0)::numeric AS credit_amount,
				0::numeric AS document_amount,
				0::numeric AS cash_amount,
				COALESCE(approved.full_name, '') AS actor_name
			FROM client_advance_receipts car
			LEFT JOIN users approved ON approved.id=car.approved_by
			WHERE car.receivable_id=$1 AND car.status IN ('approved','partially_received','received','partially_settled','settled')

			UNION ALL

			SELECT
				COALESCE(pmt.paid_at, pmt.updated_at, pmt.created_at) AS occurred_at,
				30 AS sort_order,
				('receipt_' || ra.allocation_type)::text AS entry_type,
				pmt.id::text AS source_id,
				pmt.payment_code::text AS source_code,
				pmt.status::text AS status,
				CASE
					WHEN ra.allocation_type='advance_receipt' THEN 'Thu tiền ứng trước khách hàng'
					WHEN ra.allocation_type='advance_settlement' THEN 'Quyết toán ứng trước khách hàng'
					ELSE 'Thu tiền phải thu'
				END AS description,
				0::numeric AS debit_amount,
				CASE WHEN ra.allocation_type='receivable_receipt' THEN COALESCE(ra.amount,0)::numeric ELSE 0::numeric END AS credit_amount,
				0::numeric AS document_amount,
				COALESCE(ra.amount,0)::numeric AS cash_amount,
				COALESCE(approved.full_name, created.full_name, '') AS actor_name
			FROM receipt_allocations ra
			JOIN payments pmt ON pmt.id=ra.payment_id
			LEFT JOIN users approved ON approved.id=pmt.approved_by
			LEFT JOIN users created ON created.id=pmt.created_by
			WHERE ra.receivable_id=$1 AND pmt.status='approved'

			UNION ALL

			SELECT
				COALESCE(car.settled_at, car.updated_at) AS occurred_at,
				40 AS sort_order,
				'client_advance_settled'::text AS entry_type,
				car.id::text AS source_id,
				car.receipt_code::text AS source_code,
				car.status::text AS status,
				'Quyết toán ứng trước bằng chứng từ đầu ra'::text AS description,
				0::numeric AS debit_amount,
				0::numeric AS credit_amount,
				COALESCE(car.settled_amount,0)::numeric AS document_amount,
				0::numeric AS cash_amount,
				''::text AS actor_name
			FROM client_advance_receipts car
			WHERE car.receivable_id=$1
			  AND car.status NOT IN ('cancelled','deleted')
			  AND COALESCE(car.settled_amount,0) > 0
		),
		running AS (
			SELECT
				entries.*,
				SUM(debit_amount - credit_amount) OVER (
					ORDER BY occurred_at ASC, sort_order ASC, source_id ASC
					ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
				) AS running_balance
			FROM entries
		)
		SELECT *
		FROM running
		ORDER BY occurred_at DESC, sort_order DESC, source_id DESC
	`, receivableID)
	if err != nil {
		return nil, err
	}

	return Record{
		"summary":  summary,
		"warnings": warnings,
		"ledger":   ledger,
	}, nil
}

func receivableReconciliationWarnings(summary Record) []Record {
	warnings := []Record{}
	if number(summary["remaining_amount"]) > 0 && int(number(summary["overdue_days"])) > 0 {
		severity := "warning"
		if number(summary["overdue_days"]) > 60 {
			severity = "critical"
		}
		warnings = append(warnings, receivableWarning("overdue", severity, "Khoản phải thu đã quá hạn", Record{
			"effective_due_date": summary["effective_due_date"],
			"overdue_days":       summary["overdue_days"],
			"remaining_amount":   summary["remaining_amount"],
		}))
	}
	if number(summary["issued_invoice_total"]) > number(summary["gross_amount"])+1 {
		warnings = append(warnings, receivableWarning("invoice_over_gross", "critical", "Tổng hoá đơn issued vượt giá trị phải thu", Record{
			"gross_amount":         summary["gross_amount"],
			"issued_invoice_total": summary["issued_invoice_total"],
		}))
	}
	if number(summary["received_amount"]) > number(summary["issued_invoice_total"])+1 {
		warnings = append(warnings, receivableWarning("received_without_issued_invoice", "warning", "Đã thu nhiều hơn tổng hoá đơn issued", Record{
			"received_amount":        summary["received_amount"],
			"issued_invoice_total":   summary["issued_invoice_total"],
			"approved_receipt_total": summary["approved_receipt_total"],
		}))
	}
	if number(summary["remaining_amount"]) > 0 && int(number(summary["schedule_count"])) == 0 {
		warnings = append(warnings, receivableWarning("missing_schedule", "warning", "Khoản phải thu còn dư nhưng chưa có lịch thu", Record{
			"remaining_amount": summary["remaining_amount"],
		}))
	}
	if number(summary["scheduled_remaining"]) > number(summary["remaining_amount"])+1 {
		warnings = append(warnings, receivableWarning("schedule_over_remaining", "warning", "Số còn lại theo lịch lớn hơn số còn phải thu", Record{
			"scheduled_remaining": summary["scheduled_remaining"],
			"remaining_amount":    summary["remaining_amount"],
		}))
	}
	if number(summary["advance_unreceived_total"]) > 1 {
		warnings = append(warnings, receivableWarning("advance_unreceived", "warning", "Có ứng trước đã duyệt nhưng chưa thu đủ tiền", Record{
			"advance_approved_total":   summary["advance_approved_total"],
			"advance_received_total":   summary["advance_received_total"],
			"advance_unreceived_total": summary["advance_unreceived_total"],
		}))
	}
	if number(summary["advance_unsettled_total"]) > 1 {
		warnings = append(warnings, receivableWarning("advance_unsettled", "warning", "Có ứng trước đã thu nhưng chưa quyết toán đủ", Record{
			"advance_received_total":  summary["advance_received_total"],
			"advance_settled_total":   summary["advance_settled_total"],
			"advance_unsettled_total": summary["advance_unsettled_total"],
		}))
	}
	if number(summary["balance_difference"]) > 1 {
		warnings = append(warnings, receivableWarning("balance_difference", "critical", "Số còn phải thu lệch với công thức gross - đã thu", Record{
			"remaining_amount":   summary["remaining_amount"],
			"computed_remaining": summary["computed_remaining"],
			"balance_difference": summary["balance_difference"],
		}))
	}
	if int(number(summary["pending_receipt_count"])) > 0 {
		warnings = append(warnings, receivableWarning("pending_receipt", "info", "Có phiếu thu đang chờ duyệt", Record{
			"pending_receipt_count": summary["pending_receipt_count"],
		}))
	}
	return warnings
}

func receivableWarning(code, severity, message string, details Record) Record {
	return Record{
		"code":     code,
		"severity": severity,
		"message":  message,
		"details":  details,
	}
}

func (r *Repository) ListOutputInvoicesForReceivable(ctx context.Context, receivableID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			oi.*,
			p.name AS project_name,
			rec.receivable_code,
			rec.client_name AS receivable_client_name,
			rec.gross_amount AS receivable_gross_amount,
			rec.remaining_amount AS receivable_remaining_amount,
			pay.payment_code,
			pay.payment_type,
			d.name AS document_name,
			COALESCE(uploaded.full_name, '') AS uploaded_by_name,
			COALESCE(issued.full_name, '') AS issued_by_name,
			COALESCE(voided.full_name, '') AS voided_by_name
		FROM output_invoices oi
		JOIN projects p ON p.id=oi.project_id
		JOIN receivables rec ON rec.id=oi.receivable_id
		LEFT JOIN payments pay ON pay.id=oi.payment_id
		LEFT JOIN documents d ON d.id=oi.document_id
		LEFT JOIN users uploaded ON uploaded.id=oi.uploaded_by
		LEFT JOIN users issued ON issued.id=oi.issued_by
		LEFT JOIN users voided ON voided.id=oi.voided_by
		WHERE oi.receivable_id=$1
		  AND oi.status NOT IN ('deleted','cancelled')
		  AND rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		ORDER BY oi.invoice_date DESC, oi.created_at DESC
	`, receivableID)
}

func (r *Repository) ListOutputInvoicesForProject(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			oi.*,
			p.name AS project_name,
			rec.receivable_code,
			rec.client_name AS receivable_client_name,
			rec.gross_amount AS receivable_gross_amount,
			rec.remaining_amount AS receivable_remaining_amount,
			pay.payment_code,
			pay.payment_type,
			d.name AS document_name,
			COALESCE(uploaded.full_name, '') AS uploaded_by_name,
			COALESCE(issued.full_name, '') AS issued_by_name,
			COALESCE(voided.full_name, '') AS voided_by_name
		FROM output_invoices oi
		JOIN projects p ON p.id=oi.project_id
		JOIN receivables rec ON rec.id=oi.receivable_id
		LEFT JOIN payments pay ON pay.id=oi.payment_id
		LEFT JOIN documents d ON d.id=oi.document_id
		LEFT JOIN users uploaded ON uploaded.id=oi.uploaded_by
		LEFT JOIN users issued ON issued.id=oi.issued_by
		LEFT JOIN users voided ON voided.id=oi.voided_by
		WHERE oi.project_id=$1
		  AND oi.status NOT IN ('deleted','cancelled')
		  AND rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		ORDER BY oi.invoice_date DESC, oi.created_at DESC
	`, projectID)
}

func (r *Repository) GetOutputInvoiceDetail(ctx context.Context, invoiceID string) (Record, error) {
	return getOutputInvoiceDetail(ctx, r.db, invoiceID)
}

func getOutputInvoiceDetail(ctx context.Context, db queryContext, invoiceID string) (Record, error) {
	return queryOneRecord(ctx, db, `
		SELECT
			oi.*,
			p.name AS project_name,
			rec.receivable_code,
			rec.client_name AS receivable_client_name,
			rec.gross_amount AS receivable_gross_amount,
			rec.remaining_amount AS receivable_remaining_amount,
			pay.payment_code,
			pay.payment_type,
			d.name AS document_name,
			COALESCE(uploaded.full_name, '') AS uploaded_by_name,
			COALESCE(issued.full_name, '') AS issued_by_name,
			COALESCE(voided.full_name, '') AS voided_by_name
		FROM output_invoices oi
		JOIN projects p ON p.id=oi.project_id
		JOIN receivables rec ON rec.id=oi.receivable_id
		LEFT JOIN payments pay ON pay.id=oi.payment_id
		LEFT JOIN documents d ON d.id=oi.document_id
		LEFT JOIN users uploaded ON uploaded.id=oi.uploaded_by
		LEFT JOIN users issued ON issued.id=oi.issued_by
		LEFT JOIN users voided ON voided.id=oi.voided_by
		WHERE oi.id=$1
		  AND oi.status NOT IN ('deleted','cancelled')
		  AND rec.status <> 'cancelled'
		  AND p.status <> 'cancelled'
	`, invoiceID)
}

func (r *Repository) CreateOutputInvoice(ctx context.Context, receivable Record, body Record, file StoredFileMetadata, actorID, actorName string) (result Record, err error) {
	tracker := storedMutationTracker{}
	defer tracker.wrap(&err)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	projectID := cleanID(receivable["project_id"])
	receivableID := cleanID(receivable["id"])
	invoiceID := newID()
	documentID := cleanID(body["document_id"])
	if documentID == "" {
		documentID = newID()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO documents (id, project_id, target_type, target_id, name, file_url, document_type, status, uploaded_by)
			VALUES ($1, $2, 'output_invoice', $3, $4, $5, 'output_invoice_pdf', 'active', NULLIF($6,'')::uuid)
		`, documentID, projectID, invoiceID, fmt.Sprintf("Hoá đơn đầu ra %s", fmt.Sprint(body["invoice_number"])), fmt.Sprintf("/api/output-invoices/%s/file", invoiceID), actorID)
		if err != nil {
			return nil, err
		}
	}

	rows, err := queryTx(ctx, tx, `
		INSERT INTO output_invoices (
			id, project_id, receivable_id, payment_id, document_id,
			invoice_number, invoice_symbol, invoice_date, client_partner_id, client_name, client_tax_code,
			amount_before_vat, vat_amount, total_amount, currency, status, notes,
			storage_provider, storage_key, mime_type, file_name, file_size, checksum_sha256,
			uploaded_by, accounting_book_id, tax_code_id, revenue_account_id
		)
		VALUES (
			$1, $2, $3, NULLIF($4,'')::uuid, NULLIF($5,'')::uuid,
			$6, $7, $8::date, NULLIF($9,'')::uuid, $10, $11,
			$12, $13, $14, 'VND', 'draft', $15,
			$16, $17, $18, $19, $20, $21,
			NULLIF($22,'')::uuid,$23::uuid,$24::uuid,$25::uuid
		)
		RETURNING *
	`,
		invoiceID,
		projectID,
		receivableID,
		cleanID(body["payment_id"]),
		documentID,
		fmt.Sprint(body["invoice_number"]),
		fmt.Sprint(body["invoice_symbol"]),
		fmt.Sprint(body["invoice_date"]),
		cleanID(body["client_partner_id"]),
		fmt.Sprint(body["client_name"]),
		fmt.Sprint(body["client_tax_code"]),
		number(body["amount_before_vat"]),
		number(body["vat_amount"]),
		number(body["total_amount"]),
		fmt.Sprint(body["notes"]),
		file.StorageProvider,
		file.StorageKey,
		file.MIMEType,
		fmt.Sprint(body["file_name"]),
		file.FileSize,
		file.ChecksumSHA256,
		actorID,
		body["accounting_book_id"],
		body["tax_code_id"],
		body["revenue_account_id"],
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if err := insertOutputInvoiceTaxLineTx(ctx, tx, rows[0], actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE receivables SET updated_at=now() WHERE id=$1`, receivableID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "output_invoice_uploaded", "output_invoice", invoiceID, "Tải PDF hóa đơn đầu ra", Record{
		"receivable_id":   receivableID,
		"document_id":     documentID,
		"payment_id":      cleanID(body["payment_id"]),
		"invoice_number":  body["invoice_number"],
		"total_amount":    body["total_amount"],
		"checksum_sha256": file.ChecksumSHA256,
	}); err != nil {
		return nil, err
	}
	return finalizeStoredMutation(&tracker, func() (Record, error) {
		return getOutputInvoiceDetail(ctx, tx, invoiceID)
	}, tx.Commit)
}

func (r *Repository) OutputInvoiceAction(ctx context.Context, invoiceID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM output_invoices WHERE id=$1 AND status NOT IN ('deleted','cancelled') FOR UPDATE`, invoiceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	if err := ensureVATSourcePeriodMutableTx(ctx, tx, cleanID(current["accounting_book_id"]), current["invoice_date"]); err != nil {
		return nil, err
	}
	fromStatus := fmt.Sprint(current["status"])
	toStatus := ""
	event := ""
	var updated []Record
	switch action {
	case "issue":
		if fromStatus != "draft" {
			return nil, validationError("Trạng thái hoá đơn không cho phép phát hành", Record{"status": fromStatus})
		}
		toStatus = "issued"
		event = "output_invoice_issued"
		updated, err = queryTx(ctx, tx, `
			UPDATE output_invoices
			SET status='issued',
				issued_by=NULLIF($2,'')::uuid,
				issued_at=now(),
				voided_by=NULL,
				voided_at=NULL,
				updated_at=now()
			WHERE id=$1 AND status=$3
			RETURNING *
		`, invoiceID, actorID, fromStatus)
	case "void":
		if !statusIn(fromStatus, "draft", "issued") {
			return nil, validationError("Trạng thái hoá đơn không cho phép huỷ", Record{"status": fromStatus})
		}
		if fromStatus == "issued" && actionComment(req) == "" {
			return nil, validationError("Lý do hủy hóa đơn đã ghi sổ là bắt buộc")
		}
		toStatus = "voided"
		event = "output_invoice_voided"
		updated, err = queryTx(ctx, tx, `
			UPDATE output_invoices
			SET status='voided',
				voided_by=NULLIF($2,'')::uuid,
				voided_at=now(),
				updated_at=now()
			WHERE id=$1 AND status=$3
			RETURNING *
		`, invoiceID, actorID, fromStatus)
	default:
		return nil, validationError("Hành động hoá đơn đầu ra không hợp lệ", Record{"action": action})
	}
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái hoá đơn đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	var journalEntry Record
	if action == "issue" {
		journalEntry, err = r.postOutputInvoiceTx(ctx, tx, updated[0], actorID)
		if err != nil {
			return nil, err
		}
	}
	if action == "void" && fromStatus == "issued" {
		journalEntry, err = r.reverseSourceJournalTx(
			ctx, tx, cleanID(current["project_id"]), "output_invoice", invoiceID,
			"output_invoice_issued", "output_invoice_voided", actionComment(req), actorID,
		)
		if err != nil {
			return nil, err
		}
	}
	comment := actionComment(req)
	metadata := Record{"action": action, "from_status": fromStatus, "to_status": toStatus, "receivable_id": current["receivable_id"]}
	if comment != "" {
		metadata["comment"] = comment
	}
	if journalEntry != nil {
		metadata["journal_entry_id"] = journalEntry["id"]
		metadata["journal_entry_no"] = journalEntry["entry_no"]
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(current["project_id"]), actorID, actorName, event, "output_invoice", invoiceID, fmt.Sprintf("Cập nhật hoá đơn đầu ra từ %s sang %s", fromStatus, toStatus), metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetOutputInvoiceDetail(ctx, invoiceID)
}

func (r *Repository) ReceivableAction(ctx context.Context, id, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM receivables WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	toStatus := ""
	event := ""
	metadata := Record{"action": action, "from_status": fromStatus}
	var updated []Record
	switch action {
	case "approve":
		if !statusIn(fromStatus, "draft", "submitted", "recorded", "overdue") {
			return nil, validationError("Trạng thái phải thu không cho phép duyệt", Record{"status": fromStatus})
		}
		toStatus = "approved"
		event = "receivable_approved"
		updated, err = queryTx(ctx, tx, `
			UPDATE receivables
			SET status='approved', approved_by=NULLIF($2,'')::uuid, updated_at=now()
			WHERE id=$1 AND status=$3
			RETURNING *
		`, id, actorID, fromStatus)
	case "receive":
		return nil, validationError("Không được tất toán trực tiếp khoản phải thu; hãy lập và duyệt Phiếu thu để bảo toàn phân bổ công nợ, dòng tiền và bút toán")
	default:
		return nil, validationError("Hành động phải thu không hợp lệ", Record{"action": action})
	}
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái phải thu đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	comment := actionComment(req)
	metadata["to_status"] = toStatus
	if comment != "" {
		metadata["comment"] = comment
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(current["project_id"]), actorID, actorName, event, "receivable", id, fmt.Sprintf("Cập nhật phải thu từ %s sang %s", fromStatus, toStatus), metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated[0], nil
}

func (r *Repository) PaymentAction(ctx context.Context, id, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	if action == "reverse" {
		return r.ReverseApprovedPayment(ctx, id, req, actorID, actorName)
	}
	if action != "approve" {
		return nil, validationError("Hành động phiếu thanh toán không hợp lệ", Record{"action": action})
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM payments WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	if !statusIn(fromStatus, "draft", "submitted", "pending", "waiting_approval") {
		return nil, validationError("Trạng thái phiếu thanh toán không cho phép duyệt", Record{"status": fromStatus})
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE payments
		SET status='approved',
			approved_by=NULLIF($2,'')::uuid,
			paid_at=COALESCE(paid_at, now()),
			updated_at=now()
		WHERE id=$1 AND status=$3
		RETURNING *
	`, id, actorID, fromStatus)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái phiếu thanh toán đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	if err := r.applyPaymentAllocationsTx(ctx, tx, updated[0], actorID, actorName); err != nil {
		return nil, err
	}
	if err := r.applyReceiptAllocationsTx(ctx, tx, updated[0], actorID, actorName); err != nil {
		return nil, err
	}
	journalEntry, err := r.postPaymentTx(ctx, tx, updated[0], actorID)
	if err != nil {
		return nil, err
	}
	comment := actionComment(req)
	metadata := Record{"action": action, "from_status": fromStatus, "to_status": "approved"}
	if comment != "" {
		metadata["comment"] = comment
	}
	if journalEntry != nil {
		metadata["journal_entry_id"] = journalEntry["id"]
		metadata["journal_entry_no"] = journalEntry["entry_no"]
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(current["project_id"]), actorID, actorName, "payment_approved", "payment", id, "Duyệt phiếu thanh toán", metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated[0], nil
}

func (r *Repository) applyPaymentAllocationsTx(ctx context.Context, tx *sql.Tx, payment Record, actorID, actorName string) error {
	allocations, err := queryTx(ctx, tx, `SELECT * FROM payment_allocations WHERE payment_id=$1 FOR UPDATE`, payment["id"])
	if err != nil {
		return err
	}
	for _, allocation := range allocations {
		amount := number(allocation["amount"])
		if amount <= 0 {
			return validationError("Phân bổ thanh toán không hợp lệ", Record{"allocation_id": allocation["id"]})
		}
		projectID := fmt.Sprint(allocation["project_id"])
		payableID := cleanID(allocation["payable_id"])
		switch fmt.Sprint(allocation["allocation_type"]) {
		case "payable_payment":
			payableRows, err := queryTx(ctx, tx, `SELECT * FROM payables WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, payableID)
			if err != nil {
				return err
			}
			if len(payableRows) == 0 {
				return sql.ErrNoRows
			}
			payable := payableRows[0]
			if amount > number(payable["remaining_amount"]) {
				return validationError("Số tiền thanh toán vượt số còn phải trả", Record{"amount": amount, "remaining_amount": payable["remaining_amount"]})
			}
			scheduleID := cleanID(allocation["schedule_id"])
			if scheduleID != "" {
				scheduleRows, err := queryTx(ctx, tx, `SELECT * FROM payable_payment_schedules WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, scheduleID)
				if err != nil {
					return err
				}
				if len(scheduleRows) == 0 {
					return sql.ErrNoRows
				}
				schedule := scheduleRows[0]
				if cleanID(schedule["payable_id"]) != payableID {
					return validationError("Kỳ thanh toán không thuộc phải trả", Record{"schedule_id": scheduleID})
				}
				if amount > number(schedule["remaining_amount"]) {
					return validationError("Số tiền thanh toán vượt số còn lại của kỳ", Record{"amount": amount, "schedule_remaining": schedule["remaining_amount"]})
				}
				scheduleRemaining := number(schedule["remaining_amount"]) - amount
				if _, err := tx.ExecContext(ctx, `
					UPDATE payable_payment_schedules
					SET paid_amount=paid_amount + $2,
						remaining_amount=GREATEST(remaining_amount - $2, 0),
						status=$3,
						updated_at=now()
					WHERE id=$1
				`, scheduleID, amount, scheduleSettlementStatus(scheduleRemaining)); err != nil {
					return err
				}
			}
			payableRemaining := number(payable["remaining_amount"]) - amount
			applied := number(payable["paid_amount"]) + number(payable["advance_amount"]) + amount
			if _, err := tx.ExecContext(ctx, `
				UPDATE payables
				SET paid_amount=paid_amount + $2,
					remaining_amount=GREATEST(remaining_amount - $2, 0),
					status=$3,
					approved_by=COALESCE(approved_by,NULLIF($4,'')::uuid),
					updated_at=now()
				WHERE id=$1
			`, payableID, amount, payableSettlementStatus(applied, payableRemaining), actorID); err != nil {
				return err
			}
			if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "payable_payment_allocated", "payment", cleanID(payment["id"]), "Ghi nhận thanh toán từng phần đầu vào", Record{
				"payable_id":       payableID,
				"schedule_id":      scheduleID,
				"input_invoice_id": cleanID(allocation["input_invoice_id"]),
				"amount":           amount,
			}); err != nil {
				return err
			}
		case "advance_payment":
			advanceID := cleanID(allocation["advance_request_id"])
			if advanceID == "" {
				return validationError("Thanh toán ứng trước thiếu advance_request_id", Record{"allocation_id": allocation["id"]})
			}
			advanceRows, err := queryTx(ctx, tx, `SELECT * FROM advance_requests WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, advanceID)
			if err != nil {
				return err
			}
			if len(advanceRows) == 0 {
				return sql.ErrNoRows
			}
			advance := advanceRows[0]
			if cleanID(advance["payable_id"]) != payableID {
				return validationError("Ứng trước không thuộc phải trả", Record{"advance_request_id": advanceID})
			}
			if amount > number(advance["amount"])-number(advance["paid_amount"]) {
				return validationError("Số tiền chi ứng trước vượt phần chưa chi", Record{"amount": amount})
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE advance_requests
				SET paid_amount=paid_amount + $2,
					payment_id=COALESCE(payment_id, $3),
					paid_at=COALESCE(paid_at, $4::timestamptz, now()),
					updated_at=now()
				WHERE id=$1
			`, advanceID, amount, payment["id"], payment["paid_at"]); err != nil {
				return err
			}
			if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "payable_advance_payment_recorded", "advance_request", advanceID, "Ghi nhận chi ứng trước đầu vào", Record{
				"payable_id": payableID,
				"payment_id": cleanID(payment["id"]),
				"amount":     amount,
			}); err != nil {
				return err
			}
		case "advance_settlement":
			continue
		default:
			return validationError("Loại phân bổ thanh toán không hợp lệ", Record{"allocation_type": allocation["allocation_type"]})
		}
	}
	return nil
}

func (r *Repository) applyReceiptAllocationsTx(ctx context.Context, tx *sql.Tx, payment Record, actorID, actorName string) error {
	allocations, err := queryTx(ctx, tx, `SELECT * FROM receipt_allocations WHERE payment_id=$1 FOR UPDATE`, payment["id"])
	if err != nil {
		return err
	}
	for _, allocation := range allocations {
		amount := number(allocation["amount"])
		if amount <= 0 {
			return validationError("Phân bổ phiếu thu không hợp lệ", Record{"allocation_id": allocation["id"]})
		}
		projectID := fmt.Sprint(allocation["project_id"])
		receivableID := cleanID(allocation["receivable_id"])
		switch fmt.Sprint(allocation["allocation_type"]) {
		case "receivable_receipt":
			receivableRows, err := queryTx(ctx, tx, `SELECT * FROM receivables WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, receivableID)
			if err != nil {
				return err
			}
			if len(receivableRows) == 0 {
				return sql.ErrNoRows
			}
			receivable := receivableRows[0]
			if amount > number(receivable["remaining_amount"]) {
				return validationError("Số tiền thu vượt số còn phải thu", Record{"amount": amount, "remaining_amount": receivable["remaining_amount"]})
			}
			scheduleID := cleanID(allocation["schedule_id"])
			if scheduleID != "" {
				scheduleRows, err := queryTx(ctx, tx, `SELECT * FROM receivable_payment_schedules WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, scheduleID)
				if err != nil {
					return err
				}
				if len(scheduleRows) == 0 {
					return sql.ErrNoRows
				}
				schedule := scheduleRows[0]
				if cleanID(schedule["receivable_id"]) != receivableID {
					return validationError("Kỳ thu không thuộc phải thu", Record{"schedule_id": scheduleID})
				}
				if amount > number(schedule["remaining_amount"]) {
					return validationError("Số tiền thu vượt số còn lại của kỳ", Record{"amount": amount, "schedule_remaining": schedule["remaining_amount"]})
				}
				scheduleRemaining := number(schedule["remaining_amount"]) - amount
				if _, err := tx.ExecContext(ctx, `
					UPDATE receivable_payment_schedules
					SET received_amount=received_amount + $2,
						remaining_amount=GREATEST(remaining_amount - $2, 0),
						status=$3,
						updated_at=now()
					WHERE id=$1
				`, scheduleID, amount, receiptScheduleSettlementStatus(scheduleRemaining)); err != nil {
					return err
				}
			}
			receivableRemaining := number(receivable["remaining_amount"]) - amount
			received := number(receivable["received_amount"]) + amount
			if _, err := tx.ExecContext(ctx, `
				UPDATE receivables
				SET received_amount=received_amount + $2,
					remaining_amount=GREATEST(remaining_amount - $2, 0),
					status=$3,
					approved_by=COALESCE(approved_by,NULLIF($4,'')::uuid),
					updated_at=now()
				WHERE id=$1
			`, receivableID, amount, receivableSettlementStatus(received, receivableRemaining), actorID); err != nil {
				return err
			}
			if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "receivable_payment_allocated", "payment", cleanID(payment["id"]), "Ghi nhận thu từng phần đầu ra", Record{
				"receivable_id":     receivableID,
				"schedule_id":       scheduleID,
				"output_invoice_id": cleanID(allocation["output_invoice_id"]),
				"amount":            amount,
			}); err != nil {
				return err
			}
		case "advance_receipt":
			advanceID := cleanID(allocation["advance_receipt_id"])
			if advanceID == "" {
				return validationError("Thu ứng trước thiếu advance_receipt_id", Record{"allocation_id": allocation["id"]})
			}
			advanceRows, err := queryTx(ctx, tx, `SELECT * FROM client_advance_receipts WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, advanceID)
			if err != nil {
				return err
			}
			if len(advanceRows) == 0 {
				return sql.ErrNoRows
			}
			advance := advanceRows[0]
			if cleanID(advance["receivable_id"]) != receivableID {
				return validationError("Ứng trước không thuộc phải thu", Record{"advance_receipt_id": advanceID})
			}
			if amount > number(advance["amount"])-number(advance["received_amount"]) {
				return validationError("Số tiền thu ứng trước vượt phần chưa ghi nhận tiền", Record{"amount": amount})
			}
			newReceived := number(advance["received_amount"]) + amount
			toStatus := clientAdvanceReceiptStatusAfterReceipt(number(advance["amount"]), newReceived, number(advance["settled_amount"]))
			if _, err := tx.ExecContext(ctx, `
				UPDATE client_advance_receipts
				SET received_amount=received_amount + $2,
					payment_id=COALESCE(payment_id, $3),
					received_at=COALESCE(received_at, $4::timestamptz, now()),
					status=$5,
					updated_at=now()
				WHERE id=$1
			`, advanceID, amount, payment["id"], payment["paid_at"], toStatus); err != nil {
				return err
			}
			if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "receivable_advance_payment_recorded", "client_advance_receipt", advanceID, "Ghi nhận thu ứng trước đầu ra", Record{
				"receivable_id": receivableID,
				"payment_id":    cleanID(payment["id"]),
				"amount":        amount,
			}); err != nil {
				return err
			}
		case "advance_settlement":
			continue
		default:
			return validationError("Loại phân bổ phiếu thu không hợp lệ", Record{"allocation_type": allocation["allocation_type"]})
		}
	}
	return nil
}
