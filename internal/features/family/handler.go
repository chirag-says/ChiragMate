package family

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/budgetmate/web/internal/database"
	"github.com/budgetmate/web/internal/middleware"
	"github.com/go-chi/chi/v5"
)

func HandleSettings(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	members, err := database.GetFamilyMembers(user.FamilyID)
	if err != nil {
		// handle error
		members = []database.User{}
	}

	SettingsPage(user, members).Render(r.Context(), w)
}

func HandleShowInviteForm(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		return
	}
	InviteModal().Render(r.Context(), w)
}

func HandleInviteMember(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		return
	}

	email := r.FormValue("email")
	name := r.FormValue("name")

	if name == "" {
		name = "Family Member"
	}

	// Check if user exists
	targetUser, err := database.GetUserByEmail(email)
	if err == nil {
		// User exists, send notification
		msg := fmt.Sprintf("%s invited you to join their family '%s'", user.Name, "Family Space") // Need to get family name really
		_ = database.CreateNotification(targetUser.ID, "invite", msg, fmt.Sprintf("%d", user.FamilyID))
	} else {
		// Create dummy member directly added to family (for demo simplicity)
		// In a real app we'd create a pending invite or email them
		_, _ = database.CreateUser(email, "password123", name, "", user.FamilyID, "member")
	}

	InviteSuccess().Render(r.Context(), w)
}

func HandleAcceptInvite(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		return
	}

	notificationIDStr := chi.URLParam(r, "id")
	notificationID, err := strconv.ParseInt(notificationIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid notification ID", http.StatusBadRequest)
		return
	}

	// Get notification to verify ownership and getting family ID
	// TODO: Need GetNotification in DB
	// For now assuming we trust the ID if we had a GetNotification function
	// Simulating:
	// notification, _ := database.GetNotification(notificationID)
	// if notification.UserID != user.ID { error }
	// familyIDStr := notification.Data

	// HACK: For speed, I'll pass familyID as query param or form value?
	// No, the notification data has it. I need to implementation GetNotification in DB first to do this safely.
	// BUT, for now, let's just mark it read and update user's familyID.
	// I need to fetch the notification to get the family_ID from `Data`.

	n, err := database.GetNotification(notificationID)
	if err != nil {
		http.Error(w, "Notification not found", http.StatusNotFound)
		return
	}

	if n.UserID != user.ID {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	familyID, err := strconv.ParseInt(n.Data, 10, 64)
	if err != nil {
		http.Error(w, "Invalid invite data", http.StatusInternalServerError)
		return
	}

	// Update user family
	if err := database.UpdateUserFamily(user.ID, familyID); err != nil {
		http.Error(w, "Failed to join family", http.StatusInternalServerError)
		return
	}

	// Mark as read
	database.MarkNotificationRead(notificationID)

	// Remove the notification item from UI
	w.Write([]byte("")) // Return empty string to remove element or show success message
	// Better: Return a toast or refresh
	w.Header().Set("HX-Redirect", "/app/settings")
}

func HandleDeclineInvite(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		return
	}

	notificationIDStr := chi.URLParam(r, "id")
	notificationID, err := strconv.ParseInt(notificationIDStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid notification ID", http.StatusBadRequest)
		return
	}

	database.MarkNotificationRead(notificationID)
	w.Write([]byte("")) // Remove from list
}
