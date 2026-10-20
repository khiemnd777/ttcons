package core

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const developerPasswordResetPurpose = "developer_reset_password"

func (r *Repository) DeveloperRecoveryAllowed(ctx context.Context, actorID string) (bool, error) {
	var allowed bool
	err := r.db.QueryRowContext(ctx, `SELECT `+developerProjectSupportUserSQL("$1::uuid"), actorID).Scan(&allowed)
	return allowed, err
}

func (r *Repository) CreateDeveloperRecoveryRequest(ctx context.Context, loginEmail string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var userID string
	err = tx.QueryRowContext(ctx, `
		SELECT account.id::text FROM users account
		WHERE account.email=$1 AND account.status='active' AND account.deleted_at IS NULL AND NOT account.is_sample
		  AND NOT EXISTS (SELECT 1 FROM user_roles assignment JOIN roles role ON role.id=assignment.role_id
		                  WHERE assignment.user_id=account.id AND role.external_only)
	`, loginEmail).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return tx.Commit()
	}
	if err != nil {
		return err
	}
	var requestID, status string
	var requestedAt time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT id::text,status,requested_at FROM account_recovery_requests
		WHERE user_id=$1::uuid AND status IN ('pending','issued') FOR UPDATE
	`, userID).Scan(&requestID, &status, &requestedAt)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil {
		if status == "pending" || time.Since(requestedAt) < time.Minute {
			return tx.Commit()
		}
		var activeToken bool
		if err = tx.QueryRowContext(ctx, `
			SELECT EXISTS(SELECT 1 FROM auth_email_tokens WHERE recovery_request_id=$1::uuid
			  AND purpose=$2 AND consumed_at IS NULL AND invalidated_at IS NULL AND expires_at>now())
		`, requestID, developerPasswordResetPurpose).Scan(&activeToken); err != nil {
			return err
		}
		if activeToken {
			return tx.Commit()
		}
		if _, err = tx.ExecContext(ctx, `
			UPDATE account_recovery_requests
			SET status='pending',requested_at=now(),issued_at=NULL,issued_by=NULL,verification_method=NULL
			WHERE id=$1::uuid
		`, requestID); err != nil {
			return err
		}
	} else {
		err = tx.QueryRowContext(ctx, `
			INSERT INTO account_recovery_requests(user_id)
			SELECT $1::uuid WHERE NOT EXISTS(
				SELECT 1 FROM account_recovery_requests
				WHERE user_id=$1::uuid AND requested_at>now()-interval '60 seconds')
			ON CONFLICT (user_id) WHERE status IN ('pending','issued') DO NOTHING
			RETURNING id::text
		`, userID).Scan(&requestID)
		if errors.Is(err, sql.ErrNoRows) {
			return tx.Commit()
		}
		if err != nil {
			return err
		}
	}
	rows, err := tx.QueryContext(ctx, `
		SELECT account.id::text FROM users account
		WHERE `+developerProjectSupportUserSQL("account.id")+`
		ORDER BY account.id LIMIT 500
	`)
	if err != nil {
		return err
	}
	developers := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		developers = append(developers, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(developers) > 0 {
		_, err = emitNotificationEventTx(ctx, tx, NotificationDomainEvent{
			Key: "account.recovery_requested", AggregateType: "account_recovery_request", AggregateID: requestID,
			Payload: Record{"request_id": requestID}, Audience: Record{"user_ids": developers},
			CorrelationKey: "account_recovery_request:" + requestID, IdempotencyKey: "account_recovery_request:" + requestID + ":" + time.Now().UTC().Format(time.RFC3339Nano),
		})
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) ListDeveloperRecoveryRequests(ctx context.Context, actorID, status string, page int) ([]Record, bool, error) {
	rows, err := r.Query(ctx, `
		SELECT request.id::text,account.email AS login_email,account.full_name,
		       CASE WHEN account.recovery_email_verified_at IS NOT NULL THEN account.recovery_email ELSE NULL END AS verified_contact_email,
		       request.status,request.requested_at,request.issued_at,request.completed_at,request.rejected_at,
		       request.verification_method,request.issued_by::text,request.rejected_by::text
		FROM account_recovery_requests request
		JOIN users account ON account.id=request.user_id
		WHERE request.status=$1 AND account.status='active' AND account.deleted_at IS NULL
		  AND `+developerProjectSupportUserSQL("$3::uuid")+`
		ORDER BY request.requested_at DESC,request.id DESC
		LIMIT 51 OFFSET $2
	`, status, page*50, actorID)
	if err != nil {
		return nil, false, err
	}
	hasMore := len(rows) > 50
	if hasMore {
		rows = rows[:50]
	}
	return rows, hasMore, nil
}

func (r *Repository) GetDeveloperRecoveryRequest(ctx context.Context, actorID, requestID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT request.id::text,account.email AS login_email,account.full_name,
		       CASE WHEN account.recovery_email_verified_at IS NOT NULL THEN account.recovery_email ELSE NULL END AS verified_contact_email,
		       request.status,request.requested_at,request.issued_at,request.completed_at,request.rejected_at,
		       request.verification_method,request.issued_by::text,request.rejected_by::text
		FROM account_recovery_requests request JOIN users account ON account.id=request.user_id
		WHERE request.id=$1::uuid AND account.status='active' AND account.deleted_at IS NULL
		  AND `+developerProjectSupportUserSQL("$2::uuid")+`
	`, requestID, actorID)
}

