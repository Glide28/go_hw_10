package ledger

import (
	"context"

	"github.com/Glide28/go_hw_10/ledger/internal/app"
	"github.com/Glide28/go_hw_10/ledger/internal/domain"
	"github.com/Glide28/go_hw_10/ledger/internal/service"
)

type Transaction = domain.Transaction
type Budget = domain.Budget
type ReportItem = domain.ReportItem
type ImportError = domain.ImportError
type ImportSummary = domain.ImportSummary
type Service = service.Service

var ErrBudgetExceeded = domain.ErrBudgetExceeded

func OpenService(ctx context.Context) (Service, func(), error) {
	return app.Open(ctx)
}
