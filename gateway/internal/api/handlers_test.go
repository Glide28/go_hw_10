package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Glide28/go_hw_10/ledger"
)

type fakeService struct {
	budgets      []ledger.Budget
	transactions []ledger.Transaction
	reportDelay  time.Duration
}

func (f *fakeService) SetBudget(_ context.Context, budget ledger.Budget) (ledger.Budget, error) {
	f.budgets = append(f.budgets, budget)
	return budget, nil
}
func (f *fakeService) ListBudgets(context.Context) ([]ledger.Budget, error) {
	return f.budgets, nil
}
func (f *fakeService) AddTransaction(_ context.Context, transaction ledger.Transaction) (ledger.Transaction, error) {
	transaction.ID = len(f.transactions) + 1
	f.transactions = append(f.transactions, transaction)
	return transaction, nil
}
func (f *fakeService) BulkImport(_ context.Context, items []ledger.Transaction, _ int) (ledger.ImportSummary, error) {
	return ledger.ImportSummary{Accepted: len(items), Errors: []ledger.ImportError{}}, nil
}
func (f *fakeService) ListTransactions(context.Context) ([]ledger.Transaction, error) {
	return f.transactions, nil
}
func (f *fakeService) GetReportSummary(ctx context.Context, _, _ time.Time) ([]ledger.ReportItem, error) {
	if f.reportDelay > 0 {
		select {
		case <-time.After(f.reportDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return []ledger.ReportItem{{Category: "еда", Total: 1250}}, nil
}

func perform(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

func TestBudgetEndpoints(t *testing.T) {
	handler := NewHandler(&fakeService{})
	created := perform(handler, http.MethodPost, "/api/budgets", `{"category":"еда","limit":5000}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("POST status = %d, body = %s", created.Code, created.Body.String())
	}
	list := perform(handler, http.MethodGet, "/api/budgets", "")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"category":"еда"`) {
		t.Fatalf("GET status = %d, body = %s", list.Code, list.Body.String())
	}
}

func TestInvalidInput(t *testing.T) {
	handler := NewHandler(&fakeService{})
	response := perform(handler, http.MethodPost, "/api/transactions", `{"amount":0,"category":"еда","date":"2026-10-20"}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", response.Code)
	}
	response = perform(handler, http.MethodGet, "/api/reports/summary?from=bad&to=2026-10-31", "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("report status = %d", response.Code)
	}
}

func TestReportTimeout(t *testing.T) {
	handler := NewHandlerWithTimeout(&fakeService{reportDelay: 100 * time.Millisecond}, 5*time.Millisecond)
	response := perform(handler, http.MethodGet, "/api/reports/summary?from=2026-11-01&to=2026-11-30", "")
	if response.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"error":"request timed out"`) {
		t.Fatalf("unexpected body: %s", response.Body.String())
	}
}

func TestBulkImportEndpoint(t *testing.T) {
	handler := NewHandler(&fakeService{})
	response := perform(
		handler,
		http.MethodPost,
		"/api/transactions/bulk?workers=2",
		`[{"amount":1200,"category":"еда","date":"2026-11-17"},{"amount":700,"category":"транспорт","date":"2026-11-18"}]`,
	)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"accepted":2`) || !strings.Contains(response.Body.String(), `"rejected":0`) {
		t.Fatalf("unexpected body: %s", response.Body.String())
	}
}

func TestBulkImportRejectsInvalidWorkers(t *testing.T) {
	handler := NewHandler(&fakeService{})
	response := perform(handler, http.MethodPost, "/api/transactions/bulk?workers=0", `[]`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}
