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

	family, err := database.GetFamilyByID(user.FamilyID)
	if err != nil {
		family = &database.Family{Name: "My Family", SubscriptionTier: "free"}
	}

	members, err := database.GetFamilyMembers(user.FamilyID)
	if err != nil {
		members = []database.User{}
	}

	SettingsPage(user, family, members).Render(r.Context(), w)
}

func HandleUserSettings(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	family, err := database.GetFamilyByID(user.FamilyID)
	if err != nil {
		family = &database.Family{Name: "My Family", SubscriptionTier: "free"}
	}

	UserSettingsPage(user, family).Render(r.Context(), w)
}

func HandleInviteLink(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		return
	}

	// Generate real invite code
	code, err := database.CreateInvite(user.FamilyID, user.ID)
	if err != nil {
		http.Error(w, "Failed to generate invite link", http.StatusInternalServerError)
		return
	}

	// Determine host for the link (could be localhost or prod domain)
	host := r.Host
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	// Force https for production look if needed, but r.Host is usually enough for dev

	link := fmt.Sprintf("%s://%s/join/%s", scheme, host, code)

	InviteLinkView(link).Render(r.Context(), w)
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
		msg := fmt.Sprintf("%s invited you to join their family '%s'", user.Name, "Family Space")
		_ = database.CreateNotification(targetUser.ID, "invite", msg, fmt.Sprintf("%d", user.FamilyID))
	} else {
		// For non-existing users, in a real app we'd email them.
		// Here we just notify success, assuming email service would handle it.
		// Or we could create a pending user.
	}

	InviteSuccess().Render(r.Context(), w)
}

// HandleJoinRequest processes the GET /join/{code} request
func HandleJoinRequest(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")

	// Verify Invite Code
	invite, err := database.GetInvite(code)
	if err != nil {
		// Render Invalid/Expired Link Page
		http.Error(w, "Invalid or expired invite link", http.StatusNotFound)
		return
	}

	// Get Family Details
	family, err := database.GetFamilyByID(invite.FamilyID)
	if err != nil {
		http.Error(w, "Family not found", http.StatusNotFound)
		return
	}

	// Check if user is logged in (Manual check since Public Route)
	var user *database.User
	if c, err := r.Cookie("session_token"); err == nil {
		user, _ = database.GetUserBySession(c.Value)
	}

	JoinPage(family, code, user).Render(r.Context(), w)
}

// HandleJoinAction processes the POST /join/{code} request
func HandleJoinAction(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")

	// Manual Session Check
	var user *database.User
	if c, err := r.Cookie("session_token"); err == nil {
		user, _ = database.GetUserBySession(c.Value)
	}

	// 1. Verify Invite
	invite, err := database.GetInvite(code)
	if err != nil {
		http.Error(w, "Invalid invite", http.StatusBadRequest)
		return
	}

	// 2. Determine User (Must be logged in)
	if user == nil {
		// This shouldn't happen if UI handles it, but just in case
		http.Redirect(w, r, "/login?next=/join/"+code, http.StatusSeeOther)
		return
	}

	// 3. Update User Family
	if err := database.UpdateUserFamily(user.ID, invite.FamilyID); err != nil {
		http.Error(w, "Failed to join family", http.StatusInternalServerError)
		return
	}

	// 4. Redirect to Dashboard
	http.Redirect(w, r, "/app", http.StatusSeeOther)
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

	// Redirect via HTMX to refresh settings page fully
	w.Header().Set("HX-Redirect", "/app/settings")
	w.WriteHeader(http.StatusOK)
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
