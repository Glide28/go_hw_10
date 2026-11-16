package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Glide28/go_hw_10/ledger"
)

type Server struct {
	ledger ledger.Service
	auth   *authService
}

func NewHandler(service ledger.Service) http.Handler {
	timeout := 2 * time.Second
	if configured := os.Getenv("REQUEST_TIMEOUT"); configured != "" {
		if parsed, err := time.ParseDuration(configured); err == nil && parsed > 0 {
			timeout = parsed
		}
	}
	return NewHandlerWithTimeout(service, timeout)
}

func NewHandlerWithTimeout(service ledger.Service, timeout time.Duration) http.Handler {
	s := &Server{ledger: service, auth: newAuthService(os.Getenv("JWT_SECRET"))}
	protected := http.NewServeMux()
	protected.HandleFunc("POST /api/transactions", s.createTransaction)
	protected.HandleFunc("POST /api/transactions/bulk", s.bulkImportTransactions)
	protected.HandleFunc("GET /api/transactions", s.listTransactions)
	protected.HandleFunc("GET /api/transactions/export.csv", s.exportTransactionsCSV)
	protected.HandleFunc("POST /api/transactions/import.csv", s.importTransactionsCSV)
	protected.HandleFunc("POST /api/budgets", s.createBudget)
	protected.HandleFunc("GET /api/budgets", s.listBudgets)
	protected.HandleFunc("GET /api/reports/summary", s.reportSummary)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/auth/register", s.register)
	mux.HandleFunc("POST /api/auth/login", s.login)
	mux.Handle("/api/", s.auth.middleware(protected))
	return loggingMiddleware(jsonMiddleware(timeoutMiddleware(timeout, mux)))
}

func (s *Server) createTransaction(w http.ResponseWriter, r *http.Request) {
	var request CreateTransactionRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	tx, err := request.toDomain()
	if err != nil {
		writeError(w, http.StatusBadRequest, "date must use YYYY-MM-DD format")
		return
	}
	if err := tx.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := s.ledger.AddTransaction(r.Context(), tx)
	if errors.Is(err, ledger.ErrBudgetExceeded) {
		writeError(w, http.StatusConflict, "budget exceeded")
		return
	}
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, transactionResponse(created))
}

func (s *Server) bulkImportTransactions(w http.ResponseWriter, r *http.Request) {
	var requests []CreateTransactionRequest
	if err := decodeJSON(r, &requests); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	workers := 4
	if raw := r.URL.Query().Get("workers"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 64 {
			writeError(w, http.StatusBadRequest, "workers must be an integer from 1 to 64")
			return
		}
		workers = parsed
	}

	transactions := make([]ledger.Transaction, 0, len(requests))
	for _, request := range requests {
		transactions = append(transactions, request.toDomainForBulk())
	}
	summary, err := s.ledger.BulkImport(r.Context(), transactions, workers)
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		writeJSON(w, http.StatusGatewayTimeout, map[string]any{
			"error":   "request timed out",
			"summary": summary,
		})
		return
	}
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, summary)
}

func (s *Server) listTransactions(w http.ResponseWriter, r *http.Request) {
	items, err := s.ledger.ListTransactions(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response := make([]TransactionResponse, 0, len(items))
	for _, tx := range items {
		response = append(response, transactionResponse(tx))
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) createBudget(w http.ResponseWriter, r *http.Request) {
	var request CreateBudgetRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	b := ledger.Budget{Category: request.Category, Limit: request.Limit}
	if err := b.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	created, err := s.ledger.SetBudget(r.Context(), b)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, budgetResponse(created))
}

func (s *Server) listBudgets(w http.ResponseWriter, r *http.Request) {
	items, err := s.ledger.ListBudgets(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response := make([]BudgetResponse, 0, len(items))
	for _, b := range items {
		response = append(response, budgetResponse(b))
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) reportSummary(w http.ResponseWriter, r *http.Request) {
	from, err := time.Parse("2006-01-02", r.URL.Query().Get("from"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "from must use YYYY-MM-DD format")
		return
	}
	to, err := time.Parse("2006-01-02", r.URL.Query().Get("to"))
	if err != nil || to.Before(from) {
		writeError(w, http.StatusBadRequest, "to must use YYYY-MM-DD format and not precede from")
		return
	}
	items, err := s.ledger.GetReportSummary(r.Context(), from, to)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func decodeJSON(r *http.Request, target any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("failed to encode response: %v", err)
	}
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
func writeServiceError(w http.ResponseWriter, err error) {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		writeError(w, http.StatusGatewayTimeout, "request timed out")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal server error")
}
func timeoutMiddleware(timeout time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func jsonMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		next.ServeHTTP(w, r)
	})
}
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("method=%s path=%s duration=%s", r.Method, r.URL.Path, time.Since(started))
	})
}
