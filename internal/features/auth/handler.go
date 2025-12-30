package auth

import (
	"net/http"
	"time"

	"github.com/budgetmate/web/internal/database"
)

func HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		// Pass the 'next' parameter if present, basically implies we might need to modify view to support it
		// Check r.URL.Query().Get("next")
		// But the view function Login() takes only 'error' string currently.
		// We will keep it simple: We rely on the layout or custom handling,
		// BUT actually, we can't preserve "next" if the view doesn't render it in the form action.
		// For now, let's assume the user just wants the redirect logic fixed.
		// We will need to update the view to preserve 'next'.
		Login("", r.URL.Query().Get("next")).Render(r.Context(), w)
		return
	}

	if r.Method == "POST" {
		email := r.FormValue("email")
		password := r.FormValue("password")

		// Grab 'next' from query param - NOTE: r.FormValue gets from query OR body
		next := r.FormValue("next")

		user, err := database.GetUserByEmail(email)
		if err != nil {
			Login("Invalid email or password", next).Render(r.Context(), w)
			return
		}

		if !database.CheckPasswordHash(password, user.PasswordHash) {
			Login("Invalid email or password", next).Render(r.Context(), w)
			return
		}

		// Create Session
		token, err := database.CreateSession(user.ID)
		if err != nil {
			Login("System error, please try again", next).Render(r.Context(), w)
			return
		}

		// Secure Cookie Setting
		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    token,
			Expires:  time.Now().Add(7 * 24 * time.Hour),
			HttpOnly: true,
			Path:     "/",
			Secure:   false, // Set to true in prod
			SameSite: http.SameSiteStrictMode,
		})

		// Handles Redirect
		if next != "" {
			http.Redirect(w, r, next, http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/app", http.StatusSeeOther)
		}
	}
}

func HandleDemoLogin(w http.ResponseWriter, r *http.Request) {
	demoEmail := "demo@budgetmate.app"
	// demoPassword := "demo123" // Not needed for programmatic login if we skip hash check for demo path, OR we ensure demo user has this password hashed in DB

	// Check if demo user exists, if not create
	user, err := database.GetUserByEmail(demoEmail)
	if err != nil {
		// Create demo family and user using transactional helper or manually
		// Since CreateUser now hashes by default, we can just call it

		// 1. Create Demo Family
		familyID, err := database.CreateFamily("Demo Family")
		if err != nil {
			http.Error(w, "Failed to create demo family", http.StatusInternalServerError)
			return
		}

		// 2. Create User
		user, err = database.CreateUser(demoEmail, "demo123", "Demo User", "", familyID, "admin")
		if err != nil {
			http.Error(w, "Failed to create demo user", http.StatusInternalServerError)
			return
		}

		// Seed demo data
		mockTransactions := []database.Transaction{
			{
				Amount:      249.00,
				Category:    "Food & Dining",
				Date:        time.Now().AddDate(0, 0, -1),
				Description: "Swiggy - Biryani Order",
				Type:        "expense",
				UserID:      user.ID,
				FamilyID:    familyID,
			},
			{
				Amount:      187.00,
				Category:    "Groceries",
				Date:        time.Now().AddDate(0, 0, -2),
				Description: "Blinkit - Fruits & Veggies",
				Type:        "expense",
				UserID:      user.ID,
				FamilyID:    familyID,
			},
			{
				Amount:      50000.00,
				Category:    "Salary",
				Date:        time.Now().AddDate(0, 0, -5),
				Description: "Monthly Salary - UPI Credit",
				Type:        "income",
				UserID:      user.ID,
				FamilyID:    familyID,
			},
		}
		database.BulkInsertTransactions(mockTransactions)
	}

	// Login logic
	token, err := database.CreateSession(user.ID)
	if err != nil {
		http.Redirect(w, r, "/login?error=System error", http.StatusSeeOther)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Expires:  time.Now().Add(24 * time.Hour), // 1 day for demo
		HttpOnly: true,
		Path:     "/",
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
	})

	http.Redirect(w, r, "/app", http.StatusSeeOther)
}

func HandleSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		Signup("", r.URL.Query().Get("next")).Render(r.Context(), w)
		return
	}

	if r.Method == "POST" {
		// familyName := r.FormValue("familyName") // "Name" in form is just name? Wait, logic from previous file had familyName
		// Let's check view... but prompt says "The [Name]s" in transactional helper.
		// Assuming the form sends 'name' (user name) and maybe 'familyName' implied or separate?
		// User prompt: logic.go helper RegisterFamilyAdmin(name, email, password). It implies Family Name "The [Name]s". So user name is used to name family.

		name := r.FormValue("name") // User name
		email := r.FormValue("email")
		password := r.FormValue("password")
		next := r.FormValue("next")

		if name == "" || email == "" || password == "" {
			Signup("All fields are required", next).Render(r.Context(), w)
			return
		}

		// Check if email exists first
		if _, err := database.GetUserByEmail(email); err == nil {
			Signup("Email already registered", next).Render(r.Context(), w)
			return
		}

		// Transactional Signup
		user, err := database.RegisterFamilyAdmin(name, email, password)
		if err != nil {
			Signup("Registration failed. Please try again.", next).Render(r.Context(), w)
			return
		}

		// Auto-login
		token, err := database.CreateSession(user.ID)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    token,
			Expires:  time.Now().Add(7 * 24 * time.Hour),
			HttpOnly: true,
			Path:     "/",
			Secure:   false,
			SameSite: http.SameSiteStrictMode,
		})

		// Handle next redirect if provided (though mostly used for login)
		if next != "" {
			http.Redirect(w, r, next, http.StatusSeeOther)
		} else {
			http.Redirect(w, r, "/app", http.StatusSeeOther)
		}
	}
}

func HandleLogout(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("session_token")
	if err == nil {
		database.DeleteSession(c.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		MaxAge:   -1,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
