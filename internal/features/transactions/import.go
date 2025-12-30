package transactions

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/budgetmate/web/internal/database"
)

// HandleShowImportForm returns the import form (HTMX partial)
func (h *Handler) HandleShowImportForm(w http.ResponseWriter, r *http.Request) {
	ImportForm().Render(r.Context(), w)
}

// HandleHideImportForm returns an empty response to hide the form
func (h *Handler) HandleHideImportForm(w http.ResponseWriter, r *http.Request) {
	ImportButton().Render(r.Context(), w)
}

// HandleImport processes the CSV file upload
func (h *Handler) HandleImport(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		ImportResult(false, "Failed to parse form: "+err.Error(), 0).Render(r.Context(), w)
		return
	}

	// Get the uploaded file
	file, header, err := r.FormFile("csvfile")
	if err != nil {
		ImportResult(false, "No file uploaded: "+err.Error(), 0).Render(r.Context(), w)
		return
	}
	defer file.Close()

	// Validate file extension
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".csv") {
		ImportResult(false, "Please upload a .csv file", 0).Render(r.Context(), w)
		return
	}

	// Parse CSV
	transactions, errors := parseCSV(file)
	if len(transactions) == 0 {
		errMsg := "No valid transactions found in CSV"
		if len(errors) > 0 {
			errMsg = fmt.Sprintf("No valid transactions. Errors: %s", strings.Join(errors[:min(3, len(errors))], "; "))
		}
		ImportResult(false, errMsg, 0).Render(r.Context(), w)
		return
	}

	// Bulk insert
	inserted, err := database.BulkInsertTransactions(transactions)
	if err != nil {
		ImportResult(false, "Database error: "+err.Error(), 0).Render(r.Context(), w)
		return
	}

	// Success - return success message with warnings if any
	msg := fmt.Sprintf("Successfully imported %d transactions", inserted)
	if len(errors) > 0 {
		msg += fmt.Sprintf(" (%d rows skipped)", len(errors))
	}

	// Return success result with refresh trigger
	ImportResultWithRefresh(true, msg, inserted).Render(r.Context(), w)
}

// parseCSV reads and validates CSV data
// Expected columns: date, description, category, amount, type
func parseCSV(file io.Reader) ([]database.Transaction, []string) {
	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1 // Allow variable fields

	var transactions []database.Transaction
	var errors []string

	lineNum := 0
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		lineNum++

		// Skip header row
		if lineNum == 1 {
			// Check if this looks like a header
			if len(record) > 0 && strings.ToLower(strings.TrimSpace(record[0])) == "date" {
				continue
			}
		}

		if err != nil {
			errors = append(errors, fmt.Sprintf("Line %d: %v", lineNum, err))
			continue
		}

		// Validate and parse row
		t, parseErr := parseRow(record, lineNum)
		if parseErr != nil {
			errors = append(errors, parseErr.Error())
			continue
		}

		transactions = append(transactions, *t)
	}

	return transactions, errors
}

// parseRow converts a CSV row to a Transaction
// Expected format: date, description, category, amount, type
func parseRow(record []string, lineNum int) (*database.Transaction, error) {
	if len(record) < 5 {
		return nil, fmt.Errorf("Line %d: expected 5 columns, got %d", lineNum, len(record))
	}

	// Parse date (YYYY-MM-DD)
	dateStr := strings.TrimSpace(record[0])
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		// Try alternative formats
		date, err = time.Parse("02-01-2006", dateStr)
		if err != nil {
			date, err = time.Parse("02/01/2006", dateStr)
			if err != nil {
				return nil, fmt.Errorf("Line %d: invalid date '%s' (expected YYYY-MM-DD)", lineNum, dateStr)
			}
		}
	}

	// Parse description
	description := strings.TrimSpace(record[1])
	if description == "" {
		return nil, fmt.Errorf("Line %d: description cannot be empty", lineNum)
	}

	// Parse category
	category := strings.TrimSpace(record[2])
	if category == "" {
		category = "Uncategorized"
	}

	// Parse amount
	amountStr := strings.TrimSpace(record[3])
	// Remove currency symbols and commas
	amountStr = strings.ReplaceAll(amountStr, "₹", "")
	amountStr = strings.ReplaceAll(amountStr, ",", "")
	amountStr = strings.ReplaceAll(amountStr, " ", "")
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return nil, fmt.Errorf("Line %d: invalid amount '%s'", lineNum, record[3])
	}
	if amount < 0 {
		amount = -amount // Make positive, type determines direction
	}

	// Parse type
	txType := strings.ToLower(strings.TrimSpace(record[4]))
	if txType != "income" && txType != "expense" {
		// Try to infer from common variations
		switch txType {
		case "credit", "cr", "in", "+":
			txType = "income"
		case "debit", "dr", "out", "-":
			txType = "expense"
		default:
			return nil, fmt.Errorf("Line %d: type must be 'income' or 'expense', got '%s'", lineNum, record[4])
		}
	}

	return &database.Transaction{
		Date:        date,
		Description: description,
		Category:    category,
		Amount:      amount,
		Type:        txType,
	}, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
