package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Glide28/go_hw_10/ledger/internal/domain"
)

type Service interface {
	SetBudget(ctx context.Context, budget domain.Budget) (domain.Budget, error)
	ListBudgets(ctx context.Context) ([]domain.Budget, error)
	AddTransaction(ctx context.Context, transaction domain.Transaction) (domain.Transaction, error)
	BulkImport(ctx context.Context, transactions []domain.Transaction, workers int) (domain.ImportSummary, error)
	ListTransactions(ctx context.Context) ([]domain.Transaction, error)
	GetReportSummary(ctx context.Context, from, to time.Time) ([]domain.ReportItem, error)
}

type service struct {
	budgets      domain.BudgetRepository
	expenses     domain.ExpenseRepository
	cache        domain.Cache
	transactionM sync.Mutex
}

func New(budgets domain.BudgetRepository, expenses domain.ExpenseRepository, cache domain.Cache) Service {
	return &service{budgets: budgets, expenses: expenses, cache: cache}
}

func (s *service) SetBudget(ctx context.Context, budget domain.Budget) (domain.Budget, error) {
	budget.Category = strings.TrimSpace(budget.Category)
	if err := budget.Validate(); err != nil {
		return domain.Budget{}, fmt.Errorf("invalid budget: %w", err)
	}
	created, err := s.budgets.Upsert(ctx, budget)
	if err == nil && s.cache != nil {
		_ = s.cache.Delete(ctx, "budgets:all")
	}
	return created, err
}

func (s *service) ListBudgets(ctx context.Context) ([]domain.Budget, error) {
	const key = "budgets:all"
	if s.cache != nil {
		if data, err := s.cache.Get(ctx, key); err == nil {
			var cached []domain.Budget
			if json.Unmarshal(data, &cached) == nil {
				return cached, nil
			}
		}
	}

	budgets, err := s.budgets.List(ctx)
	if err != nil {
		return nil, err
	}
	if s.cache != nil {
		if data, err := json.Marshal(budgets); err == nil {
			_ = s.cache.Set(ctx, key, data, 20*time.Second)
		}
	}
	return budgets, nil
}

func (s *service) AddTransaction(ctx context.Context, transaction domain.Transaction) (domain.Transaction, error) {
	transaction.Category = strings.TrimSpace(transaction.Category)
	if err := transaction.Validate(); err != nil {
		return domain.Transaction{}, fmt.Errorf("invalid transaction: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return domain.Transaction{}, err
	}

	// The budget check and insert must be one critical section. Otherwise two
	// workers could both observe the same spent amount and exceed the limit.
	s.transactionM.Lock()
	defer s.transactionM.Unlock()
	if err := ctx.Err(); err != nil {
		return domain.Transaction{}, err
	}

	budget, exists, err := s.budgets.GetByCategory(ctx, transaction.Category)
	if err != nil {
		return domain.Transaction{}, err
	}
	if exists {
		spent, err := s.expenses.SumByCategory(ctx, transaction.Category)
		if err != nil {
			return domain.Transaction{}, err
		}
		if spent+transaction.Amount > budget.Limit {
			return domain.Transaction{}, domain.ErrBudgetExceeded
		}
	}
	return s.expenses.Add(ctx, transaction)
}

func (s *service) BulkImport(
	ctx context.Context,
	transactions []domain.Transaction,
	workers int,
) (domain.ImportSummary, error) {
	summary := domain.ImportSummary{Errors: make([]domain.ImportError, 0)}
	if len(transactions) == 0 {
		return summary, ctx.Err()
	}
	if workers <= 0 {
		workers = 4
	}
	if workers > len(transactions) {
		workers = len(transactions)
	}

	type job struct {
		index       int
		transaction domain.Transaction
	}
	type result struct {
		index int
		err   error
	}

	jobs := make(chan job)
	results := make(chan result, len(transactions))
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case item, ok := <-jobs:
					if !ok {
						return
					}
					_, err := s.AddTransaction(ctx, item.transaction)
					results <- result{index: item.index, err: err}
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for index, transaction := range transactions {
			select {
			case <-ctx.Done():
				return
			case jobs <- job{index: index, transaction: transaction}:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	// The collector is the sole writer of the summary, so counters and the
	// error slice are race-free without sharing mutable state between workers.
	for item := range results {
		if item.err == nil {
			summary.Accepted++
			continue
		}
		summary.Rejected++
		summary.Errors = append(summary.Errors, domain.ImportError{
			Index: item.index,
			Error: item.err.Error(),
		})
	}
	sort.Slice(summary.Errors, func(i, j int) bool {
		return summary.Errors[i].Index < summary.Errors[j].Index
	})
	if err := ctx.Err(); err != nil {
		return summary, err
	}
	return summary, nil
}

func (s *service) ListTransactions(ctx context.Context) ([]domain.Transaction, error) {
	return s.expenses.List(ctx)
}

func (s *service) GetReportSummary(ctx context.Context, from, to time.Time) ([]domain.ReportItem, error) {
	key := fmt.Sprintf("report:summary:%s:%s", from.Format("2006-01-02"), to.Format("2006-01-02"))
	if s.cache != nil {
		if data, err := s.cache.Get(ctx, key); err == nil {
			var cached []domain.ReportItem
			if json.Unmarshal(data, &cached) == nil {
				return cached, nil
			}
		}
	}
	categories, err := s.expenses.Categories(ctx, from, to)
	if err != nil {
		return nil, err
	}

	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go reportHeartbeat(workCtx, 400*time.Millisecond)

	type categoryResult struct {
		item domain.ReportItem
		err  error
	}
	results := make(chan categoryResult, len(categories))
	var wg sync.WaitGroup
	for _, category := range categories {
		category := category
		wg.Add(1)
		go func() {
			defer wg.Done()
			total, err := s.expenses.SumByCategoryPeriod(workCtx, category, from, to)
			select {
			case results <- categoryResult{item: domain.ReportItem{Category: category, Total: total}, err: err}:
			case <-workCtx.Done():
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	result := make([]domain.ReportItem, 0, len(categories))
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case entry, ok := <-results:
			if !ok {
				sort.Slice(result, func(i, j int) bool {
					return result[i].Category < result[j].Category
				})
				goto completed
			}
			if entry.err != nil {
				return nil, entry.err
			}
			result = append(result, entry.item)
		}
	}

completed:
	if s.cache != nil {
		if data, err := json.Marshal(result); err == nil {
			_ = s.cache.Set(ctx, key, data, 30*time.Second)
		}
	}
	return result, nil
}

func reportHeartbeat(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer func() {
		ticker.Stop()
		log.Println("report calculation stopped")
	}()
	for {
		select {
		case <-ticker.C:
			log.Println("report calculation is in progress")
		case <-ctx.Done():
			return
		}
	}
}
