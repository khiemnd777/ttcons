package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (r *Repository) ReverseApprovedPayment(ctx context.Context, paymentID string, req ActionRequest, actorID, actorName string) (Record, error) {
	reason := actionComment(req)
	if strings.TrimSpace(reason) == "" {
		return nil, validationError("Lý do đảo Phiếu thu/Phiếu chi là bắt buộc")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	payments, err := queryTx(ctx, tx, `SELECT * FROM payments WHERE id=$1 FOR UPDATE`, paymentID)
	if err != nil {
		return nil, err
	}
	if len(payments) == 0 {
		return nil, sql.ErrNoRows
	}
	payment := payments[0]
	if fmt.Sprint(payment["status"]) != "approved" {
		return nil, validationError("Chỉ được đảo Phiếu thu/Phiếu chi đã duyệt", Record{"status": payment["status"]})
	}
	originalEntries, err := queryTx(ctx, tx, `
		SELECT * FROM journal_entries
		WHERE source_type='payment' AND source_id=$1 AND status='posted' AND reversal_of_id IS NULL
		FOR SHARE
	`, paymentID)
	if err != nil {
		return nil, err
	}
	if len(originalEntries) == 0 {
		return nil, validationError("Phiếu đã duyệt chưa có bút toán nguồn; cần hoàn tất đối chiếu/backfill trước khi đảo", Record{"payment_id": paymentID})
	}
	if len(originalEntries) > 1 {
		return nil, validationError("Phiếu có nhiều bút toán nguồn, cần đối chiếu trước khi đảo", Record{"payment_id": paymentID})
	}
	original := originalEntries[0]
	if err := r.reversePaymentAllocationsTx(ctx, tx, payment, actorID, actorName); err != nil {
		return nil, err
	}
	reversal, err := r.reverseSourceJournalTx(
		ctx, tx, cleanID(payment["project_id"]), "payment", paymentID,
		fmt.Sprint(original["source_event"]), "payment_reversed", reason, actorID,
	)
	if err != nil {
		return nil, err
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE payments
		SET status='cancelled',updated_at=now()
		WHERE id=$1 AND status='approved'
		RETURNING *
	`, paymentID)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái Phiếu thu/Phiếu chi đã thay đổi, vui lòng tải lại")
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO payment_reversals (
			payment_id,project_id,original_journal_entry_id,reversal_journal_entry_id,reason,reversed_by
		) VALUES ($1,$2,$3,$4,$5,NULLIF($6,'')::uuid)
	`, paymentID, payment["project_id"], original["id"], reversal["id"], reason, actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(payment["project_id"]), actorID, actorName,
		"payment_reversed", "payment", paymentID, "Đảo Phiếu thu/Phiếu chi đã duyệt", Record{
			"reason": reason, "original_journal_entry_id": original["id"], "reversal_journal_entry_id": reversal["id"],
		}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	updated[0]["reversal_journal_entry_id"] = reversal["id"]
	updated[0]["reversal_journal_entry_no"] = reversal["entry_no"]
	return updated[0], nil
}

func (r *Repository) reversePaymentAllocationsTx(ctx context.Context, tx *sql.Tx, payment Record, actorID, actorName string) error {
	paymentAllocations, err := queryTx(ctx, tx, `SELECT * FROM payment_allocations WHERE payment_id=$1 FOR UPDATE`, payment["id"])
	if err != nil {
		return err
	}
	for _, allocation := range paymentAllocations {
		amount := number(allocation["amount"])
		switch fmt.Sprint(allocation["allocation_type"]) {
		case "payable_payment":
			payables, err := queryTx(ctx, tx, `SELECT * FROM payables WHERE id=$1 FOR UPDATE`, allocation["payable_id"])
			if err != nil {
				return err
			}
			if len(payables) == 0 || number(payables[0]["paid_amount"])+0.005 < amount {
				return validationError("Không thể đảo phân bổ vì số đã trả không còn đủ", Record{"allocation_id": allocation["id"]})
			}
			payable := payables[0]
			newPaid := number(payable["paid_amount"]) - amount
			newRemaining := number(payable["remaining_amount"]) + amount
			newStatus := payableSettlementStatus(newPaid+number(payable["advance_amount"]), newRemaining)
			if _, err := tx.ExecContext(ctx, `
				UPDATE payables SET paid_amount=$2,remaining_amount=$3,status=$4,updated_at=now() WHERE id=$1
			`, payable["id"], newPaid, newRemaining, newStatus); err != nil {
				return err
			}
			if scheduleID := cleanID(allocation["schedule_id"]); scheduleID != "" {
				schedules, err := queryTx(ctx, tx, `SELECT * FROM payable_payment_schedules WHERE id=$1 FOR UPDATE`, scheduleID)
				if err != nil {
					return err
				}
				if len(schedules) == 0 || number(schedules[0]["paid_amount"])+0.005 < amount {
					return validationError("Không thể đảo phân bổ kỳ thanh toán", Record{"schedule_id": scheduleID})
				}
				newSchedulePaid := number(schedules[0]["paid_amount"]) - amount
				newScheduleRemaining := number(schedules[0]["remaining_amount"]) + amount
				newScheduleStatus := "approved"
				if newSchedulePaid > 0 {
					newScheduleStatus = "partially_paid"
				}
				if _, err := tx.ExecContext(ctx, `
					UPDATE payable_payment_schedules
					SET paid_amount=$2,remaining_amount=$3,status=$4,updated_at=now() WHERE id=$1
				`, scheduleID, newSchedulePaid, newScheduleRemaining, newScheduleStatus); err != nil {
					return err
				}
			}
		case "advance_payment":
			advances, err := queryTx(ctx, tx, `SELECT * FROM advance_requests WHERE id=$1 FOR UPDATE`, allocation["advance_request_id"])
			if err != nil {
				return err
			}
			if len(advances) == 0 || number(advances[0]["paid_amount"])+0.005 < amount {
				return validationError("Không thể đảo chi ứng trước", Record{"advance_request_id": allocation["advance_request_id"]})
			}
			newPaid := number(advances[0]["paid_amount"]) - amount
			if number(advances[0]["settled_amount"]) > newPaid+0.005 {
				return validationError("Ứng trước đã được quyết toán; phải đảo quyết toán trước khi đảo Phiếu chi", Record{"advance_request_id": allocation["advance_request_id"]})
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE advance_requests
				SET paid_amount=$2,payment_id=CASE WHEN payment_id=$3 AND $2=0 THEN NULL ELSE payment_id END,
					paid_at=CASE WHEN payment_id=$3 AND $2=0 THEN NULL ELSE paid_at END,updated_at=now()
				WHERE id=$1
			`, advances[0]["id"], newPaid, payment["id"]); err != nil {
				return err
			}
		case "advance_settlement":
			return validationError("Phiếu quyết toán ứng trước phải được điều chỉnh tại nghiệp vụ quyết toán trước")
		}
	}

	receiptAllocations, err := queryTx(ctx, tx, `SELECT * FROM receipt_allocations WHERE payment_id=$1 FOR UPDATE`, payment["id"])
	if err != nil {
		return err
	}
	for _, allocation := range receiptAllocations {
		amount := number(allocation["amount"])
		switch fmt.Sprint(allocation["allocation_type"]) {
		case "receivable_receipt":
			receivables, err := queryTx(ctx, tx, `SELECT * FROM receivables WHERE id=$1 FOR UPDATE`, allocation["receivable_id"])
			if err != nil {
				return err
			}
			if len(receivables) == 0 || number(receivables[0]["received_amount"])+0.005 < amount {
				return validationError("Không thể đảo phân bổ vì số đã thu không còn đủ", Record{"allocation_id": allocation["id"]})
			}
			receivable := receivables[0]
			newReceived := number(receivable["received_amount"]) - amount
			newRemaining := number(receivable["remaining_amount"]) + amount
			newStatus := receivableSettlementStatus(newReceived, newRemaining)
			if _, err := tx.ExecContext(ctx, `
				UPDATE receivables SET received_amount=$2,remaining_amount=$3,status=$4,updated_at=now() WHERE id=$1
			`, receivable["id"], newReceived, newRemaining, newStatus); err != nil {
				return err
			}
			if scheduleID := cleanID(allocation["schedule_id"]); scheduleID != "" {
				schedules, err := queryTx(ctx, tx, `SELECT * FROM receivable_payment_schedules WHERE id=$1 FOR UPDATE`, scheduleID)
				if err != nil {
					return err
				}
				if len(schedules) == 0 || number(schedules[0]["received_amount"])+0.005 < amount {
					return validationError("Không thể đảo phân bổ kỳ thu", Record{"schedule_id": scheduleID})
				}
				newScheduleReceived := number(schedules[0]["received_amount"]) - amount
				newScheduleRemaining := number(schedules[0]["remaining_amount"]) + amount
				newScheduleStatus := "approved"
				if newScheduleReceived > 0 {
					newScheduleStatus = "partially_received"
				}
				if _, err := tx.ExecContext(ctx, `
					UPDATE receivable_payment_schedules
					SET received_amount=$2,remaining_amount=$3,status=$4,updated_at=now() WHERE id=$1
				`, scheduleID, newScheduleReceived, newScheduleRemaining, newScheduleStatus); err != nil {
					return err
				}
			}
		case "advance_receipt":
			advances, err := queryTx(ctx, tx, `SELECT * FROM client_advance_receipts WHERE id=$1 FOR UPDATE`, allocation["advance_receipt_id"])
			if err != nil {
				return err
			}
			if len(advances) == 0 || number(advances[0]["received_amount"])+0.005 < amount {
				return validationError("Không thể đảo thu ứng trước", Record{"advance_receipt_id": allocation["advance_receipt_id"]})
			}
			newReceived := number(advances[0]["received_amount"]) - amount
			if number(advances[0]["settled_amount"]) > newReceived+0.005 {
				return validationError("Khoản khách hàng ứng trước đã được quyết toán; phải đảo quyết toán trước khi đảo Phiếu thu", Record{"advance_receipt_id": allocation["advance_receipt_id"]})
			}
			newStatus := "approved"
			if newReceived > 0 {
				newStatus = clientAdvanceReceiptStatusAfterReceipt(number(advances[0]["amount"]), newReceived, number(advances[0]["settled_amount"]))
			}
			if _, err := tx.ExecContext(ctx, `
				UPDATE client_advance_receipts
				SET received_amount=$2,status=$3,payment_id=CASE WHEN payment_id=$4 AND $2=0 THEN NULL ELSE payment_id END,
					received_at=CASE WHEN payment_id=$4 AND $2=0 THEN NULL ELSE received_at END,updated_at=now()
				WHERE id=$1
			`, advances[0]["id"], newReceived, newStatus, payment["id"]); err != nil {
				return err
			}
		case "advance_settlement":
			return validationError("Phiếu quyết toán ứng trước phải được điều chỉnh tại nghiệp vụ quyết toán trước")
		}
	}
	if err := addAuditTx(ctx, tx, cleanID(payment["project_id"]), actorID, actorName,
		"payment_allocations_reversed", "payment", cleanID(payment["id"]),
		"Hoàn nguyên phân bổ công nợ của Phiếu thu/Phiếu chi", Record{"reason": "payment_reversal"}); err != nil {
		return err
	}
	return nil
}
