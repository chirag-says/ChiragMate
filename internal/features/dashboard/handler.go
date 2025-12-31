package dashboard

import (
	"net/http"

	"github.com/budgetmate/web/internal/database"
	"github.com/budgetmate/web/internal/middleware"
	"github.com/budgetmate/web/internal/shared/components"
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
	UserName          string  // Personalized greeting
}

// HandleIndex renders the main dashboard page
func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Optimize: Fetch ALL transactions once (1 DB Call) and compute stats in-memory
	// This reduces 6 DB round-trips to just 1, significantly improving speed on Cloud DBs.
	allTransactions, err := database.GetAllTransactions(user.FamilyID)
	if err != nil {
		http.Error(w, "Failed to get transactions", http.StatusInternalServerError)
		return
	}

	var balance, income, expenses float64
	categoryBreakdown := make(map[string]float64)

	// Compute stats
	for _, t := range allTransactions {
		if t.Type == "income" {
			income += t.Amount
		} else { // expense
			expenses += t.Amount
			categoryBreakdown[t.Category] += t.Amount
		}
	}
	balance = income - expenses

	// Get recent 5 transactions (DB already sorts by Date DESC)
	var recentTransactions []database.Transaction
	if len(allTransactions) > 5 {
		recentTransactions = allTransactions[:5]
	} else {
		recentTransactions = allTransactions
	}

	// Generate Calm AI insight
	insight := GenerateInsight(allTransactions)

	// Build dashboard data
	data := DashboardData{
		Balance:           balance,
		TotalIncome:       income,
		TotalExpenses:     expenses,
		Transactions:      recentTransactions,
		CategoryBreakdown: categoryBreakdown,
		Insight:           insight,
		UserName:          user.Name, // Pass User Name
	}

	// Render the dashboard
	DashboardPage(data).Render(r.Context(), w)
}

func (h *Handler) HandleNotifications(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		return
	}

	notifications, err := database.GetUnreadNotifications(user.ID)
	if err != nil {
		// Return empty or error view
		return
	}

	components.NotificationList(notifications).Render(r.Context(), w)
}
