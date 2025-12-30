package settings

import (
	"net/http"
	"strings"

	"github.com/budgetmate/web/internal/database"
	"github.com/budgetmate/web/internal/middleware"
)

// Handler is the settings feature handler
type Handler struct{}

// NewHandler creates a new settings handler
func NewHandler() *Handler {
	return &Handler{}
}

// HandleUpdateProfile updates user profile information
func (h *Handler) HandleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))

	// Validation
	if name == "" || email == "" {
		SettingsToast("error", "Name and email are required").Render(r.Context(), w)
		return
	}

	if len(name) < 2 {
		SettingsToast("error", "Name must be at least 2 characters").Render(r.Context(), w)
		return
	}

	if !strings.Contains(email, "@") {
		SettingsToast("error", "Invalid email address").Render(r.Context(), w)
		return
	}

	// Check if email is already taken by another user
	existingUser, err := database.GetUserByEmail(email)
	if err == nil && existingUser.ID != user.ID {
		SettingsToast("error", "Email is already in use").Render(r.Context(), w)
		return
	}

	// Update user
	if err := database.UpdateUser(user.ID, name, email); err != nil {
		SettingsToast("error", "Failed to update profile").Render(r.Context(), w)
		return
	}

	// Return success toast + updated profile card
	SettingsToast("success", "Profile updated successfully").Render(r.Context(), w)
}

// HandleChangePassword changes the user's password
func (h *Handler) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	currentPassword := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	// Validation
	if currentPassword == "" || newPassword == "" {
		PasswordToast("error", "All fields are required").Render(r.Context(), w)
		return
	}

	if newPassword != confirmPassword {
		PasswordToast("error", "New passwords do not match").Render(r.Context(), w)
		return
	}

	if len(newPassword) < 6 {
		PasswordToast("error", "Password must be at least 6 characters").Render(r.Context(), w)
		return
	}

	// Verify current password
	if !database.VerifyPassword(user.ID, currentPassword) {
		PasswordToast("error", "Current password is incorrect").Render(r.Context(), w)
		return
	}

	// Hash new password
	newHash, err := database.HashPassword(newPassword)
	if err != nil {
		PasswordToast("error", "Failed to process password").Render(r.Context(), w)
		return
	}

	// Update password
	if err := database.UpdatePassword(user.ID, newHash); err != nil {
		PasswordToast("error", "Failed to update password").Render(r.Context(), w)
		return
	}

	// Get current session token from cookie
	cookie, err := r.Cookie("session_token")
	if err == nil {
		// Revoke all other sessions for security
		database.RevokeOtherSessions(user.ID, cookie.Value)
	}

	PasswordToast("success", "Password updated! Other sessions have been logged out.").Render(r.Context(), w)
}
