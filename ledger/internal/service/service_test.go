package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Glide28/go_hw_10/ledger/internal/domain"
)

type budgetRepository struct {
	budget    *domain.Budget
	listCalls int
}

func (r *budgetRepository) Upsert(_ context.Context, budget domain.Budget) (domain.Budget, error) {
	r.budget = &budget
	return budget, nil
}
func (r *budgetRepository) GetByCategory(_ context.Context, category string) (domain.Budget, bool, error) {
	if r.budget == nil || r.budget.Category != category {
		return domain.Budget{}, false, nil
	}
	return *r.budget, true, nil
}
func (r *budgetRepository) List(context.Context) ([]domain.Budget, error) {
	r.listCalls++
	if r.budget == nil {
		return nil, nil
	}
	return []domain.Budget{*r.budget}, nil
}

type memoryCache struct {
	values  map[string][]byte
	deleted []string
}

func (c *memoryCache) Get(_ context.Context, key string) ([]byte, error) {
	value, ok := c.values[key]
	if !ok {
		return nil, errors.New("cache miss")
	}
	return value, nil
}

func (c *memoryCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	if c.values == nil {
		c.values = make(map[string][]byte)
	}
	c.values[key] = value
	return nil
}

func (c *memoryCache) Delete(_ context.Context, key string) error {
	delete(c.values, key)
	c.deleted = append(c.deleted, key)
	return nil
}

type expenseRepository struct {
	spent          float64
	items          []domain.Transaction
	categoryTotals map[string]float64
	delay          time.Duration
}

