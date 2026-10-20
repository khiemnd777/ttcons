package core

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (s *Service) ListDeveloperRecoveryRequests(ctx context.Context, actorID, status string, page int) (Record, error) {
	if !statusIn(status, "pending", "issued", "completed", "rejected") || page < 0 || page > 1000 {
		return nil, validationError("Bộ lọc yêu cầu khôi phục không hợp lệ")
	}
	rows, hasMore, err := s.repo.ListDeveloperRecoveryRequests(ctx, actorID, status, page)
	if err != nil {
		return nil, err
	}
	return Record{"items": rows, "has_more": hasMore, "page": page}, nil
}

func (s *Service) GetDeveloperRecoveryRequest(ctx context.Context, actorID, requestID string) (Record, error) {
	if !validActivityUUID(requestID) {
		return nil, validationError("Yêu cầu khôi phục không hợp lệ")
	}
	row, err := s.repo.GetDeveloperRecoveryRequest(ctx, actorID, requestID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, validationError("Yêu cầu khôi phục không tồn tại")
	}
	return row, err
}

func (s *Service) IssueDeveloperRecoveryLink(ctx context.Context, requestID, actorID, actorName string) (Record, error) {
	if !s.developerRecoveryReady() {
		return nil, recoveryUnavailableError()
	}
	if !validActivityUUID(requestID) {
		return nil, validationError("Yêu cầu khôi phục không hợp lệ")
	}
	token, err := newSessionToken()
	if err != nil {
		return nil, err
	}
	ttl := s.recoveryTokenTTL
	if ttl > 15*time.Minute {
		ttl = 15 * time.Minute
	}
	expiresAt := time.Now().Add(ttl)
	if err = s.repo.IssueDeveloperRecoveryLink(ctx, requestID, actorID, actorName, hashSessionToken(token), expiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, conflictError("Yêu cầu khôi phục không còn khả dụng")
		}
		return nil, err
	}
	s.invalidateDeveloperRecoveryNotifications(ctx, requestID)
	return Record{"request_id": requestID, "link": s.frontendOrigin + "/reset-password#token=" + token, "expires_at": expiresAt.UTC().Format(time.RFC3339)}, nil
}

func (s *Service) RejectDeveloperRecoveryRequest(ctx context.Context, requestID, actorID, actorName string) (Record, error) {
	if !validActivityUUID(requestID) {
		return nil, validationError("Yêu cầu khôi phục không hợp lệ")
	}
	if err := s.repo.RejectDeveloperRecoveryRequest(ctx, requestID, actorID, actorName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, conflictError("Yêu cầu khôi phục không còn khả dụng")
		}
		return nil, err
	}
	s.invalidateDeveloperRecoveryNotifications(ctx, requestID)
	return Record{"request_id": requestID, "status": "rejected"}, nil
}

func (s *Service) invalidateDeveloperRecoveryNotifications(ctx context.Context, requestID string) {
	ids, err := s.repo.DeveloperRecoveryNotificationRecipients(ctx, requestID)
	if err != nil {
		return
	}
	for _, id := range ids {
		s.publishPersonalInvalidation("notification_changed", id, "")
	}
}
