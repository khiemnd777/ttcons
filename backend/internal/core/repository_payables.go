package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (r *Repository) PayableAction(ctx context.Context, id, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM payables WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, id)
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
			return nil, validationError("Trạng thái phải trả không cho phép duyệt", Record{"status": fromStatus})
		}
		toStatus = "approved"
		event = "payable_approved"
		updated, err = queryTx(ctx, tx, `
			UPDATE payables
			SET status='approved', approved_by=NULLIF($2,'')::uuid, updated_at=now()
			WHERE id=$1 AND status=$3
			RETURNING *
		`, id, actorID, fromStatus)
	case "pay":
		return nil, validationError("Không được tất toán trực tiếp khoản phải trả; hãy lập và duyệt Phiếu chi để bảo toàn phân bổ công nợ, dòng tiền và bút toán")
	default:
		return nil, validationError("Hành động phải trả không hợp lệ", Record{"action": action})
	}
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái phải trả đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	if action == "approve" {
		if err := r.recordPayableCommitmentTx(ctx, tx, updated[0], actorID); err != nil {
			return nil, err
		}
	}
	comment := actionComment(req)
	metadata["to_status"] = toStatus
	if comment != "" {
		metadata["comment"] = comment
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(current["project_id"]), actorID, actorName, event, "payable", id, fmt.Sprintf("Cập nhật phải trả từ %s sang %s", fromStatus, toStatus), metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated[0], nil
}

