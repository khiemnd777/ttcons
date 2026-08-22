package core

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const (
	recoveryVerificationPurpose = "verify_recovery_email"
	passwordResetPurpose        = "reset_password"
)

func (r *Repository) CreateRecoveryVerificationToken(ctx context.Context, userID, expectedPasswordHash, tokenHash, targetEmail string, expiresAt time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := cleanupAuthEmailTokensTx(ctx, tx); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE auth_email_tokens
		SET invalidated_at=now()
		WHERE user_id=$1 AND purpose=$2
		  AND consumed_at IS NULL AND invalidated_at IS NULL
	`, userID, recoveryVerificationPurpose); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO auth_email_tokens (user_id,purpose,token_hash,target_email,expires_at)
		SELECT account.id,$2,$3,$4,$5
		FROM users account
		WHERE account.id=$1 AND account.status='active' AND account.password_hash=$6
		  AND lower(account.email)<>$4
		  AND NOT (account.recovery_email=$4 AND account.recovery_email_verified_at IS NOT NULL)
		  AND NOT EXISTS (
			SELECT 1 FROM user_roles assignment
			JOIN roles role ON role.id=assignment.role_id
			WHERE assignment.user_id=account.id AND role.external_only
		  )
	`, userID, recoveryVerificationPurpose, tokenHash, targetEmail, expiresAt, expectedPasswordHash)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return tx.Commit()
}

func (r *Repository) InvalidateAuthEmailToken(ctx context.Context, tokenHash string) error {
	return r.Exec(ctx, `
		UPDATE auth_email_tokens
		SET invalidated_at=now()
		WHERE token_hash=$1 AND consumed_at IS NULL AND invalidated_at IS NULL
	`, tokenHash)
}

func (r *Repository) VerifyRecoveryEmail(ctx context.Context, tokenHash string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var tokenID, userID, fullName, targetEmail string
	err = tx.QueryRowContext(ctx, `
		SELECT token.id::text,account.id::text,account.full_name,token.target_email
		FROM auth_email_tokens token
		JOIN users account ON account.id=token.user_id
		WHERE token.token_hash=$1 AND token.purpose=$2
		  AND token.consumed_at IS NULL AND token.invalidated_at IS NULL
		  AND token.expires_at > now() AND account.status='active'
		  AND NOT EXISTS (
			SELECT 1 FROM user_roles assignment
			JOIN roles role ON role.id=assignment.role_id
			WHERE assignment.user_id=account.id AND role.external_only
		  )
		FOR UPDATE OF token,account
	`, tokenHash, recoveryVerificationPurpose).Scan(&tokenID, &userID, &fullName, &targetEmail)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET recovery_email=$2,recovery_email_verified_at=now(),updated_at=now()
		WHERE id=$1
	`, userID, targetEmail); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE auth_email_tokens SET consumed_at=now() WHERE id=$1`, tokenID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE auth_email_tokens SET invalidated_at=now()
		WHERE user_id=$1 AND purpose=$2 AND id<>$3
		  AND consumed_at IS NULL AND invalidated_at IS NULL
	`, userID, recoveryVerificationPurpose, tokenID); err != nil {
		return err
	}
	if err := addAuditTx(ctx, tx, "", userID, fullName, "user_recovery_email_verified", "user_account", userID, "Người dùng xác minh email khôi phục", Record{
		"recovery_email_masked": maskEmail(targetEmail),
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) RemoveRecoveryEmail(ctx context.Context, userID, actorName, expectedPasswordHash string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var currentEmail sql.NullString
	if err := tx.QueryRowContext(ctx, `
		SELECT recovery_email FROM users WHERE id=$1 AND status='active' AND password_hash=$2 FOR UPDATE
	`, userID, expectedPasswordHash).Scan(&currentEmail); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE users SET recovery_email=NULL,recovery_email_verified_at=NULL,updated_at=now() WHERE id=$1
	`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE auth_email_tokens SET invalidated_at=now()
		WHERE user_id=$1 AND consumed_at IS NULL AND invalidated_at IS NULL
	`, userID); err != nil {
		return err
	}
	metadata := Record{"pending_tokens_invalidated": true}
	if currentEmail.Valid {
		metadata["recovery_email_masked"] = maskEmail(currentEmail.String)
	}
	if err := addAuditTx(ctx, tx, "", userID, actorName, "user_recovery_email_removed", "user_account", userID, "Người dùng gỡ email khôi phục", metadata); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) FindRecoveryAccountByLoginEmail(ctx context.Context, loginEmail string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT account.id::text,account.full_name,account.recovery_email
		FROM users account
		WHERE account.email=$1 AND account.status='active'
		  AND account.recovery_email IS NOT NULL
		  AND account.recovery_email_verified_at IS NOT NULL
		  AND NOT EXISTS (
			SELECT 1 FROM user_roles assignment
			JOIN roles role ON role.id=assignment.role_id
			WHERE assignment.user_id=account.id AND role.external_only
		  )
	`, loginEmail)
}

