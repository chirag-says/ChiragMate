package landing

import (
	"net/http"
)

// Handler handles landing page HTTP requests
type Handler struct{}

// NewHandler creates a new landing handler
func NewHandler() *Handler {
	return &Handler{}
}

// HandleIndex renders the public landing page
func (h *Handler) HandleIndex(w http.ResponseWriter, r *http.Request) {
	LandingPage().Render(r.Context(), w)
}