func (r *Repository) IssueDeveloperRecoveryLink(ctx context.Context, requestID, actorID, actorName, tokenHash string, expiresAt time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockDeveloperRecoveryActorTx(ctx, tx, actorID); err != nil {
		return err
	}
	var userID, loginEmail, status string
	var issuedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `
		SELECT account.id::text,account.email,request.status,request.issued_at
		FROM account_recovery_requests request JOIN users account ON account.id=request.user_id
		WHERE request.id=$1::uuid AND account.status='active' AND account.deleted_at IS NULL AND NOT account.is_sample
		  AND NOT EXISTS (SELECT 1 FROM user_roles assignment JOIN roles role ON role.id=assignment.role_id
		                  WHERE assignment.user_id=account.id AND role.external_only)
		  AND `+developerProjectSupportUserSQL("$2::uuid")+`
		FOR UPDATE OF request,account
	`, requestID, actorID).Scan(&userID, &loginEmail, &status, &issuedAt)
	if err != nil {
		return err
	}
	if status != "pending" && status != "issued" {
		return conflictError("Yêu cầu khôi phục đã đóng")
	}
	if issuedAt.Valid && time.Since(issuedAt.Time) < time.Minute {
		return conflictError("Vui lòng chờ một phút trước khi cấp liên kết khác")
	}
	if err := cleanupAuthEmailTokensTx(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE auth_email_tokens SET invalidated_at=now()
		WHERE user_id=$1::uuid AND purpose IN ('reset_password','developer_reset_password')
		  AND consumed_at IS NULL AND invalidated_at IS NULL
	`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO auth_email_tokens(user_id,purpose,token_hash,target_email,expires_at,recovery_request_id)
		VALUES($1::uuid,$2,$3,$4,$5,$6::uuid)
	`, userID, developerPasswordResetPurpose, tokenHash, loginEmail, expiresAt, requestID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE account_recovery_requests SET status='issued',issued_at=now(),issued_by=$2::uuid,verification_method=NULL
		WHERE id=$1::uuid
	`, requestID, actorID); err != nil {
		return err
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "account_recovery_link_issued", "user_account", userID,
		"Developer cấp liên kết đặt lại mật khẩu", Record{"request_id": requestID, "expires_at": expiresAt.UTC().Format(time.RFC3339)}); err != nil {
		return err
	}
	if err := resolveDeveloperRecoveryNotificationsTx(ctx, tx, requestID); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) RejectDeveloperRecoveryRequest(ctx context.Context, requestID, actorID, actorName string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := lockDeveloperRecoveryActorTx(ctx, tx, actorID); err != nil {
		return err
	}
	var userID, status string
	err = tx.QueryRowContext(ctx, `
		SELECT request.user_id::text,request.status FROM account_recovery_requests request
		WHERE request.id=$1::uuid AND `+developerProjectSupportUserSQL("$2::uuid")+` FOR UPDATE OF request
	`, requestID, actorID).Scan(&userID, &status)
	if err != nil {
		return err
	}
	if status != "pending" && status != "issued" {
		return conflictError("Yêu cầu khôi phục đã đóng")
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE auth_email_tokens SET invalidated_at=now()
		WHERE recovery_request_id=$1::uuid AND consumed_at IS NULL AND invalidated_at IS NULL
	`, requestID); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `
		UPDATE account_recovery_requests SET status='rejected',rejected_at=now(),rejected_by=$2::uuid WHERE id=$1::uuid
	`, requestID, actorID); err != nil {
		return err
	}
	if err = addAuditTx(ctx, tx, "", actorID, actorName, "account_recovery_request_rejected", "user_account", userID,
		"Developer từ chối yêu cầu khôi phục", Record{"request_id": requestID}); err != nil {
		return err
	}
	if err = resolveDeveloperRecoveryNotificationsTx(ctx, tx, requestID); err != nil {
		return err
	}
	return tx.Commit()
}

func lockDeveloperRecoveryActorTx(ctx context.Context, tx *sql.Tx, actorID string) error {
	var roleID string
	return tx.QueryRowContext(ctx, `
		SELECT assignment.role_id::text FROM user_roles assignment
		JOIN users actor ON actor.id=assignment.user_id
		JOIN roles role ON role.id=assignment.role_id
		WHERE actor.id=$1::uuid AND actor.status='active' AND actor.deleted_at IS NULL AND NOT actor.is_sample
		  AND assignment.project_id IS NULL AND role.name='developer' AND role.system_role AND NOT role.external_only
		  AND NOT EXISTS (SELECT 1 FROM user_roles external_assignment JOIN roles external_role ON external_role.id=external_assignment.role_id
		                  WHERE external_assignment.user_id=actor.id AND external_role.external_only)
		FOR SHARE OF assignment,actor
	`, actorID).Scan(&roleID)
}

func resolveDeveloperRecoveryNotificationsTx(ctx context.Context, tx *sql.Tx, requestID string) error {
	rows, err := tx.QueryContext(ctx, `
		UPDATE user_notifications SET resolved_at=COALESCE(resolved_at,now())
		WHERE source_kind='account_recovery_request' AND source_id=$1::uuid AND resolved_at IS NULL
		RETURNING recipient_user_id::text
	`, requestID)
	if err != nil {
		return err
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for _, id := range ids {
		if err = bumpNotificationActorRevisionTx(ctx, tx, id); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) DeveloperRecoveryNotificationRecipients(ctx context.Context, requestID string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT recipient_user_id::text FROM user_notifications
		WHERE source_kind='account_recovery_request' AND source_id=$1::uuid
	`, requestID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
