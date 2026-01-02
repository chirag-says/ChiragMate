package ai

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/budgetmate/web/internal/database"
	"github.com/budgetmate/web/internal/middleware"
)

type Handler struct {
	Service *Service
}

func NewHandler() *Handler {
	return &Handler{
		Service: NewService(),
	}
}

type CategorizeRequest struct {
	Description string `json:"description"`
}

type CategorizeResponse struct {
	Category string `json:"category"`
	Model    string `json:"model"`
}

// HandleCategorize processes the categorization request
func (h *Handler) HandleCategorize(w http.ResponseWriter, r *http.Request) {
	var req CategorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	category, err := h.Service.CategorizeTransaction(req.Description)
	if err != nil {
		// Log the error (in a real app) -> defaulting to basic logic or error
		// For now, we return 503 so the frontend knows AI is offline
		http.Error(w, "AI Service Unavailable: "+err.Error(), http.StatusServiceUnavailable)
		return
	}

	resp := CategorizeResponse{
		Category: category,
		Model:    "Hybrid (Groq/Rules)",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleShowChat renders the AI Advisor chat page
func (h *Handler) HandleShowChat(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	ChatPage().Render(r.Context(), w)
}

// HandleChat processes chat messages and returns AI responses
func (h *Handler) HandleChat(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get the user's message from form
	message := strings.TrimSpace(r.FormValue("message"))
	if message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}

	// Get conversation history from form (JSON array)
	historyJSON := r.FormValue("history")
	var conversationHistory []ChatMessage
	if historyJSON != "" {
		json.Unmarshal([]byte(historyJSON), &conversationHistory)
	}

	// Fetch last 30 days of transactions
	transactions, err := database.GetRecentTransactionsForDays(user.FamilyID, 30)
	if err != nil {
		// Continue with empty history if error
		transactions = []database.Transaction{}
	}

	// Format transactions as CSV-like string for AI context
	var historyBuilder strings.Builder
	historyBuilder.WriteString("Date,Amount,Type,Category,Description\n")
	for _, t := range transactions {
		amount := t.Amount
		if t.Type == "expense" {
			amount = -amount
		}
		historyBuilder.WriteString(fmt.Sprintf("%s,%.2f,%s,%s,%s\n",
			t.Date.Format("2006-01-02"),
			amount,
			t.Type,
			t.Category,
			t.Description,
		))
	}

	history := historyBuilder.String()

	// If no transactions, add a note
	if len(transactions) == 0 {
		history = "No transactions in the last 30 days."
	}

	// Get AI response with conversation history
	response, err := h.Service.GenerateFinancialAdvice(history, message, conversationHistory)
	if err != nil {
		response = "I apologize, but I'm having trouble processing your request right now. Please try again later."
	}

	// Return both user message and AI response as chat bubbles
	ChatMessages(message, response).Render(r.Context(), w)
}
