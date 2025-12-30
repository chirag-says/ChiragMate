package middleware

import (
	"context"
	"net/http"

	"github.com/budgetmate/web/internal/database"
)

type contextKey string

const UserKey contextKey = "user"

// RequireAuth middleware ensures the user is logged in
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for session cookie
		c, err := r.Cookie("session_token")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Validate session
		user, err := database.GetUserBySession(c.Value)
		if err != nil {
			// Invalid session, clear cookie
			http.SetCookie(w, &http.Cookie{
				Name:     "session_token",
				Value:    "",
				Path:     "/",
				MaxAge:   -1,
				HttpOnly: true,
			})
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Add user to context
		ctx := context.WithValue(r.Context(), UserKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RedirectIfLoggedIn redirects authenticated users to /app (for login/signup pages)
func RedirectIfLoggedIn(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("session_token")
		if err == nil {
			if _, err := database.GetUserBySession(c.Value); err == nil {
				http.Redirect(w, r, "/app", http.StatusSeeOther)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// GetUser retrieves the user from context
func GetUser(ctx context.Context) *database.User {
	user, ok := ctx.Value(UserKey).(*database.User)
	if !ok {
		return nil
	}
	return user
}
