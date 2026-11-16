package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Glide28/go_hw_10/ledger/internal/domain"
)

type BudgetRepository struct{ db *sql.DB }
type ExpenseRepository struct{ db *sql.DB }

func NewBudgetRepository(db *sql.DB) *BudgetRepository   { return &BudgetRepository{db: db} }
func NewExpenseRepository(db *sql.DB) *ExpenseRepository { return &ExpenseRepository{db: db} }

func (r *BudgetRepository) Upsert(ctx context.Context, budget domain.Budget) (domain.Budget, error) {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO budgets(category, limit_amount)
		VALUES ($1, $2)
		ON CONFLICT(category) DO UPDATE SET limit_amount = EXCLUDED.limit_amount`,
		budget.Category, budget.Limit)
	if err != nil {
		return domain.Budget{}, fmt.Errorf("upsert budget: %w", err)
	}
	return budget, nil
}

func (r *BudgetRepository) GetByCategory(ctx context.Context, category string) (domain.Budget, bool, error) {
	var budget domain.Budget
	err := r.db.QueryRowContext(ctx,
		`SELECT category, limit_amount FROM budgets WHERE category = $1`, category,
	).Scan(&budget.Category, &budget.Limit)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Budget{}, false, nil
	}
	if err != nil {
		return domain.Budget{}, false, fmt.Errorf("get budget: %w", err)
	}
	return budget, true, nil
}

func (r *BudgetRepository) List(ctx context.Context) ([]domain.Budget, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT category, limit_amount FROM budgets ORDER BY category`)
	if err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Budget, 0)
	for rows.Next() {
		var budget domain.Budget
		if err := rows.Scan(&budget.Category, &budget.Limit); err != nil {
			return nil, fmt.Errorf("scan budget: %w", err)
		}
		result = append(result, budget)
	}
	return result, rows.Err()
}

func (r *ExpenseRepository) Add(ctx context.Context, transaction domain.Transaction) (domain.Transaction, error) {
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO expenses(amount, category, description, date)
		VALUES ($1, $2, $3, $4)
		RETURNING id`, transaction.Amount, transaction.Category, transaction.Description, transaction.Date,
	).Scan(&transaction.ID)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("add transaction: %w", err)
	}
	return transaction, nil
}

func (r *ExpenseRepository) List(ctx context.Context) ([]domain.Transaction, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, amount, category, description, date
		FROM expenses
		ORDER BY date DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Transaction, 0)
	for rows.Next() {
		var transaction domain.Transaction
		if err := rows.Scan(&transaction.ID, &transaction.Amount, &transaction.Category, &transaction.Description, &transaction.Date); err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		result = append(result, transaction)
	}
	return result, rows.Err()
}

func (r *ExpenseRepository) SumByCategory(ctx context.Context, category string) (float64, error) {
	var total float64
	if err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE category = $1`, category,
	).Scan(&total); err != nil {
		return 0, fmt.Errorf("sum expenses: %w", err)
	}
	return total, nil
}

func (r *ExpenseRepository) Categories(ctx context.Context, from, to time.Time) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT category
		FROM expenses
		WHERE date BETWEEN $1 AND $2
		ORDER BY category`, from, to)
	if err != nil {
		return nil, fmt.Errorf("list report categories: %w", err)
	}
	defer rows.Close()
	result := make([]string, 0)
	for rows.Next() {
		var category string
		if err := rows.Scan(&category); err != nil {
			return nil, fmt.Errorf("scan report category: %w", err)
		}
		result = append(result, category)
	}
	return result, rows.Err()
}

func (r *ExpenseRepository) SumByCategoryPeriod(
	ctx context.Context,
	category string,
	from, to time.Time,
) (float64, error) {
	var total float64
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount), 0)
		FROM expenses
		WHERE category = $1 AND date BETWEEN $2 AND $3`, category, from, to,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("sum report category %q: %w", category, err)
	}
	return total, nil
}
