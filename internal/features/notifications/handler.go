package notifications

import (
	"net/http"
	"strconv"

	"github.com/budgetmate/web/internal/database"
	"github.com/budgetmate/web/internal/middleware"
	"github.com/go-chi/chi/v5"
)

// Handler is the notifications feature handler
type Handler struct{}

// NewHandler creates a new notifications handler
func NewHandler() *Handler {
	return &Handler{}
}

// HandleGetNotifications returns the notification list (HTMX partial)
func (h *Handler) HandleGetNotifications(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	notifications, err := database.GetUnreadNotifications(user.ID)
	if err != nil {
		notifications = []database.Notification{}
	}

	NotificationList(notifications).Render(r.Context(), w)
}

// HandleMarkRead marks a notification as read
func (h *Handler) HandleMarkRead(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// Verify the notification belongs to the user
	notification, err := database.GetNotification(id)
	if err != nil || notification.UserID != user.ID {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	database.MarkNotificationRead(id)

	// Return empty response to remove the item
	w.WriteHeader(http.StatusOK)
}

// HandleMarkAllRead marks all notifications as read for the user
func (h *Handler) HandleMarkAllRead(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	database.MarkAllNotificationsRead(user.ID)

	// Return empty list
	NotificationList([]database.Notification{}).Render(r.Context(), w)
}

// HandleGetCount returns just the notification count for polling
func (h *Handler) HandleGetCount(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	count := database.GetUnreadNotificationCount(user.ID)
	NotificationBadge(count).Render(r.Context(), w)
}
