package goals

import (
	"net/http"
	"strconv"
	"time"

	"github.com/budgetmate/web/internal/database"
	"github.com/budgetmate/web/internal/middleware"
	"github.com/go-chi/chi/v5"
)

// GoalsData holds all data for the goals page
type GoalsData struct {
	Goals       []database.Goal
	TotalSaved  float64
	TotalTarget float64
}

// Handler is the goals feature handler
type Handler struct{}

// NewHandler creates a new goals handler
func NewHandler() *Handler {
	return &Handler{}
}

// HandleIndex shows the goals overview page
func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	goals, _ := database.GetFamilyGoals(user.FamilyID)

	var totalSaved, totalTarget float64
	for _, g := range goals {
		totalSaved += g.CurrentAmount
		totalTarget += g.TargetAmount
	}

	data := GoalsData{
		Goals:       goals,
		TotalSaved:  totalSaved,
		TotalTarget: totalTarget,
	}

	GoalsPage(data).Render(r.Context(), w)
}

// HandleCreate creates a new goal
func (h *Handler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	name := r.FormValue("name")
	targetStr := r.FormValue("target_amount")
	icon := r.FormValue("icon")
	color := r.FormValue("color")
	deadlineStr := r.FormValue("deadline")

	if name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}

	target, err := strconv.ParseFloat(targetStr, 64)
	if err != nil || target <= 0 {
		http.Error(w, "Invalid target amount", http.StatusBadRequest)
		return
	}

	var deadline *time.Time
	if deadlineStr != "" {
		t, err := time.Parse("2006-01-02", deadlineStr)
		if err == nil {
			deadline = &t
		}
	}

	_, err = database.CreateGoal(user.FamilyID, name, target, icon, color, deadline)
	if err != nil {
		http.Error(w, "Failed to create goal", http.StatusInternalServerError)
		return
	}

	// Refresh page
	w.Header().Set("HX-Refresh", "true")
}

// HandleContribute adds funds to a goal
func (h *Handler) HandleContribute(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	goalIDStr := r.FormValue("goal_id")
	amountStr := r.FormValue("amount")

	goalID, err := strconv.ParseInt(goalIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid goal ID", http.StatusBadRequest)
		return
	}

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount <= 0 {
		http.Error(w, "Invalid amount", http.StatusBadRequest)
		return
	}

	// Verify goal belongs to user's family
	goal, err := database.GetGoalByID(goalID)
	if err != nil || goal.FamilyID != user.FamilyID {
		http.Error(w, "Goal not found", http.StatusNotFound)
		return
	}

	if err := database.ContributeToGoal(goalID, amount); err != nil {
		http.Error(w, "Failed to contribute", http.StatusInternalServerError)
		return
	}

	// Return updated goal card
	updatedGoal, _ := database.GetGoalByID(goalID)
	if updatedGoal != nil {
		GoalCard(*updatedGoal).Render(r.Context(), w)
	}
}

// HandleDelete deletes a goal
func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	goalIDStr := chi.URLParam(r, "id")
	goalID, err := strconv.ParseInt(goalIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid goal ID", http.StatusBadRequest)
		return
	}

	// Verify goal belongs to user's family
	goal, err := database.GetGoalByID(goalID)
	if err != nil || goal.FamilyID != user.FamilyID {
		http.Error(w, "Goal not found", http.StatusNotFound)
		return
	}

	if err := database.DeleteGoal(goalID); err != nil {
		http.Error(w, "Failed to delete goal", http.StatusInternalServerError)
		return
	}

	// Return empty (card removed via HTMX)
	w.WriteHeader(http.StatusOK)
}