func (r *Repository) GetPayableDetail(ctx context.Context, payableID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT pay.*, p.name AS project_name
		FROM payables pay
		JOIN projects p ON p.id=pay.project_id
		WHERE pay.id=$1
		  AND pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
	`, payableID)
}

func (r *Repository) PayableReconciliation(ctx context.Context, payableID string) (Record, error) {
	summary, err := r.QueryOne(ctx, `
		WITH pay AS (
			SELECT pay.*, p.name AS project_name
			FROM payables pay
			JOIN projects p ON p.id=pay.project_id
			WHERE pay.id=$1
			  AND pay.status <> 'cancelled'
			  AND p.status <> 'cancelled'
		),
		inv AS (
			SELECT
				payable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('deleted','cancelled')) AS invoice_count,
				COUNT(*) FILTER (WHERE status='verified') AS verified_invoice_count,
				COUNT(*) FILTER (WHERE status='rejected') AS rejected_invoice_count,
				COALESCE(SUM(total_amount) FILTER (WHERE status NOT IN ('deleted','cancelled')), 0) AS invoice_total,
				COALESCE(SUM(total_amount) FILTER (WHERE status='verified'), 0) AS verified_invoice_total,
				MAX(invoice_date) FILTER (WHERE status NOT IN ('deleted','cancelled')) AS latest_invoice_date
			FROM input_invoices
			WHERE payable_id=$1
			GROUP BY payable_id
		),
		sch AS (
			SELECT
				payable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('cancelled','deleted')) AS schedule_count,
				COUNT(*) FILTER (WHERE status IN ('approved','partially_paid','overdue') AND remaining_amount > 0 AND due_date < CURRENT_DATE) AS overdue_schedule_count,
				COALESCE(SUM(planned_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS scheduled_total,
				COALESCE(SUM(paid_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS scheduled_paid_total,
				COALESCE(SUM(remaining_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS scheduled_remaining,
				MIN(due_date) FILTER (WHERE status IN ('approved','partially_paid','overdue') AND remaining_amount > 0) AS next_schedule_due_date
			FROM payable_payment_schedules
			WHERE payable_id=$1
			GROUP BY payable_id
		),
		adv AS (
			SELECT
				payable_id,
				COUNT(*) FILTER (WHERE status NOT IN ('cancelled','deleted')) AS advance_request_count,
				COALESCE(SUM(amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS advance_requested_total,
				COALESCE(SUM(amount) FILTER (WHERE status IN ('approved','partially_settled','settled')), 0) AS advance_approved_total,
				COALESCE(SUM(paid_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS advance_paid_total,
				COALESCE(SUM(settled_amount) FILTER (WHERE status NOT IN ('cancelled','deleted')), 0) AS advance_settled_total
			FROM advance_requests
			WHERE payable_id=$1
			GROUP BY payable_id
		),
		alloc AS (
			SELECT
				pa.payable_id,
				COALESCE(SUM(pa.amount) FILTER (WHERE pmt.status='approved' AND pa.allocation_type='payable_payment'), 0) AS approved_payable_payment_total,
				COALESCE(SUM(pa.amount) FILTER (WHERE pmt.status='approved' AND pa.allocation_type='advance_payment'), 0) AS approved_advance_payment_total,
				COALESCE(SUM(pa.amount) FILTER (WHERE pmt.status <> 'cancelled'), 0) AS payment_allocation_total,
				COUNT(*) FILTER (WHERE pmt.status NOT IN ('approved','cancelled')) AS pending_payment_count
			FROM payment_allocations pa
			JOIN payments pmt ON pmt.id=pa.payment_id
			WHERE pa.payable_id=$1
			GROUP BY pa.payable_id
		),
		calc AS (
			SELECT
				pay.*,
				COALESCE(inv.invoice_count, 0) AS invoice_count,
				COALESCE(inv.verified_invoice_count, 0) AS verified_invoice_count,
				COALESCE(inv.rejected_invoice_count, 0) AS rejected_invoice_count,
				COALESCE(inv.invoice_total, 0) AS invoice_total,
				COALESCE(inv.verified_invoice_total, 0) AS verified_invoice_total,
				inv.latest_invoice_date,
				COALESCE(sch.schedule_count, 0) AS schedule_count,
				COALESCE(sch.overdue_schedule_count, 0) AS overdue_schedule_count,
				COALESCE(sch.scheduled_total, 0) AS scheduled_total,
				COALESCE(sch.scheduled_paid_total, 0) AS scheduled_paid_total,
				COALESCE(sch.scheduled_remaining, 0) AS scheduled_remaining,
				sch.next_schedule_due_date,
				COALESCE(adv.advance_request_count, 0) AS advance_request_count,
				COALESCE(adv.advance_requested_total, 0) AS advance_requested_total,
				COALESCE(adv.advance_approved_total, 0) AS advance_approved_total,
				COALESCE(adv.advance_paid_total, 0) AS advance_paid_total,
				COALESCE(adv.advance_settled_total, 0) AS advance_settled_total,
				COALESCE(alloc.approved_payable_payment_total, 0) AS approved_payable_payment_total,
				COALESCE(alloc.approved_advance_payment_total, 0) AS approved_advance_payment_total,
				COALESCE(alloc.payment_allocation_total, 0) AS payment_allocation_total,
				COALESCE(alloc.pending_payment_count, 0) AS pending_payment_count,
				COALESCE(pay.due_date, sch.next_schedule_due_date, inv.latest_invoice_date) AS effective_due_date,
				GREATEST(COALESCE(pay.gross_amount,0) - COALESCE(pay.advance_amount,0) - COALESCE(pay.paid_amount,0) - COALESCE(pay.deduction_amount,0), 0) AS computed_remaining
			FROM pay
			LEFT JOIN inv ON inv.payable_id=pay.id
			LEFT JOIN sch ON sch.payable_id=pay.id
			LEFT JOIN adv ON adv.payable_id=pay.id
			LEFT JOIN alloc ON alloc.payable_id=pay.id
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
			GREATEST(COALESCE(gross_amount,0) - COALESCE(verified_invoice_total,0), 0) AS unverified_invoice_gap,
			GREATEST(COALESCE(advance_approved_total,0) - COALESCE(advance_paid_total,0), 0) AS advance_unpaid_total,
			GREATEST(COALESCE(advance_paid_total,0) - COALESCE(advance_settled_total,0), 0) AS advance_unsettled_total
		FROM calc
	`, payableID)
	if err != nil {
		return nil, err
	}
	warnings := payableReconciliationWarnings(summary)
	summary["warning_count"] = len(warnings)
	if len(warnings) > 0 {
		summary["reconciliation_status"] = "needs_review"
	} else {
		summary["reconciliation_status"] = "ok"
	}

	ledger, err := r.Query(ctx, `
		WITH entries AS (
			SELECT
				pay.created_at AS occurred_at,
				0 AS sort_order,
				'payable_recorded'::text AS entry_type,
				pay.id::text AS source_id,
				pay.payable_code::text AS source_code,
				pay.status::text AS status,
				'Ghi nhận phải trả'::text AS description,
				COALESCE(pay.gross_amount,0)::numeric AS debit_amount,
				0::numeric AS credit_amount,
				0::numeric AS document_amount,
				0::numeric AS cash_amount,
				COALESCE(created.full_name, '') AS actor_name
			FROM payables pay
			LEFT JOIN users created ON created.id=pay.created_by
			WHERE pay.id=$1 AND pay.status <> 'cancelled'

			UNION ALL

			SELECT
				COALESCE(ii.verified_at, ii.rejected_at, ii.created_at) AS occurred_at,
				10 AS sort_order,
				('input_invoice_' || ii.status)::text AS entry_type,
				ii.id::text AS source_id,
				ii.invoice_number::text AS source_code,
				ii.status::text AS status,
				CASE
					WHEN ii.status='verified' THEN 'Hoá đơn đầu vào đã xác thực'
					WHEN ii.status='rejected' THEN 'Hoá đơn đầu vào bị từ chối'
					ELSE 'Tải lên hoá đơn đầu vào'
				END AS description,
				0::numeric AS debit_amount,
				0::numeric AS credit_amount,
				COALESCE(ii.total_amount,0)::numeric AS document_amount,
				0::numeric AS cash_amount,
				COALESCE(actor.full_name, '') AS actor_name
			FROM input_invoices ii
			LEFT JOIN users actor ON actor.id=COALESCE(ii.verified_by, ii.rejected_by, ii.uploaded_by)
			WHERE ii.payable_id=$1 AND ii.status NOT IN ('deleted','cancelled')

			UNION ALL

			SELECT
				ar.updated_at AS occurred_at,
				20 AS sort_order,
				'advance_approved'::text AS entry_type,
				ar.id::text AS source_id,
				ar.request_code::text AS source_code,
				ar.status::text AS status,
				'Ứng trước được duyệt ghi giảm phải trả'::text AS description,
				0::numeric AS debit_amount,
				COALESCE(ar.amount,0)::numeric AS credit_amount,
				0::numeric AS document_amount,
				0::numeric AS cash_amount,
				COALESCE(approved.full_name, '') AS actor_name
			FROM advance_requests ar
			LEFT JOIN users approved ON approved.id=ar.approved_by
			WHERE ar.payable_id=$1 AND ar.status IN ('approved','partially_settled','settled')

			UNION ALL

			SELECT
				COALESCE(pmt.paid_at, pmt.updated_at, pmt.created_at) AS occurred_at,
				30 AS sort_order,
				('payment_' || pa.allocation_type)::text AS entry_type,
				pmt.id::text AS source_id,
				pmt.payment_code::text AS source_code,
				pmt.status::text AS status,
				CASE
					WHEN pa.allocation_type='advance_payment' THEN 'Chi tiền ứng trước'
					WHEN pa.allocation_type='advance_settlement' THEN 'Quyết toán ứng trước'
					ELSE 'Thanh toán phải trả'
				END AS description,
				0::numeric AS debit_amount,
				CASE WHEN pa.allocation_type='payable_payment' THEN COALESCE(pa.amount,0)::numeric ELSE 0::numeric END AS credit_amount,
				0::numeric AS document_amount,
				COALESCE(pa.amount,0)::numeric AS cash_amount,
				COALESCE(approved.full_name, created.full_name, '') AS actor_name
			FROM payment_allocations pa
			JOIN payments pmt ON pmt.id=pa.payment_id
			LEFT JOIN users approved ON approved.id=pmt.approved_by
			LEFT JOIN users created ON created.id=pmt.created_by
			WHERE pa.payable_id=$1 AND pmt.status='approved'

			UNION ALL

			SELECT
				COALESCE(ar.settled_at, ar.updated_at) AS occurred_at,
				40 AS sort_order,
				'advance_settled'::text AS entry_type,
				ar.id::text AS source_id,
				ar.request_code::text AS source_code,
				ar.status::text AS status,
				'Quyết toán ứng trước bằng chứng từ'::text AS description,
				0::numeric AS debit_amount,
				0::numeric AS credit_amount,
				COALESCE(ar.settled_amount,0)::numeric AS document_amount,
				0::numeric AS cash_amount,
				''::text AS actor_name
			FROM advance_requests ar
			WHERE ar.payable_id=$1
			  AND ar.status NOT IN ('cancelled','deleted')
			  AND COALESCE(ar.settled_amount,0) > 0
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
	`, payableID)
	if err != nil {
		return nil, err
	}

	return Record{
		"summary":  summary,
		"warnings": warnings,
		"ledger":   ledger,
	}, nil
}

func payableReconciliationWarnings(summary Record) []Record {
	warnings := []Record{}
	if number(summary["remaining_amount"]) > 0 && int(number(summary["overdue_days"])) > 0 {
		severity := "warning"
		if number(summary["overdue_days"]) > 60 {
			severity = "critical"
		}
		warnings = append(warnings, payableWarning("overdue", severity, "Khoản phải trả đã quá hạn", Record{
			"effective_due_date": summary["effective_due_date"],
			"overdue_days":       summary["overdue_days"],
			"remaining_amount":   summary["remaining_amount"],
		}))
	}
	if number(summary["verified_invoice_total"]) > number(summary["gross_amount"])+1 {
		warnings = append(warnings, payableWarning("invoice_over_gross", "critical", "Tổng hoá đơn verified vượt giá trị phải trả", Record{
			"gross_amount":           summary["gross_amount"],
			"verified_invoice_total": summary["verified_invoice_total"],
		}))
	}
	if number(summary["paid_amount"]) > number(summary["verified_invoice_total"])+1 {
		warnings = append(warnings, payableWarning("paid_without_verified_invoice", "warning", "Đã thanh toán nhiều hơn tổng hoá đơn verified", Record{
			"paid_amount":            summary["paid_amount"],
			"verified_invoice_total": summary["verified_invoice_total"],
		}))
	}
	if number(summary["remaining_amount"]) > 0 && int(number(summary["schedule_count"])) == 0 {
		warnings = append(warnings, payableWarning("missing_schedule", "warning", "Khoản phải trả còn dư nhưng chưa có lịch thanh toán", Record{
			"remaining_amount": summary["remaining_amount"],
		}))
	}
	if number(summary["scheduled_remaining"]) > number(summary["remaining_amount"])+1 {
		warnings = append(warnings, payableWarning("schedule_over_remaining", "warning", "Số còn lại theo lịch lớn hơn số còn phải trả", Record{
			"scheduled_remaining": summary["scheduled_remaining"],
			"remaining_amount":    summary["remaining_amount"],
		}))
	}
	if number(summary["advance_unpaid_total"]) > 1 {
		warnings = append(warnings, payableWarning("advance_unpaid", "warning", "Có ứng trước đã duyệt nhưng chưa chi đủ", Record{
			"advance_approved_total": summary["advance_approved_total"],
			"advance_paid_total":     summary["advance_paid_total"],
			"advance_unpaid_total":   summary["advance_unpaid_total"],
		}))
	}
	if number(summary["advance_unsettled_total"]) > 1 {
		warnings = append(warnings, payableWarning("advance_unsettled", "warning", "Có ứng trước đã chi nhưng chưa quyết toán đủ", Record{
			"advance_paid_total":      summary["advance_paid_total"],
			"advance_settled_total":   summary["advance_settled_total"],
			"advance_unsettled_total": summary["advance_unsettled_total"],
		}))
	}
	if number(summary["balance_difference"]) > 1 {
		warnings = append(warnings, payableWarning("balance_difference", "critical", "Số còn phải trả lệch với công thức gross - ứng trước - đã trả - khấu trừ", Record{
			"remaining_amount":   summary["remaining_amount"],
			"computed_remaining": summary["computed_remaining"],
			"balance_difference": summary["balance_difference"],
		}))
	}
	if int(number(summary["pending_payment_count"])) > 0 {
		warnings = append(warnings, payableWarning("pending_payment", "info", "Có phiếu thanh toán đang chờ duyệt", Record{
			"pending_payment_count": summary["pending_payment_count"],
		}))
	}
	return warnings
}

func payableWarning(code, severity, message string, details Record) Record {
	return Record{
		"code":     code,
		"severity": severity,
		"message":  message,
		"details":  details,
	}
}

func (r *Repository) ListPayablePaymentSchedules(ctx context.Context, payableID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			pps.*,
			p.name AS project_name,
			pay.payable_code,
			pay.vendor_name,
			COALESCE(created.full_name, '') AS created_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM payable_payment_schedules pps
		JOIN projects p ON p.id=pps.project_id
		JOIN payables pay ON pay.id=pps.payable_id
		LEFT JOIN users created ON created.id=pps.created_by
		LEFT JOIN users approved ON approved.id=pps.approved_by
		WHERE pps.payable_id=$1
		  AND pps.status NOT IN ('cancelled','deleted')
		  AND pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		ORDER BY pps.due_date NULLS LAST, pps.created_at DESC
	`, payableID)
}

func (r *Repository) GetPayablePaymentScheduleDetail(ctx context.Context, scheduleID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			pps.*,
			p.name AS project_name,
			pay.payable_code,
			pay.vendor_name,
			COALESCE(created.full_name, '') AS created_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM payable_payment_schedules pps
		JOIN projects p ON p.id=pps.project_id
		JOIN payables pay ON pay.id=pps.payable_id
		LEFT JOIN users created ON created.id=pps.created_by
		LEFT JOIN users approved ON approved.id=pps.approved_by
		WHERE pps.id=$1
		  AND pps.status NOT IN ('cancelled','deleted')
		  AND pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
	`, scheduleID)
}

func (r *Repository) CreatePayablePaymentSchedule(ctx context.Context, payable Record, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	scheduleID := newID()
	projectID := cleanID(payable["project_id"])
	payableID := cleanID(payable["id"])
	rows, err := queryTx(ctx, tx, `
		INSERT INTO payable_payment_schedules (
			id, project_id, payable_id, schedule_code, milestone_name, schedule_type,
			planned_amount, paid_amount, remaining_amount, due_date, status, notes, created_by
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,0,$7,NULLIF($8,'')::date,'draft',$9,NULLIF($10,'')::uuid)
		RETURNING *
	`, scheduleID, projectID, payableID, body["schedule_code"], body["milestone_name"], body["schedule_type"], number(body["planned_amount"]), fmt.Sprint(body["due_date"]), body["notes"], actorID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "payable_payment_schedule_created", "payable_payment_schedule", scheduleID, "Tạo kỳ thanh toán đầu vào", Record{
		"payable_id":     payableID,
		"schedule_code":  body["schedule_code"],
		"planned_amount": body["planned_amount"],
		"schedule_type":  body["schedule_type"],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPayablePaymentScheduleDetail(ctx, scheduleID)
}

func (r *Repository) PayablePaymentScheduleAction(ctx context.Context, scheduleID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	if action != "approve" {
		return nil, validationError("Hành động kỳ thanh toán không hợp lệ", Record{"action": action})
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM payable_payment_schedules WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, scheduleID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	if !statusIn(fromStatus, "draft", "overdue") {
		return nil, validationError("Trạng thái kỳ thanh toán không cho phép duyệt", Record{"status": fromStatus})
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE payable_payment_schedules
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
		return nil, validationError("Trạng thái kỳ thanh toán đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	comment := actionComment(req)
	metadata := Record{"action": action, "from_status": fromStatus, "to_status": "approved", "payable_id": current["payable_id"]}
	if comment != "" {
		metadata["comment"] = comment
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(current["project_id"]), actorID, actorName, "payable_payment_schedule_approved", "payable_payment_schedule", scheduleID, "Duyệt kỳ thanh toán đầu vào", metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPayablePaymentScheduleDetail(ctx, scheduleID)
}

func (r *Repository) ListPayableAdvanceRequests(ctx context.Context, payableID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			ar.*,
			p.name AS project_name,
			pay.payable_code,
			pay.vendor_type,
			pay.vendor_name AS payable_vendor_name,
			pmt.payment_code,
			pmt.payment_type,
			COALESCE(requested.full_name, '') AS requested_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM advance_requests ar
		JOIN projects p ON p.id=ar.project_id
		JOIN payables pay ON pay.id=ar.payable_id
		LEFT JOIN payments pmt ON pmt.id=ar.payment_id
		LEFT JOIN users requested ON requested.id=ar.requested_by
		LEFT JOIN users approved ON approved.id=ar.approved_by
		WHERE ar.payable_id=$1
		  AND ar.status NOT IN ('cancelled','deleted')
		  AND pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		ORDER BY ar.due_date NULLS LAST, ar.created_at DESC
	`, payableID)
}

func (r *Repository) GetPayableAdvanceRequestDetail(ctx context.Context, advanceID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			ar.*,
			p.name AS project_name,
			pay.payable_code,
			pay.vendor_type,
			pay.vendor_name AS payable_vendor_name,
			pmt.payment_code,
			pmt.payment_type,
			COALESCE(requested.full_name, '') AS requested_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM advance_requests ar
		JOIN projects p ON p.id=ar.project_id
		JOIN payables pay ON pay.id=ar.payable_id
		LEFT JOIN payments pmt ON pmt.id=ar.payment_id
		LEFT JOIN users requested ON requested.id=ar.requested_by
		LEFT JOIN users approved ON approved.id=ar.approved_by
		WHERE ar.id=$1
		  AND ar.status NOT IN ('cancelled','deleted')
		  AND pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
	`, advanceID)
}

func (r *Repository) CreatePayableAdvanceRequest(ctx context.Context, payable Record, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	advanceID := newID()
	projectID := cleanID(payable["project_id"])
	payableID := cleanID(payable["id"])
	rows, err := queryTx(ctx, tx, `
		INSERT INTO advance_requests (
			id, project_id, payable_id, request_code, requested_by, amount, reason, status,
			vendor_name, due_date, notes
		)
		VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid,$6,$7,'draft',$8,NULLIF($9,'')::date,$10)
		RETURNING *
	`, advanceID, projectID, payableID, body["request_code"], actorID, number(body["amount"]), body["reason"], body["vendor_name"], fmt.Sprint(body["due_date"]), body["notes"])
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "payable_advance_requested", "advance_request", advanceID, "Tạo yêu cầu ứng trước đầu vào", Record{
		"payable_id":   payableID,
		"request_code": body["request_code"],
		"amount":       body["amount"],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPayableAdvanceRequestDetail(ctx, advanceID)
}

func (r *Repository) PayableAdvanceRequestAction(ctx context.Context, advanceID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	if action != "approve" {
		return nil, validationError("Hành động ứng trước không hợp lệ", Record{"action": action})
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	advanceRows, err := queryTx(ctx, tx, `SELECT * FROM advance_requests WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, advanceID)
	if err != nil {
		return nil, err
	}
	if len(advanceRows) == 0 {
		return nil, sql.ErrNoRows
	}
	advance := advanceRows[0]
	fromStatus := fmt.Sprint(advance["status"])
	if !statusIn(fromStatus, "draft", "submitted") {
		return nil, validationError("Trạng thái ứng trước không cho phép duyệt", Record{"status": fromStatus})
	}
	payableRows, err := queryTx(ctx, tx, `SELECT * FROM payables WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, advance["payable_id"])
	if err != nil {
		return nil, err
	}
	if len(payableRows) == 0 {
		return nil, sql.ErrNoRows
	}
	payable := payableRows[0]
	if !statusIn(fmt.Sprint(payable["status"]), "approved", "partially_paid", "overdue") {
		return nil, validationError("Phải trả cần được duyệt trước khi duyệt ứng trước", Record{"status": payable["status"]})
	}
	amount := number(advance["amount"])
	if amount <= 0 || amount > number(payable["remaining_amount"]) {
		return nil, validationError("Số tiền ứng trước vượt số còn phải trả", Record{"amount": amount, "remaining_amount": payable["remaining_amount"]})
	}
	updatedAdvance, err := queryTx(ctx, tx, `
		UPDATE advance_requests
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
	newRemaining := number(payable["remaining_amount"]) - amount
	newStatus := payableSettlementStatus(number(payable["paid_amount"])+number(payable["advance_amount"])+amount, newRemaining)
	if _, err := tx.ExecContext(ctx, `
		UPDATE payables
		SET advance_amount=advance_amount + $2,
			remaining_amount=GREATEST(remaining_amount - $2, 0),
			status=$3,
			approved_by=COALESCE(approved_by,NULLIF($4,'')::uuid),
			updated_at=now()
		WHERE id=$1
	`, advance["payable_id"], amount, newStatus, actorID); err != nil {
		return nil, err
	}
	comment := actionComment(req)
	metadata := Record{"action": action, "from_status": fromStatus, "to_status": "approved", "payable_id": advance["payable_id"], "amount": amount}
	if comment != "" {
		metadata["comment"] = comment
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(advance["project_id"]), actorID, actorName, "payable_advance_approved", "advance_request", advanceID, "Duyệt ứng trước đầu vào", metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPayableAdvanceRequestDetail(ctx, advanceID)
}

func (r *Repository) SettlePayableAdvanceRequest(ctx context.Context, advanceID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM advance_requests WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, advanceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	if !statusIn(fromStatus, "approved", "partially_settled") {
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
		UPDATE advance_requests
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
		"from_status":      fromStatus,
		"to_status":        toStatus,
		"payable_id":       current["payable_id"],
		"amount":           amount,
		"input_invoice_id": cleanID(body["input_invoice_id"]),
		"schedule_id":      cleanID(body["schedule_id"]),
	}
	if comment := strings.TrimSpace(fmt.Sprint(body["comment"])); comment != "" {
		metadata["comment"] = comment
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(current["project_id"]), actorID, actorName, "payable_advance_settled", "advance_request", advanceID, "Quyết toán ứng trước đầu vào", metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPayableAdvanceRequestDetail(ctx, advanceID)
}

func (r *Repository) ListPayablePayments(ctx context.Context, payableID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			pmt.*,
			p.name AS project_name,
			pa.id AS allocation_id,
			pa.schedule_id,
			pa.advance_request_id,
			pa.input_invoice_id,
			pa.allocation_type,
			pa.amount AS allocation_amount,
			pa.notes AS allocation_notes,
			pps.schedule_code,
			pps.milestone_name,
			ar.request_code AS advance_request_code,
			ii.invoice_number AS input_invoice_number,
			pay.payable_code,
			pay.vendor_name AS payable_vendor_name,
			COALESCE(created.full_name, '') AS created_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM payments pmt
		JOIN projects p ON p.id=pmt.project_id
		JOIN payment_allocations pa ON pa.payment_id=pmt.id
		JOIN payables pay ON pay.id=pa.payable_id
		LEFT JOIN payable_payment_schedules pps ON pps.id=pa.schedule_id
		LEFT JOIN advance_requests ar ON ar.id=pa.advance_request_id
		LEFT JOIN input_invoices ii ON ii.id=pa.input_invoice_id
		LEFT JOIN users created ON created.id=pmt.created_by
		LEFT JOIN users approved ON approved.id=pmt.approved_by
		WHERE pa.payable_id=$1
		  AND pmt.status <> 'cancelled'
		  AND pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		ORDER BY pmt.created_at DESC
	`, payableID)
}

func (r *Repository) GetPayablePaymentDetail(ctx context.Context, paymentID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			pmt.*,
			p.name AS project_name,
			pa.id AS allocation_id,
			pa.payable_id,
			pa.schedule_id,
			pa.advance_request_id,
			pa.input_invoice_id,
			pa.allocation_type,
			pa.amount AS allocation_amount,
			pa.notes AS allocation_notes,
			pps.schedule_code,
			pps.milestone_name,
			ar.request_code AS advance_request_code,
			ii.invoice_number AS input_invoice_number,
			pay.payable_code,
			pay.vendor_name AS payable_vendor_name,
			COALESCE(created.full_name, '') AS created_by_name,
			COALESCE(approved.full_name, '') AS approved_by_name
		FROM payments pmt
		JOIN projects p ON p.id=pmt.project_id
		JOIN payment_allocations pa ON pa.payment_id=pmt.id
		JOIN payables pay ON pay.id=pa.payable_id
		LEFT JOIN payable_payment_schedules pps ON pps.id=pa.schedule_id
		LEFT JOIN advance_requests ar ON ar.id=pa.advance_request_id
		LEFT JOIN input_invoices ii ON ii.id=pa.input_invoice_id
		LEFT JOIN users created ON created.id=pmt.created_by
		LEFT JOIN users approved ON approved.id=pmt.approved_by
		WHERE pmt.id=$1
		  AND pmt.status <> 'cancelled'
		  AND pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
	`, paymentID)
}

func (r *Repository) CreatePayablePayment(ctx context.Context, payable Record, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	projectID := cleanID(payable["project_id"])
	payableID := cleanID(payable["id"])
	payableRows, err := queryTx(ctx, tx, `SELECT * FROM payables WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, payableID)
	if err != nil {
		return nil, err
	}
	if len(payableRows) == 0 {
		return nil, sql.ErrNoRows
	}
	currentPayable := payableRows[0]
	amount := number(body["amount"])
	allocationType := fmt.Sprint(body["allocation_type"])
	scheduleID := cleanID(body["schedule_id"])
	advanceID := cleanID(body["advance_request_id"])
	if allocationType == "payable_payment" && scheduleID == "" && amount > number(currentPayable["remaining_amount"]) {
		return nil, validationError("Số tiền thanh toán vượt số còn phải trả", Record{"amount": amount, "remaining_amount": currentPayable["remaining_amount"]})
	}
	if scheduleID != "" {
		scheduleRows, err := queryTx(ctx, tx, `SELECT * FROM payable_payment_schedules WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, scheduleID)
		if err != nil {
			return nil, err
		}
		if len(scheduleRows) == 0 {
			return nil, sql.ErrNoRows
		}
		schedule := scheduleRows[0]
		if cleanID(schedule["payable_id"]) != payableID || !statusIn(fmt.Sprint(schedule["status"]), "approved", "partially_paid", "overdue") {
			return nil, validationError("Kỳ thanh toán không thuộc phải trả hoặc chưa được duyệt", Record{"schedule_id": scheduleID})
		}
		if amount > number(schedule["remaining_amount"]) {
			return nil, validationError("Số tiền thanh toán vượt số còn lại của kỳ", Record{"amount": amount, "schedule_remaining": schedule["remaining_amount"]})
		}
	}
	if advanceID != "" {
		advanceRows, err := queryTx(ctx, tx, `SELECT * FROM advance_requests WHERE id=$1 AND status NOT IN ('cancelled','deleted') FOR UPDATE`, advanceID)
		if err != nil {
			return nil, err
		}
		if len(advanceRows) == 0 {
			return nil, sql.ErrNoRows
		}
		advance := advanceRows[0]
		if cleanID(advance["payable_id"]) != payableID || !statusIn(fmt.Sprint(advance["status"]), "approved", "partially_settled") {
			return nil, validationError("Ứng trước không thuộc phải trả hoặc chưa được duyệt", Record{"advance_request_id": advanceID})
		}
		if amount > number(advance["amount"])-number(advance["paid_amount"]) {
			return nil, validationError("Số tiền thanh toán vượt số ứng trước chưa chi", Record{"amount": amount})
		}
	}
	paymentID := newID()
	rows, err := queryTx(ctx, tx, `
		INSERT INTO payments (
			id, project_id, payment_code, payment_type, target_type, target_id,
			amount, payment_method, status, paid_at, created_by,
			accounting_book_id, budget_id, cash_account_id
		)
		VALUES (
			$1,$2,$3,$4,'payable',$5,$6,$7,'draft',NULLIF($8,'')::timestamptz,NULLIF($9,'')::uuid,
			NULLIF($10,'')::uuid,NULLIF($11,'')::uuid,NULLIF($12,'')::uuid
		)
		RETURNING *
	`, paymentID, projectID, body["payment_code"], body["payment_type"], payableID, amount, body["payment_method"], fmt.Sprint(body["paid_at"]), actorID,
		body["accounting_book_id"], body["budget_id"], body["cash_account_id"])
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO payment_allocations (
			project_id, payment_id, payable_id, schedule_id, advance_request_id, input_invoice_id,
			allocation_type, amount, notes, created_by
		)
		VALUES ($1,$2,$3,NULLIF($4,'')::uuid,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,$7,$8,$9,NULLIF($10,'')::uuid)
	`, projectID, paymentID, payableID, scheduleID, advanceID, cleanID(body["input_invoice_id"]), allocationType, amount, body["notes"], actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "payable_payment_created", "payment", paymentID, "Tạo phiếu thanh toán đầu vào", Record{
		"payable_id":         payableID,
		"amount":             amount,
		"allocation_type":    allocationType,
		"schedule_id":        scheduleID,
		"advance_request_id": advanceID,
		"input_invoice_id":   cleanID(body["input_invoice_id"]),
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPayablePaymentDetail(ctx, paymentID)
}

func (r *Repository) ListInputInvoicesForPayable(ctx context.Context, payableID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			ii.*,
			p.name AS project_name,
			pay.payable_code,
			pay.vendor_type,
			pay.vendor_name AS payable_vendor_name,
			pay.gross_amount AS payable_gross_amount,
			pay.remaining_amount AS payable_remaining_amount,
			pr.request_code AS procurement_request_code,
			d.name AS document_name,
			COALESCE(uploaded.full_name, '') AS uploaded_by_name,
			COALESCE(verified.full_name, '') AS verified_by_name,
			COALESCE(rejected.full_name, '') AS rejected_by_name
		FROM input_invoices ii
		JOIN projects p ON p.id=ii.project_id
		JOIN payables pay ON pay.id=ii.payable_id
		LEFT JOIN procurement_requests pr ON pr.id=ii.procurement_request_id
		LEFT JOIN documents d ON d.id=ii.document_id
		LEFT JOIN users uploaded ON uploaded.id=ii.uploaded_by
		LEFT JOIN users verified ON verified.id=ii.verified_by
		LEFT JOIN users rejected ON rejected.id=ii.rejected_by
		WHERE ii.payable_id=$1
		  AND ii.status NOT IN ('deleted','cancelled')
		  AND pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		ORDER BY ii.invoice_date DESC, ii.created_at DESC
	`, payableID)
}

func (r *Repository) ListInputInvoicesForProject(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			ii.*,
			p.name AS project_name,
			pay.payable_code,
			pay.vendor_type,
			pay.vendor_name AS payable_vendor_name,
			pay.gross_amount AS payable_gross_amount,
			pay.remaining_amount AS payable_remaining_amount,
			pr.request_code AS procurement_request_code,
			d.name AS document_name,
			COALESCE(uploaded.full_name, '') AS uploaded_by_name,
			COALESCE(verified.full_name, '') AS verified_by_name,
			COALESCE(rejected.full_name, '') AS rejected_by_name
		FROM input_invoices ii
		JOIN projects p ON p.id=ii.project_id
		JOIN payables pay ON pay.id=ii.payable_id
		LEFT JOIN procurement_requests pr ON pr.id=ii.procurement_request_id
		LEFT JOIN documents d ON d.id=ii.document_id
		LEFT JOIN users uploaded ON uploaded.id=ii.uploaded_by
		LEFT JOIN users verified ON verified.id=ii.verified_by
		LEFT JOIN users rejected ON rejected.id=ii.rejected_by
		WHERE ii.project_id=$1
		  AND ii.status NOT IN ('deleted','cancelled')
		  AND pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
		ORDER BY ii.invoice_date DESC, ii.created_at DESC
	`, projectID)
}

func (r *Repository) GetInputInvoiceDetail(ctx context.Context, invoiceID string) (Record, error) {
	return getInputInvoiceDetail(ctx, r.db, invoiceID)
}

func getInputInvoiceDetail(ctx context.Context, db queryContext, invoiceID string) (Record, error) {
	return queryOneRecord(ctx, db, `
		SELECT
			ii.*,
			p.name AS project_name,
			pay.payable_code,
			pay.vendor_type,
			pay.vendor_name AS payable_vendor_name,
			pay.gross_amount AS payable_gross_amount,
			pay.remaining_amount AS payable_remaining_amount,
			pr.request_code AS procurement_request_code,
			d.name AS document_name,
			COALESCE(uploaded.full_name, '') AS uploaded_by_name,
			COALESCE(verified.full_name, '') AS verified_by_name,
			COALESCE(rejected.full_name, '') AS rejected_by_name
		FROM input_invoices ii
		JOIN projects p ON p.id=ii.project_id
		JOIN payables pay ON pay.id=ii.payable_id
		LEFT JOIN procurement_requests pr ON pr.id=ii.procurement_request_id
		LEFT JOIN documents d ON d.id=ii.document_id
		LEFT JOIN users uploaded ON uploaded.id=ii.uploaded_by
		LEFT JOIN users verified ON verified.id=ii.verified_by
		LEFT JOIN users rejected ON rejected.id=ii.rejected_by
		WHERE ii.id=$1
		  AND ii.status NOT IN ('deleted','cancelled')
		  AND pay.status <> 'cancelled'
		  AND p.status <> 'cancelled'
	`, invoiceID)
}

func (r *Repository) CreateInputInvoice(ctx context.Context, payable Record, body Record, file StoredFileMetadata, actorID, actorName string) (result Record, err error) {
	tracker := storedMutationTracker{}
	defer tracker.wrap(&err)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	projectID := cleanID(payable["project_id"])
	payableID := cleanID(payable["id"])
	invoiceID := newID()
	documentID := cleanID(body["document_id"])
	if documentID == "" {
		documentID = newID()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO documents (id, project_id, target_type, target_id, name, file_url, document_type, status, uploaded_by)
			VALUES ($1, $2, 'input_invoice', $3, $4, $5, 'input_invoice_pdf', 'active', NULLIF($6,'')::uuid)
		`, documentID, projectID, invoiceID, fmt.Sprintf("Hoá đơn đầu vào %s", fmt.Sprint(body["invoice_number"])), fmt.Sprintf("/api/input-invoices/%s/file", invoiceID), actorID)
		if err != nil {
			return nil, err
		}
	}

	rows, err := queryTx(ctx, tx, `
		INSERT INTO input_invoices (
			id, project_id, payable_id, procurement_request_id, document_id,
			invoice_number, invoice_symbol, invoice_date, vendor_partner_id, vendor_name, vendor_tax_code,
			amount_before_vat, vat_amount, total_amount, currency, status, notes,
			storage_provider, storage_key, mime_type, file_name, file_size, checksum_sha256,
			uploaded_by, accounting_book_id, tax_code_id, expense_account_id
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
		payableID,
		cleanID(body["procurement_request_id"]),
		documentID,
		fmt.Sprint(body["invoice_number"]),
		fmt.Sprint(body["invoice_symbol"]),
		fmt.Sprint(body["invoice_date"]),
		cleanID(body["vendor_partner_id"]),
		fmt.Sprint(body["vendor_name"]),
		fmt.Sprint(body["vendor_tax_code"]),
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
		body["expense_account_id"],
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	if err := insertInputInvoiceTaxLineTx(ctx, tx, rows[0], actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE payables SET updated_at=now() WHERE id=$1`, payableID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "input_invoice_uploaded", "input_invoice", invoiceID, "Tải PDF hóa đơn đầu vào", Record{
		"payable_id":             payableID,
		"document_id":            documentID,
		"procurement_request_id": cleanID(body["procurement_request_id"]),
		"invoice_number":         body["invoice_number"],
		"total_amount":           body["total_amount"],
		"checksum_sha256":        file.ChecksumSHA256,
	}); err != nil {
		return nil, err
	}
	return finalizeStoredMutation(&tracker, func() (Record, error) {
		return getInputInvoiceDetail(ctx, tx, invoiceID)
	}, tx.Commit)
}

func (r *Repository) InputInvoiceAction(ctx context.Context, invoiceID, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM input_invoices WHERE id=$1 AND status NOT IN ('deleted','cancelled') FOR UPDATE`, invoiceID)
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
	case "verify":
		if fromStatus != "draft" {
			return nil, validationError("Trạng thái hoá đơn không cho phép xác thực", Record{"status": fromStatus})
		}
		toStatus = "verified"
		event = "input_invoice_verified"
		updated, err = queryTx(ctx, tx, `
			UPDATE input_invoices
			SET status='verified',
				verified_by=NULLIF($2,'')::uuid,
				verified_at=now(),
				rejected_by=NULL,
				rejected_at=NULL,
				updated_at=now()
			WHERE id=$1 AND status=$3
			RETURNING *
		`, invoiceID, actorID, fromStatus)
	case "reject":
		if !statusIn(fromStatus, "draft", "verified") {
			return nil, validationError("Trạng thái hoá đơn không cho phép từ chối", Record{"status": fromStatus})
		}
		if fromStatus == "verified" && actionComment(req) == "" {
			return nil, validationError("Lý do điều chỉnh hóa đơn đã ghi sổ là bắt buộc")
		}
		toStatus = "rejected"
		event = "input_invoice_rejected"
		updated, err = queryTx(ctx, tx, `
			UPDATE input_invoices
			SET status='rejected',
				rejected_by=NULLIF($2,'')::uuid,
				rejected_at=now(),
				verified_by=CASE WHEN $3='verified' THEN verified_by ELSE NULL END,
				verified_at=CASE WHEN $3='verified' THEN verified_at ELSE NULL END,
				updated_at=now()
			WHERE id=$1 AND status=$3
			RETURNING *
		`, invoiceID, actorID, fromStatus)
	default:
		return nil, validationError("Hành động hoá đơn đầu vào không hợp lệ", Record{"action": action})
	}
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái hoá đơn đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	var journalEntry Record
	if action == "verify" {
		journalEntry, err = r.postInputInvoiceTx(ctx, tx, updated[0], actorID)
		if err != nil {
			return nil, err
		}
	}
	if action == "reject" && fromStatus == "verified" {
		journalEntry, err = r.reverseSourceJournalTx(
			ctx, tx, cleanID(current["project_id"]), "input_invoice", invoiceID,
			"input_invoice_verified", "input_invoice_rejected", actionComment(req), actorID,
		)
		if err != nil {
			return nil, err
		}
	}
	comment := actionComment(req)
	metadata := Record{"action": action, "from_status": fromStatus, "to_status": toStatus, "payable_id": current["payable_id"]}
	if comment != "" {
		metadata["comment"] = comment
	}
	if journalEntry != nil {
		metadata["journal_entry_id"] = journalEntry["id"]
		metadata["journal_entry_no"] = journalEntry["entry_no"]
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(current["project_id"]), actorID, actorName, event, "input_invoice", invoiceID, fmt.Sprintf("Cập nhật hoá đơn đầu vào từ %s sang %s", fromStatus, toStatus), metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetInputInvoiceDetail(ctx, invoiceID)
}
