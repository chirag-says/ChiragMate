package auth

import (
	"net/http"
	"time"

	"github.com/budgetmate/web/internal/database"
	"golang.org/x/crypto/bcrypt"
)

func HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		Login("").Render(r.Context(), w)
		return
	}

	if r.Method == "POST" {
		email := r.FormValue("email")
		password := r.FormValue("password")

		user, err := database.GetUserByEmail(email)
		if err != nil {
			Login("Invalid email or password").Render(r.Context(), w)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
			Login("Invalid email or password").Render(r.Context(), w)
			return
		}

		// Create Session
		token, err := database.CreateSession(user.ID)
		if err != nil {
			Login("System error, please try again").Render(r.Context(), w)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    token,
			Expires:  time.Now().Add(7 * 24 * time.Hour),
			HttpOnly: true,
			Path:     "/",
		})

		http.Redirect(w, r, "/app", http.StatusSeeOther)
	}
}

func HandleDemoLogin(w http.ResponseWriter, r *http.Request) {
	demoEmail := "demo@budgetmate.app"
	demoPassword := "demo123"

	// Check if demo user exists, if not create
	user, err := database.GetUserByEmail(demoEmail)
	if err != nil {
		// Create demo family and user
		familyID, err := database.CreateFamily("Demo Family")
		if err != nil {
			http.Error(w, "Failed to create demo family", http.StatusInternalServerError)
			return
		}

		user, err = database.CreateUser(demoEmail, demoPassword, "Demo User", "", familyID, "admin")
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
	})

	http.Redirect(w, r, "/app", http.StatusSeeOther)
}

func HandleSignup(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		Signup("").Render(r.Context(), w)
		return
	}

	if r.Method == "POST" {
		familyName := r.FormValue("familyName")
		name := r.FormValue("name")
		email := r.FormValue("email")
		password := r.FormValue("password")

		if familyName == "" || name == "" || email == "" || password == "" {
			Signup("All fields are required").Render(r.Context(), w)
			return
		}

		// 1. Create Family
		// Check if email exists first
		if _, err := database.GetUserByEmail(email); err == nil {
			Signup("Email already registered").Render(r.Context(), w)
			return
		}

		familyID, err := database.CreateFamily(familyName)
		if err != nil {
			Signup("Failed to create family space").Render(r.Context(), w)
			return
		}

		// 2. Create User
		user, err := database.CreateUser(email, password, name, "", familyID, "admin")
		if err != nil {
			Signup("Failed to create user").Render(r.Context(), w)
			return
		}

		// 3. Login
		token, err := database.CreateSession(user.ID)
		if err != nil {
			// Should fallback to login page
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    token,
			Expires:  time.Now().Add(7 * 24 * time.Hour),
			HttpOnly: true,
			Path:     "/",
		})

		http.Redirect(w, r, "/app", http.StatusSeeOther)
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
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
