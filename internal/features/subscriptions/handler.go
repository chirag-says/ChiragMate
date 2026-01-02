package subscriptions

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/budgetmate/web/internal/database"
	"github.com/budgetmate/web/internal/middleware"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

// SubscriptionWithDue extends Subscription with calculated due date info
type SubscriptionWithDue struct {
	database.Subscription
	NextDueDate time.Time
	DaysUntil   int
	DueStatus   string // "overdue", "due-today", "due-soon", "upcoming"
}

// SubscriptionsData holds all data for the subscriptions page
type SubscriptionsData struct {
	Subscriptions     []SubscriptionWithDue
	PotentialSubs     []database.PotentialSubscription
	TotalMonthlyBurn  float64
	SubscriptionCount int
}

// HandleIndex displays the subscriptions page
func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Fetch active subscriptions
	subs, err := database.GetSubscriptions(user.FamilyID)
	if err != nil {
		subs = []database.Subscription{}
	}

	// Fetch potential subscriptions (auto-detected)
	potentials, err := database.DetectPotentialSubscriptions(user.FamilyID)
	if err != nil {
		potentials = []database.PotentialSubscription{}
	}

	// Filter out already-added subscriptions from potentials
	existingNames := make(map[string]bool)
	for _, s := range subs {
		existingNames[strings.ToLower(s.Name)] = true
	}

	var filteredPotentials []database.PotentialSubscription
	for _, p := range potentials {
		if !existingNames[strings.ToLower(p.Name)] {
			filteredPotentials = append(filteredPotentials, p)
		}
	}

	// Calculate next due dates and total monthly burn
	subsWithDue := calculateDueDates(subs)
	totalBurn := calculateMonthlyBurn(subs)

	data := SubscriptionsData{
		Subscriptions:     subsWithDue,
		PotentialSubs:     filteredPotentials,
		TotalMonthlyBurn:  totalBurn,
		SubscriptionCount: len(subs),
	}

	SubscriptionsPage(data).Render(r.Context(), w)
}

// HandleAdd adds a new subscription manually
func (h *Handler) HandleAdd(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	amountStr := r.FormValue("amount")
	billingDayStr := r.FormValue("billing_day")
	category := r.FormValue("category")

	if name == "" || amountStr == "" || billingDayStr == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount <= 0 {
		http.Error(w, "Invalid amount", http.StatusBadRequest)
		return
	}

	billingDay, err := strconv.Atoi(billingDayStr)
	if err != nil || billingDay < 1 || billingDay > 31 {
		http.Error(w, "Invalid billing day", http.StatusBadRequest)
		return
	}

	if category == "" {
		category = "Subscriptions"
	}

	err = database.CreateSubscription(user.FamilyID, name, amount, billingDay, category)
	if err != nil {
		http.Error(w, "Failed to add subscription", http.StatusInternalServerError)
		return
	}

	// Return updated subscription list
	h.renderSubscriptionsList(w, r, user.FamilyID)
}

// HandleConvert converts a detected potential subscription to an actual subscription
func (h *Handler) HandleConvert(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	amountStr := r.FormValue("amount")
	billingDayStr := r.FormValue("billing_day")
	category := r.FormValue("category")

	if name == "" || amountStr == "" || billingDayStr == "" {
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		http.Error(w, "Invalid amount", http.StatusBadRequest)
		return
	}

	billingDay, err := strconv.Atoi(billingDayStr)
	if err != nil || billingDay < 1 || billingDay > 31 {
		http.Error(w, "Invalid billing day", http.StatusBadRequest)
		return
	}

	if category == "" {
		category = "Subscriptions"
	}

	// Check if already exists
	existing, _ := database.GetSubscriptionByName(user.FamilyID, name)
	if existing != nil {
		http.Error(w, "Subscription already exists", http.StatusConflict)
		return
	}

	err = database.CreateSubscription(user.FamilyID, name, amount, billingDay, category)
	if err != nil {
		http.Error(w, "Failed to add subscription", http.StatusInternalServerError)
		return
	}

	// Redirect back to subscriptions page
	http.Redirect(w, r, "/app/subscriptions", http.StatusSeeOther)
}

// HandleDelete removes a subscription
func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "Subscription ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	err = database.DeleteSubscription(id)
	if err != nil {
		http.Error(w, "Failed to delete subscription", http.StatusInternalServerError)
		return
	}

	// Return updated list
	h.renderSubscriptionsList(w, r, user.FamilyID)
}

// Helper: renders just the subscription list partial
func (h *Handler) renderSubscriptionsList(w http.ResponseWriter, r *http.Request, familyID int64) {
	subs, _ := database.GetSubscriptions(familyID)
	subsWithDue := calculateDueDates(subs)
	totalBurn := calculateMonthlyBurn(subs)

	SubscriptionsList(subsWithDue, totalBurn).Render(r.Context(), w)
}

// calculateDueDates calculates next due date for each subscription
func calculateDueDates(subs []database.Subscription) []SubscriptionWithDue {
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var result []SubscriptionWithDue
	for _, s := range subs {
		sub := SubscriptionWithDue{Subscription: s}

		// Calculate next due date
		billingDay := s.BillingDay

		// Handle months with fewer days
		monthDays := getDaysInMonth(now.Year(), now.Month())
		if billingDay > monthDays {
			billingDay = monthDays
		}

		nextDue := time.Date(now.Year(), now.Month(), billingDay, 0, 0, 0, 0, now.Location())

		// If billing day has passed this month, move to next month
		if nextDue.Before(today) {
			nextMonth := now.Month() + 1
			nextYear := now.Year()
			if nextMonth > 12 {
				nextMonth = 1
				nextYear++
			}
			nextMonthDays := getDaysInMonth(nextYear, nextMonth)
			billingDay = s.BillingDay
			if billingDay > nextMonthDays {
				billingDay = nextMonthDays
			}
			nextDue = time.Date(nextYear, nextMonth, billingDay, 0, 0, 0, 0, now.Location())
		}

		sub.NextDueDate = nextDue
		sub.DaysUntil = int(nextDue.Sub(today).Hours() / 24)

		// Determine status
		switch {
		case sub.DaysUntil < 0:
			sub.DueStatus = "overdue"
		case sub.DaysUntil == 0:
			sub.DueStatus = "due-today"
		case sub.DaysUntil <= 3:
			sub.DueStatus = "due-soon"
		default:
			sub.DueStatus = "upcoming"
		}

		result = append(result, sub)
	}

	// Sort by days until due
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].DaysUntil < result[i].DaysUntil {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// calculateMonthlyBurn sums all active subscription amounts
func calculateMonthlyBurn(subs []database.Subscription) float64 {
	var total float64
	for _, s := range subs {
		total += s.Amount
	}
	return total
}

// getDaysInMonth returns the number of days in a given month
func getDaysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
