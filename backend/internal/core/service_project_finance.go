package core

import "context"

// ProjectFinanceWorkspace exposes source-backed handoff work, not an internal ledger.
func (s *Service) ProjectFinanceWorkspace(ctx context.Context, projectID string) (Record, error) {
	return s.AccountingHandoffWorkspace(ctx, projectID)
}
