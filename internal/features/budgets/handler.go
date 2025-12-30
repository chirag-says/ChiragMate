package budgets

import (
	"net/http"
	"strconv"
	"time"

	"github.com/budgetmate/web/internal/database"
	"github.com/budgetmate/web/internal/middleware"
)

// BudgetRow represents a single budget category with spending info
type BudgetRow struct {
	Category   string
	Spent      float64
	Limit      float64
	Percentage float64
	Status     string // "safe", "warning", "danger"
}

// BudgetsData holds all data for the budgets page
type BudgetsData struct {
	Month            string
	MonthLabel       string
	Rows             []BudgetRow
	TotalSpent       float64
	TotalLimit       float64
	PurchaseRequests []database.PurchaseRequest
	CurrentUserID    int64
}

// Handler is the budget feature handler
type Handler struct{}

// NewHandler creates a new budget handler
func NewHandler() *Handler {
	return &Handler{}
}

// HandleIndex shows the budget overview page
func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Get current month or from query
	month := r.URL.Query().Get("month")
	if month == "" {
		month = time.Now().Format("2006-01")
	}

	data := h.getBudgetData(user.FamilyID, month)

	// Get purchase requests
	requests, _ := database.GetFamilyRequests(user.FamilyID, user.ID)
	data.PurchaseRequests = requests
	data.CurrentUserID = user.ID

	BudgetsPage(data).Render(r.Context(), w)
}

// HandleSave updates a budget for a category (HTMX)
func (h *Handler) HandleSave(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	category := r.FormValue("category")
	amountStr := r.FormValue("amount")
	month := r.FormValue("month")

	if category == "" || month == "" {
		http.Error(w, "Missing fields", http.StatusBadRequest)
		return
	}

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount < 0 {
		http.Error(w, "Invalid amount", http.StatusBadRequest)
		return
	}

	if err := database.SetBudget(user.FamilyID, category, month, amount); err != nil {
		http.Error(w, "Failed to save budget", http.StatusInternalServerError)
		return
	}

	// Return updated row
	data := h.getBudgetData(user.FamilyID, month)
	for _, row := range data.Rows {
		if row.Category == category {
			BudgetCard(row, month).Render(r.Context(), w)
			return
		}
	}

	// If category not found (new budget), return full list
	w.Header().Set("HX-Refresh", "true")
}

// HandleAddCategory handles adding a new budget category
func (h *Handler) HandleAddCategory(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	category := r.FormValue("category")
	amountStr := r.FormValue("amount")
	month := r.FormValue("month")

	if category == "" || month == "" {
		http.Error(w, "Missing fields", http.StatusBadRequest)
		return
	}

	amount, _ := strconv.ParseFloat(amountStr, 64)
	if amount <= 0 {
		amount = 5000 // Default budget
	}

	if err := database.SetBudget(user.FamilyID, category, month, amount); err != nil {
		http.Error(w, "Failed to create budget", http.StatusInternalServerError)
		return
	}

	// Refresh entire budgets grid
	w.Header().Set("HX-Refresh", "true")
}

func (h *Handler) getBudgetData(familyID int64, month string) BudgetsData {
	// Get spending by category for the month
	spending, _ := database.GetCategorySpendingForMonth(familyID, month)

	// Get budget limits
	limits, _ := database.GetMonthlyBudgets(familyID, month)

	// Get all categories (from spending and budgets combined)
	categories, _ := database.GetAllCategories(familyID)

	var rows []BudgetRow
	var totalSpent, totalLimit float64

	for _, cat := range categories {
		spent := spending[cat]
		limit := limits[cat]

		var pct float64
		var status string

		if limit > 0 {
			pct = (spent / limit) * 100
			if pct >= 100 {
				status = "danger"
			} else if pct >= 75 {
				status = "warning"
			} else {
				status = "safe"
			}
		} else {
			pct = 0
			status = "unset"
		}

		rows = append(rows, BudgetRow{
			Category:   cat,
			Spent:      spent,
			Limit:      limit,
			Percentage: pct,
			Status:     status,
		})

		totalSpent += spent
		totalLimit += limit
	}

	// Parse month for display
	t, _ := time.Parse("2006-01", month)
	monthLabel := t.Format("January 2006")

	return BudgetsData{
		Month:      month,
		MonthLabel: monthLabel,
		Rows:       rows,
		TotalSpent: totalSpent,
		TotalLimit: totalLimit,
	}
}

// HandleCreateRequest creates a new purchase request
func (h *Handler) HandleCreateRequest(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	itemName := r.FormValue("item_name")
	amountStr := r.FormValue("amount")

	if itemName == "" {
		http.Error(w, "Item name is required", http.StatusBadRequest)
		return
	}

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount <= 0 {
		http.Error(w, "Invalid amount", http.StatusBadRequest)
		return
	}

	_, err = database.CreatePurchaseRequest(user.FamilyID, user.ID, itemName, amount)
	if err != nil {
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	// Refresh the requests section
	requests, _ := database.GetFamilyRequests(user.FamilyID, user.ID)
	PurchaseRequestsSection(requests, user.ID).Render(r.Context(), w)
}

// HandleVote casts a vote on a purchase request
func (h *Handler) HandleVote(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	requestIDStr := r.FormValue("request_id")
	vote := r.FormValue("vote")

	requestID, err := strconv.ParseInt(requestIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid request ID", http.StatusBadRequest)
		return
	}

	if vote != "approve" && vote != "reject" {
		http.Error(w, "Invalid vote", http.StatusBadRequest)
		return
	}

	// Cast the vote
	if err := database.CastVote(requestID, user.ID, vote); err != nil {
		http.Error(w, "Failed to cast vote", http.StatusInternalServerError)
		return
	}

	// Get updated request
	req, err := database.GetPurchaseRequest(requestID, user.ID)
	if err != nil {
		http.Error(w, "Request not found", http.StatusNotFound)
		return
	}

	// Check if request should be auto-approved/rejected (majority vote)
	if req.TotalVoters > 0 {
		majority := (req.TotalVoters / 2) + 1
		if req.ApproveVotes >= majority {
			database.UpdateRequestStatus(requestID, "approved")
			req.Status = "approved"
		} else if req.RejectVotes >= majority {
			database.UpdateRequestStatus(requestID, "rejected")
			req.Status = "rejected"
		}
	}

	// Return updated card
	PurchaseRequestCard(*req, user.ID).Render(r.Context(), w)
}
