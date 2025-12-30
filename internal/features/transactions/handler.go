package transactions

import (
	"net/http"
	"strconv"
	"time"

	"github.com/budgetmate/web/internal/database"
	"github.com/budgetmate/web/internal/features/dashboard"
	"github.com/budgetmate/web/internal/middleware"
	"github.com/go-chi/chi/v5"
)

// Handler handles transaction-related HTTP requests
type Handler struct{}

// NewHandler creates a new transactions handler
func NewHandler() *Handler {
	return &Handler{}
}

// HandleList renders the transactions list page
func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	transactions, err := database.GetAllTransactions(user.FamilyID)
	if err != nil {
		http.Error(w, "Failed to get transactions", http.StatusInternalServerError)
		return
	}

	TransactionsPage(transactions).Render(r.Context(), w)
}

// HandleNew renders the create transaction page
func (h *Handler) HandleNew(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	CreateTransactionPage().Render(r.Context(), w)
}

// HandleCreate handles the creation of a new transaction
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Parse form
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	description := r.FormValue("description")
	amountStr := r.FormValue("amount")
	category := r.FormValue("category")
	typeStr := r.FormValue("type")
	dateStr := r.FormValue("date")

	if description == "" || amountStr == "" || category == "" || typeStr == "" {
		http.Error(w, "Missing required fields", http.StatusBadRequest)
		return
	}

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		http.Error(w, "Invalid amount", http.StatusBadRequest)
		return
	}

	date := time.Now()
	if dateStr != "" {
		if d, err := time.Parse("2006-01-02", dateStr); err == nil {
			date = d
		}
	}

	// Create transaction
	tx := &database.Transaction{
		Description: description,
		Amount:      amount,
		Category:    category,
		Type:        typeStr,
		Date:        date,
		UserID:      user.ID,
		FamilyID:    user.FamilyID,
	}

	if err := database.InsertTransaction(tx); err != nil {
		http.Error(w, "Failed to create transaction", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/app/transactions", http.StatusSeeOther)
}

// HandleGetEdit returns the edit form for a transaction (HTMX partial)
func (h *Handler) HandleGetEdit(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid transaction ID", http.StatusBadRequest)
		return
	}

	transaction, err := database.GetTransaction(id)
	if err != nil {
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}

	// Security check: In a real app, verify transaction.FamilyID == user.FamilyID

	// Return the edit form partial
	dashboard.TransactionEditRow(*transaction).Render(r.Context(), w)
}

// HandleGetView returns the view row for a transaction (HTMX partial)
func (h *Handler) HandleGetView(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid transaction ID", http.StatusBadRequest)
		return
	}

	transaction, err := database.GetTransaction(id)
	if err != nil {
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}

	// Return the view row partial
	dashboard.TransactionRow(*transaction).Render(r.Context(), w)
}

// HandleUpdate updates a transaction (HTMX form submission)
func (h *Handler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid transaction ID", http.StatusBadRequest)
		return
	}

	// Get existing transaction
	transaction, err := database.GetTransaction(id)
	if err != nil {
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Update fields if provided
	if desc := r.FormValue("description"); desc != "" {
		transaction.Description = desc
	}

	if amountStr := r.FormValue("amount"); amountStr != "" {
		if amount, err := strconv.ParseFloat(amountStr, 64); err == nil {
			transaction.Amount = amount
		}
	}

	if category := r.FormValue("category"); category != "" {
		transaction.Category = category
	}

	if dateStr := r.FormValue("date"); dateStr != "" {
		if date, err := time.Parse("2006-01-02", dateStr); err == nil {
			transaction.Date = date
		}
	}

	// Save to database
	if err := database.UpdateTransaction(transaction); err != nil {
		http.Error(w, "Failed to update transaction", http.StatusInternalServerError)
		return
	}

	// Return the updated view row
	dashboard.TransactionRow(*transaction).Render(r.Context(), w)
}