func (r *expenseRepository) Add(ctx context.Context, transaction domain.Transaction) (domain.Transaction, error) {
	if r.delay > 0 {
		select {
		case <-time.After(r.delay):
		case <-ctx.Done():
			return domain.Transaction{}, ctx.Err()
		}
	}
	transaction.ID = len(r.items) + 1
	r.items = append(r.items, transaction)
	r.spent += transaction.Amount
	return transaction, nil
}
func (r *expenseRepository) List(context.Context) ([]domain.Transaction, error) { return r.items, nil }
func (r *expenseRepository) SumByCategory(context.Context, string) (float64, error) {
	return r.spent, nil
}
func (r *expenseRepository) Categories(context.Context, time.Time, time.Time) ([]string, error) {
	if len(r.categoryTotals) == 0 {
		return []string{"еда"}, nil
	}
	result := make([]string, 0, len(r.categoryTotals))
	for category := range r.categoryTotals {
		result = append(result, category)
	}
	return result, nil
}
func (r *expenseRepository) SumByCategoryPeriod(
	ctx context.Context,
	category string,
	_, _ time.Time,
) (float64, error) {
	if r.delay > 0 {
		select {
		case <-time.After(r.delay):
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
	if r.categoryTotals != nil {
		return r.categoryTotals[category], nil
	}
	return r.spent, nil
}

func TestAddTransactionChecksBudget(t *testing.T) {
	budgets := &budgetRepository{budget: &domain.Budget{Category: "еда", Limit: 5000}}
	expenses := &expenseRepository{spent: 3000}
	application := New(budgets, expenses, nil)

	_, err := application.AddTransaction(context.Background(), domain.Transaction{
		Amount: 2500, Category: "еда", Date: time.Now(),
	})
	if !errors.Is(err, domain.ErrBudgetExceeded) {
		t.Fatalf("expected ErrBudgetExceeded, got %v", err)
	}
}

func TestSetBudgetTrimsCategory(t *testing.T) {
	budgets := &budgetRepository{}
	application := New(budgets, &expenseRepository{}, nil)
	created, err := application.SetBudget(context.Background(), domain.Budget{Category: "  еда ", Limit: 5000})
	if err != nil || created.Category != "еда" {
		t.Fatalf("SetBudget() = %+v, %v", created, err)
	}
}

func TestListBudgetsUsesRedisCache(t *testing.T) {
	repository := &budgetRepository{budget: &domain.Budget{Category: "еда", Limit: 5000}}
	cache := &memoryCache{values: make(map[string][]byte)}
	application := New(repository, &expenseRepository{}, cache)

	first, err := application.ListBudgets(context.Background())
	if err != nil || len(first) != 1 {
		t.Fatalf("first ListBudgets() = %+v, %v", first, err)
	}
	if repository.listCalls != 1 {
		t.Fatalf("repository calls after cache miss = %d, want 1", repository.listCalls)
	}

	repository.budget = &domain.Budget{Category: "транспорт", Limit: 7000}
	second, err := application.ListBudgets(context.Background())
	if err != nil || len(second) != 1 || second[0].Category != "еда" {
		t.Fatalf("cached ListBudgets() = %+v, %v", second, err)
	}
	if repository.listCalls != 1 {
		t.Fatalf("repository calls after cache hit = %d, want 1", repository.listCalls)
	}
}

func TestSetBudgetInvalidatesCachedList(t *testing.T) {
	cached, _ := json.Marshal([]domain.Budget{{Category: "еда", Limit: 5000}})
	cache := &memoryCache{values: map[string][]byte{"budgets:all": cached}}
	application := New(&budgetRepository{}, &expenseRepository{}, cache)

	_, err := application.SetBudget(context.Background(), domain.Budget{Category: "еда", Limit: 6000})
	if err != nil {
		t.Fatal(err)
	}
	if len(cache.deleted) != 1 || cache.deleted[0] != "budgets:all" {
		t.Fatalf("deleted keys = %v", cache.deleted)
	}
}

func TestValidation(t *testing.T) {
	application := New(&budgetRepository{}, &expenseRepository{}, nil)
	_, err := application.AddTransaction(context.Background(), domain.Transaction{Amount: 0, Category: "еда", Date: time.Now()})
	if err == nil {
		t.Fatal("zero amount must fail")
	}
}

func TestReportSummaryCalculatesCategories(t *testing.T) {
	expenses := &expenseRepository{categoryTotals: map[string]float64{
		"еда": 1250, "транспорт": 780,
	}}
	application := New(&budgetRepository{}, expenses, nil)

	items, err := application.GetReportSummary(
		context.Background(),
		time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 11, 30, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Category != "еда" || items[1].Category != "транспорт" {
		t.Fatalf("unexpected report: %+v", items)
	}
}

func TestReportSummaryStopsOnContextCancellation(t *testing.T) {
	expenses := &expenseRepository{categoryTotals: map[string]float64{"еда": 1250}, delay: time.Second}
	application := New(&budgetRepository{}, expenses, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := application.GetReportSummary(ctx, time.Now().AddDate(0, -1, 0), time.Now())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestBulkImportUsesWorkerPoolAndCollectsErrors(t *testing.T) {
	application := New(&budgetRepository{}, &expenseRepository{}, nil)
	transactions := []domain.Transaction{
		{Amount: 1200, Category: "еда", Date: time.Now()},
		{Amount: 0, Category: "еда", Date: time.Now()},
		{Amount: 700, Category: "транспорт", Date: time.Now()},
	}

	summary, err := application.BulkImport(context.Background(), transactions, 3)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Accepted != 2 || summary.Rejected != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if len(summary.Errors) != 1 || summary.Errors[0].Index != 1 {
		t.Fatalf("unexpected errors: %+v", summary.Errors)
	}
}

func TestBulkImportReturnsPartialResultOnCancellation(t *testing.T) {
	expenses := &expenseRepository{delay: 100 * time.Millisecond}
	application := New(&budgetRepository{}, expenses, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	summary, err := application.BulkImport(ctx, []domain.Transaction{
		{Amount: 100, Category: "еда", Date: time.Now()},
		{Amount: 200, Category: "еда", Date: time.Now()},
	}, 2)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if summary.Accepted != 0 {
		t.Fatalf("unexpected partial summary: %+v", summary)
	}
}
