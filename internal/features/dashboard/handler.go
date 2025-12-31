package dashboard

import (
	"context"
	"net/http"

	"github.com/budgetmate/web/internal/database"
	"github.com/budgetmate/web/internal/middleware"
	"github.com/budgetmate/web/internal/shared/components"
	"golang.org/x/sync/errgroup"
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

// HandleIndex renders the main dashboard page using parallel SQL aggregations
// Optimized for high-latency cloud environments (Railway + Turso)
func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	familyID := user.FamilyID

	// Results containers - use pointers/values that are safe for concurrent access
	var (
		recentTransactions []database.Transaction
		totalIncome        float64
		totalExpenses      float64
		categoryBreakdown  map[string]float64
		insightTxns        []database.Transaction // For insight generation (last 30 days)
	)

	// Create errgroup with context for parallel execution
	g, ctx := errgroup.WithContext(r.Context())

	// G1: Fetch Recent Transactions (Limit 5) - SQL LIMIT
	g.Go(func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		txns, err := database.GetRecentTransactions(familyID, 5)
		if err != nil {
			return err
		}
		recentTransactions = txns
		return nil
	})

	// G2: Fetch Total Income - SQL SUM aggregation
	g.Go(func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		income, err := database.GetTotalIncome(familyID)
		if err != nil {
			return err
		}
		totalIncome = income
		return nil
	})

	// G3: Fetch Total Expenses - SQL SUM aggregation
	g.Go(func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		expenses, err := database.GetTotalExpenses(familyID)
		if err != nil {
			return err
		}
		totalExpenses = expenses
		return nil
	})

	// G4: Fetch Category Breakdown - SQL GROUP BY aggregation
	g.Go(func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		breakdown, err := database.GetCategoryBreakdown(familyID)
		if err != nil {
			return err
		}
		categoryBreakdown = breakdown
		return nil
	})

	// G5: Fetch last 30 days transactions for insight generation
	// This is a lightweight query with time filter
	g.Go(func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		txns, err := database.GetRecentTransactionsForDays(familyID, 30)
		if err != nil {
			// Non-fatal: insights are optional, use empty slice
			insightTxns = []database.Transaction{}
			return nil
		}
		insightTxns = txns
		return nil
	})

	// Wait for all goroutines to complete
	if err := g.Wait(); err != nil {
		// Log error but try to render with available data
		// In production, you might want more sophisticated error handling
		http.Error(w, "Failed to load dashboard data", http.StatusInternalServerError)
		return
	}

	// Calculate balance from aggregated values
	balance := totalIncome - totalExpenses

	// Generate insight from the last 30 days transactions
	insight := GenerateInsight(insightTxns)

	// Assemble dashboard data from parallel results
	data := DashboardData{
		Balance:           balance,
		TotalIncome:       totalIncome,
		TotalExpenses:     totalExpenses,
		Transactions:      recentTransactions,
		CategoryBreakdown: categoryBreakdown,
		Insight:           insight,
		UserName:          user.Name,
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

// Ensure context is used (silence unused import if needed)
var _ = context.Background
