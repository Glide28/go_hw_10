package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrBudgetExceeded = errors.New("budget exceeded")

type Transaction struct {
	ID          int       `json:"id"`
	Amount      float64   `json:"amount"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Date        time.Time `json:"date"`
}

func (tx Transaction) Validate() error {
	if tx.Amount <= 0 {
		return errors.New("amount must be positive")
	}
	if strings.TrimSpace(tx.Category) == "" {
		return errors.New("category must not be empty")
	}
	if tx.Date.IsZero() {
		return errors.New("date must not be empty")
	}
	return nil
}

type Budget struct {
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
}

func (b Budget) Validate() error {
	if strings.TrimSpace(b.Category) == "" {
		return errors.New("category must not be empty")
	}
	if b.Limit <= 0 {
		return errors.New("limit must be positive")
	}
	return nil
}

type ReportItem struct {
	Category string  `json:"category"`
	Total    float64 `json:"total"`
}

type ImportError struct {
	Index int    `json:"index"`
	Error string `json:"error"`
}

type ImportSummary struct {
	Accepted int           `json:"accepted"`
	Rejected int           `json:"rejected"`
	Errors   []ImportError `json:"errors"`
}

type BudgetRepository interface {
	Upsert(ctx context.Context, budget Budget) (Budget, error)
	GetByCategory(ctx context.Context, category string) (Budget, bool, error)
	List(ctx context.Context) ([]Budget, error)
}

type ExpenseRepository interface {
	Add(ctx context.Context, transaction Transaction) (Transaction, error)
	List(ctx context.Context) ([]Transaction, error)
	SumByCategory(ctx context.Context, category string) (float64, error)
	Categories(ctx context.Context, from, to time.Time) ([]string, error)
	SumByCategoryPeriod(ctx context.Context, category string, from, to time.Time) (float64, error)
}

type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}
