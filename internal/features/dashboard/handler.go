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

// DashboardData contains all data needed for the dashboard view
type DashboardData struct {
	Balance           float64
	TotalIncome       float64
	TotalExpenses     float64
	Transactions      []database.Transaction
	CategoryBreakdown map[string]float64
	Insight           Insight // Calm AI nudge
}

// HandleIndex renders the main dashboard page
func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	// Get total balance
	balance, err := database.GetTotalBalance()
	if err != nil {
		http.Error(w, "Failed to get balance", http.StatusInternalServerError)
		return
	}

	// Get total income
	income, err := database.GetTotalIncome()
	if err != nil {
		http.Error(w, "Failed to get income", http.StatusInternalServerError)
		return
	}

	// Get total expenses
	expenses, err := database.GetTotalExpenses()
	if err != nil {
		http.Error(w, "Failed to get expenses", http.StatusInternalServerError)
		return
	}

	// Get recent transactions
	transactions, err := database.GetRecentTransactions(5)
	if err != nil {
		http.Error(w, "Failed to get transactions", http.StatusInternalServerError)
		return
	}

	// Get ALL transactions for insight analysis
	allTransactions, err := database.GetAllTransactions()
	if err != nil {
		http.Error(w, "Failed to get all transactions", http.StatusInternalServerError)
		return
	}

	// Get category breakdown for chart
	categoryBreakdown, err := database.GetCategoryBreakdown()
	if err != nil {
		http.Error(w, "Failed to get category breakdown", http.StatusInternalServerError)
		return
	}

	// Generate Calm AI insight
	insight := GenerateInsight(allTransactions)

	// Build dashboard data
	data := DashboardData{
		Balance:           balance,
		TotalIncome:       income,
		TotalExpenses:     expenses,
		Transactions:      transactions,
		CategoryBreakdown: categoryBreakdown,
		Insight:           insight,
	}

	// Render the dashboard
	DashboardPage(data).Render(r.Context(), w)
}
