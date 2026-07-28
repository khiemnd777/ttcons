package core

import (
	"context"
	"database/sql"
	"fmt"
)

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if cleanID(value) != "" {
			return cleanID(value)
		}
	}
	return ""
}

func (s *Service) resolveAccountingAccount(ctx context.Context, projectID, requestedAccountID, defaultAccountCode string) (string, string, error) {
	projectID = cleanID(projectID)
	requestedAccountID = cleanID(requestedAccountID)
	if requestedAccountID != "" && !validActivityUUID(requestedAccountID) {
		return "", "", validationError("Tài khoản hạch toán không hợp lệ", Record{"account_id": requestedAccountID})
	}
	row, err := s.repo.QueryOne(ctx, `
		SELECT project.accounting_book_id,
			COALESCE(selected.id,default_account.id) AS account_id,
			COALESCE(selected.account_code,default_account.account_code) AS account_code
		FROM projects project
		LEFT JOIN accounting_accounts selected
		  ON selected.id=NULLIF($2,'')::uuid
		 AND selected.book_id=project.accounting_book_id
		 AND selected.active AND selected.is_postable
		LEFT JOIN accounting_accounts default_account
		  ON default_account.book_id=project.accounting_book_id
		 AND default_account.account_code=$3
		 AND default_account.active AND default_account.is_postable
		WHERE project.id=$1 AND project.status <> 'cancelled'
	`, projectID, requestedAccountID, defaultAccountCode)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", validationError("Công trình hoặc sổ kế toán không hợp lệ", Record{"project_id": projectID})
		}
		return "", "", err
	}
	accountID := cleanID(row["account_id"])
	if accountID == "" || (requestedAccountID != "" && accountID != requestedAccountID) {
		return "", "", validationError("Tài khoản không thuộc sổ kế toán của công trình hoặc không được phép hạch toán", Record{"account_id": requestedAccountID})
	}
	return cleanID(row["accounting_book_id"]), accountID, nil
}

func (s *Service) validateAccountingBudget(ctx context.Context, projectID, budgetID string) (string, error) {
	budgetID = cleanID(budgetID)
	if budgetID == "" {
		return "", nil
	}
	if !validActivityUUID(budgetID) {
		return "", validationError("Ngân sách không hợp lệ", Record{"budget_id": budgetID})
	}
	row, err := s.repo.QueryOne(ctx, `
		SELECT id,status FROM accounting_budgets
		WHERE id=$1 AND project_id=$2 AND status <> 'cancelled'
	`, budgetID, projectID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", validationError("Ngân sách không thuộc công trình", Record{"budget_id": budgetID, "project_id": projectID})
		}
		return "", err
	}
	if fmt.Sprint(row["status"]) != "approved" {
		return "", validationError("Chỉ được hạch toán vào ngân sách đã duyệt", Record{"budget_id": budgetID, "status": row["status"]})
	}
	return budgetID, nil
}

func (s *Service) resolveInvoiceTaxCode(
	ctx context.Context,
	projectID, requestedTaxCodeID, direction string,
	taxableAmount, vatAmount float64,
) (string, error) {
	requestedTaxCodeID = cleanID(requestedTaxCodeID)
	if requestedTaxCodeID != "" && !validActivityUUID(requestedTaxCodeID) {
		return "", validationError("Thuế suất không hợp lệ", Record{"tax_code_id": requestedTaxCodeID})
	}
	row, err := s.repo.QueryOne(ctx, `
		SELECT tax.id,tax.tax_code,tax.vat_rate
		FROM projects project
		JOIN tax_codes tax ON tax.book_id=project.accounting_book_id
		WHERE project.id=$1
		  AND tax.id=COALESCE(
			NULLIF($2,'')::uuid,
			accounting_tax_code_id_for_amounts(project.accounting_book_id,$3,$4,$5)
		  )
		  AND tax.active AND tax.direction IN ($5,'both')
		  AND tax.effective_from <= CURRENT_DATE
		  AND (tax.effective_to IS NULL OR tax.effective_to >= CURRENT_DATE)
	`, projectID, requestedTaxCodeID, taxableAmount, vatAmount, direction)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", validationError("Không tìm thấy thuế suất phù hợp với hóa đơn", Record{
				"tax_code_id": requestedTaxCodeID, "direction": direction,
			})
		}
		return "", err
	}
	return cleanID(row["id"]), nil
}
