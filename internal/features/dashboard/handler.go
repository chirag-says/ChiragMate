package dashboard

import (
	"net/http"

	"github.com/budgetmate/web/internal/database"
)

// Handler handles dashboard-related HTTP requests
type Handler struct{}

// NewHandler creates a new dashboard handler
func NewHandler() *Handler {
	return &Handler{}
}

// HandleIndex renders the main dashboard page
func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	// Get total balance
	balance, err := database.GetTotalBalance()
	if err != nil {
		http.Error(w, "Failed to get balance", http.StatusInternalServerError)
		return
	}

	// Get recent transactions
	transactions, err := database.GetRecentTransactions(5)
	if err != nil {
		http.Error(w, "Failed to get transactions", http.StatusInternalServerError)
		return
	}

	// Render the dashboard
	DashboardPage(balance, transactions).Render(r.Context(), w)
}
