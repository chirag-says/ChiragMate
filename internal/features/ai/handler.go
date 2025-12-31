package ai

import (
	"encoding/json"
	"net/http"
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

// HandleChat (Future) - generic assistant endpoint
func (h *Handler) HandleChat(w http.ResponseWriter, r *http.Request) {
	// similar logic for free-form chat
}
