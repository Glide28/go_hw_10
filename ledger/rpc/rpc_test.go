package rpc

import (
	"context"
	"testing"
	"time"

	"github.com/Glide28/go_hw_10/ledger"
	"google.golang.org/grpc"
)

type fakeService struct{}

func (fakeService) SetBudget(_ context.Context, value ledger.Budget) (ledger.Budget, error) {
	return value, nil
}
func (fakeService) ListBudgets(context.Context) ([]ledger.Budget, error) { return nil, nil }
func (fakeService) AddTransaction(_ context.Context, value ledger.Transaction) (ledger.Transaction, error) {
	return value, nil
}
func (fakeService) BulkImport(_ context.Context, items []ledger.Transaction, _ int) (ledger.ImportSummary, error) {
	return ledger.ImportSummary{Accepted: len(items), Errors: []ledger.ImportError{}}, nil
}
func (fakeService) ListTransactions(context.Context) ([]ledger.Transaction, error) { return nil, nil }
func (fakeService) GetReportSummary(context.Context, time.Time, time.Time) ([]ledger.ReportItem, error) {
	return nil, nil
}

func TestRegisterService(t *testing.T) {
	server := grpc.NewServer()
	Register(server, fakeService{})
	if len(server.GetServiceInfo()) != 1 {
		t.Fatalf("registered services = %d, want 1", len(server.GetServiceInfo()))
	}
}
