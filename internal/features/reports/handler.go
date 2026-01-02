package reports

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/budgetmate/web/internal/database"
	"github.com/budgetmate/web/internal/middleware"
)

// Handler handles report-related HTTP requests
type Handler struct {
	Service *Service
}

// NewHandler creates a new reports handler
func NewHandler() *Handler {
	return &Handler{
		Service: NewService(),
	}
}

// HandleDownload generates and serves a PDF report
func (h *Handler) HandleDownload(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get month and year from query params (default to current month)
	now := time.Now()
	year := now.Year()
	month := now.Month()

	if yearStr := r.URL.Query().Get("year"); yearStr != "" {
		if y, err := strconv.Atoi(yearStr); err == nil && y > 2000 && y < 3000 {
			year = y
		}
	}

	if monthStr := r.URL.Query().Get("month"); monthStr != "" {
		if m, err := strconv.Atoi(monthStr); err == nil && m >= 1 && m <= 12 {
			month = time.Month(m)
		}
	}

	// Fetch report data
	data, err := database.GetMonthlyReportData(user.FamilyID, year, month)
	if err != nil {
		http.Error(w, "Failed to fetch report data", http.StatusInternalServerError)
		return
	}

	// Generate PDF
	pdfBytes, err := h.Service.GeneratePDF(data)
	if err != nil {
		http.Error(w, "Failed to generate PDF: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Set response headers for PDF download
	filename := fmt.Sprintf("BudgetMate_Report_%s_%d.pdf", month.String(), year)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(pdfBytes)))

	// Write PDF to response
	w.Write(pdfBytes)
}