func (r *Repository) CreatePasswordResetToken(ctx context.Context, userID, tokenHash, targetEmail string, expiresAt time.Time) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if err := cleanupAuthEmailTokensTx(ctx, tx); err != nil {
		return false, err
	}

	var lockedEmail string
	if err := tx.QueryRowContext(ctx, `
		SELECT account.recovery_email
		FROM users account
		WHERE account.id=$1 AND account.status='active'
		  AND account.recovery_email=$2 AND account.recovery_email_verified_at IS NOT NULL
		  AND NOT EXISTS (
			SELECT 1 FROM user_roles assignment
			JOIN roles role ON role.id=assignment.role_id
			WHERE assignment.user_id=account.id AND role.external_only
		  )
		FOR UPDATE OF account
	`, userID, targetEmail).Scan(&lockedEmail); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	var coolingDown bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM auth_email_tokens
			WHERE user_id=$1 AND purpose=$2 AND created_at > now()-interval '60 seconds'
			  AND consumed_at IS NULL AND invalidated_at IS NULL AND expires_at > now()
		)
	`, userID, passwordResetPurpose).Scan(&coolingDown); err != nil {
		return false, err
	}
	if coolingDown {
		return false, nil
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE auth_email_tokens SET invalidated_at=now()
		WHERE user_id=$1 AND purpose=$2 AND consumed_at IS NULL AND invalidated_at IS NULL
	`, userID, passwordResetPurpose); err != nil {
		return false, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO auth_email_tokens (user_id,purpose,token_hash,target_email,expires_at)
		VALUES ($1,$2,$3,$4,$5)
	`, userID, passwordResetPurpose, tokenHash, targetEmail, expiresAt); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func (r *Repository) ResetPasswordWithToken(ctx context.Context, tokenHash, passwordHash, passwordValue string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var tokenID, userID, fullName, currentPasswordHash string
	err = tx.QueryRowContext(ctx, `
		SELECT token.id::text,account.id::text,account.full_name,account.password_hash
		FROM auth_email_tokens token
		JOIN users account ON account.id=token.user_id
		WHERE token.token_hash=$1 AND token.purpose=$2
		  AND token.consumed_at IS NULL AND token.invalidated_at IS NULL
		  AND token.expires_at > now() AND account.status='active'
		  AND account.recovery_email=token.target_email
		  AND account.recovery_email_verified_at IS NOT NULL
		  AND NOT EXISTS (
			SELECT 1 FROM user_roles assignment
			JOIN roles role ON role.id=assignment.role_id
			WHERE assignment.user_id=account.id AND role.external_only
		  )
		FOR UPDATE OF token,account
	`, tokenHash, passwordResetPurpose).Scan(&tokenID, &userID, &fullName, &currentPasswordHash)
	if err != nil {
		return err
	}
	if passwordMatches(currentPasswordHash, passwordValue) {
		return validationError("Mật khẩu mới phải khác mật khẩu hiện tại", Record{"field": "new_password"})
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash=$2,must_change_password=false,updated_at=now() WHERE id=$1`, userID, passwordHash); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE auth_sessions SET revoked_at=COALESCE(revoked_at,now())
		WHERE user_id=$1 AND revoked_at IS NULL
	`, userID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE auth_email_tokens SET consumed_at=now() WHERE id=$1`, tokenID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE auth_email_tokens SET invalidated_at=now()
		WHERE user_id=$1 AND id<>$2 AND consumed_at IS NULL AND invalidated_at IS NULL
	`, userID, tokenID); err != nil {
		return err
	}
	if err := addAuditTx(ctx, tx, "", userID, fullName, "user_password_reset", "user_account", userID, "Người dùng đặt lại mật khẩu bằng email khôi phục", Record{
		"session_revoked": true,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func recoveryTokenNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

func cleanupAuthEmailTokensTx(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		DELETE FROM auth_email_tokens
		WHERE created_at < now()-interval '30 days'
		  AND (consumed_at IS NOT NULL OR invalidated_at IS NOT NULL OR expires_at <= now())
	`)
	return err
}
