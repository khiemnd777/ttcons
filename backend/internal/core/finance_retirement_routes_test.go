package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestRetiredFinanceRoutesAreUnavailable(t *testing.T) {
	app := fiber.New()
	registerFinanceRoutes(app, NewHandler(NewService(NewRepository(nil), "test-secret")))
	const projectPath = "/projects/00000000-0000-0000-0000-000000000001"
	for _, target := range []struct{ method, path string }{
		{http.MethodGet, "/payables"},
		{http.MethodPost, "/payables"},
		{http.MethodGet, "/receivables"},
		{http.MethodPost, "/receivables"},
		{http.MethodGet, "/payments"},
		{http.MethodPost, "/payments"},
		{http.MethodGet, projectPath + "/budget"},
		{http.MethodPost, projectPath + "/budget"},
		{http.MethodGet, projectPath + "/input-invoices"},
		{http.MethodGet, projectPath + "/output-invoices"},
		{http.MethodGet, projectPath + "/project-finance/options"},
		{http.MethodGet, projectPath + "/project-finance/export"},
		{http.MethodPost, projectPath + "/project-finance/material-costs"},
		{http.MethodPost, projectPath + "/project-finance/cash-requests"},
		{http.MethodGet, "/accounting/accounts"},
		{http.MethodGet, "/accounting/periods"},
		{http.MethodGet, "/accounting/tax-codes"},
		{http.MethodGet, "/accounting/tax-periods"},
		{http.MethodGet, "/accounting/journal-entries"},
		{http.MethodPost, projectPath + "/accounting/journal-entries"},
		{http.MethodGet, "/accounting/reports/trial-balance"},
		{http.MethodGet, "/accounting/reports/vat"},
		{http.MethodGet, "/accounting/reconciliation-issues"},
	} {
		response, err := app.Test(httptest.NewRequest(target.method, target.path, nil))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("retired route %s %s status=%d, want 404", target.method, target.path, response.StatusCode)
		}
	}
}

func TestRetiredFinanceDiscussionTargetsRejectedBeforeLookup(t *testing.T) {
	service := &Service{}
	for _, target := range []string{"payable", "receivable", "payment"} {
		_, err := service.normalizeProjectActivityTarget(context.Background(), newID(), WorkItemActivityRequest{ScopeType: target, ScopeID: newID()}, newID())
		if err == nil || !strings.Contains(err.Error(), "Phạm vi hoạt động không hợp lệ") {
			t.Fatalf("retired target %s error=%v, want invalid scope before repository access", target, err)
		}
	}
}
