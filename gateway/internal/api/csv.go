package api

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"time"

	"github.com/Glide28/go_hw_10/ledger"
)

func (s *Server) exportTransactionsCSV(w http.ResponseWriter, r *http.Request) {
	items, err := s.ledger.ListTransactions(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="transactions.csv"`)
	w.WriteHeader(http.StatusOK)
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"amount", "category", "description", "date"})
	for _, item := range items {
		_ = writer.Write([]string{
			strconv.FormatFloat(item.Amount, 'f', 2, 64),
			item.Category,
			item.Description,
			item.Date.Format("2006-01-02"),
		})
	}
	writer.Flush()
}

func (s *Server) importTransactionsCSV(w http.ResponseWriter, r *http.Request) {
	reader := csv.NewReader(r.Body)
	records, err := reader.ReadAll()
	if err != nil || len(records) < 2 {
		writeError(w, http.StatusBadRequest, "CSV must contain a header and at least one transaction")
		return
	}
	imported := 0
	errorsCount := 0
	for _, record := range records[1:] {
		if len(record) != 4 {
			errorsCount++
			continue
		}
		amount, amountErr := strconv.ParseFloat(record[0], 64)
		date, dateErr := time.Parse("2006-01-02", record[3])
		transaction := ledger.Transaction{Amount: amount, Category: record[1], Description: record[2], Date: date}
		if amountErr != nil || dateErr != nil || transaction.Validate() != nil {
			errorsCount++
			continue
		}
		if _, err := s.ledger.AddTransaction(r.Context(), transaction); err != nil {
			errorsCount++
			continue
		}
		imported++
	}
	writeJSON(w, http.StatusOK, map[string]int{"imported": imported, "errors": errorsCount})
}
