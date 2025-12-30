package main

import (
	"log"
	"net/http"
	"os"

	"github.com/budgetmate/web/internal/database"
	"github.com/budgetmate/web/internal/features/auth"
	"github.com/budgetmate/web/internal/features/budgets"
	"github.com/budgetmate/web/internal/features/dashboard"
	"github.com/budgetmate/web/internal/features/family"
	"github.com/budgetmate/web/internal/features/landing"
	"github.com/budgetmate/web/internal/features/notifications"
	"github.com/budgetmate/web/internal/features/transactions"
	mw "github.com/budgetmate/web/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	// Initialize database
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./budgetmate.db"
	}

	if err := database.Init(dbPath); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Initialize router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// Static files
	fileServer := http.FileServer(http.Dir("./assets"))
	r.Handle("/assets/*", http.StripPrefix("/assets/", fileServer))

	// Initialize handlers
	landingHandler := landing.NewHandler()
	dashboardHandler := dashboard.NewHandler()
	transactionsHandler := transactions.NewHandler()
	budgetsHandler := budgets.NewHandler()
	notificationsHandler := notifications.NewHandler()

	// =====================
	// PUBLIC ROUTES (Marketing & Auth)
	// =====================
	r.Group(func(r chi.Router) {
		r.Use(mw.RedirectIfLoggedIn)
		r.Get("/", landingHandler.HandleIndex)
		r.Get("/login", auth.HandleLogin)
		r.Post("/login", auth.HandleLogin)
		r.Get("/signup", auth.HandleSignup)
		r.Post("/signup", auth.HandleSignup)
		// DEMO ROUTE
		r.Post("/demo-login", auth.HandleDemoLogin)
	})

	// Public Invite Join Route (Accessible by both guests and auth users)
	r.Get("/join/{code}", family.HandleJoinRequest)
	r.Post("/join/{code}", family.HandleJoinAction)

	r.Post("/logout", auth.HandleLogout)

	// =====================
	// APP ROUTES (Authenticated area)
	// =====================
	r.Route("/app", func(r chi.Router) {
		r.Use(mw.RequireAuth)

		// Dashboard
		r.Get("/", dashboardHandler.HandleIndex)
		r.Get("/notifications", dashboardHandler.HandleNotifications)

		// Transactions
		r.Get("/transactions", transactionsHandler.HandleList)
		r.Get("/transactions/new", transactionsHandler.HandleNew) // NEW PAGE
		r.Post("/transactions", transactionsHandler.HandleCreate) // NEW POST
		r.Get("/transactions/{id}/edit", transactionsHandler.HandleGetEdit)
		r.Get("/transactions/{id}/view", transactionsHandler.HandleGetView)
		r.Post("/transactions/{id}", transactionsHandler.HandleUpdate)

		// CSV Import
		r.Get("/transactions/import/form", transactionsHandler.HandleShowImportForm)
		r.Get("/transactions/import/cancel", transactionsHandler.HandleHideImportForm)
		r.Post("/transactions/import", transactionsHandler.HandleImport)

		// Settings (User Account Settings)
		r.Get("/settings", family.HandleUserSettings)
		r.Get("/settings/invite/form", family.HandleShowInviteForm)
		r.Post("/settings/invite", family.HandleInviteMember)
		r.Post("/settings/invite/{id}/accept", family.HandleAcceptInvite)
		r.Post("/settings/invite/{id}/decline", family.HandleDeclineInvite)

		// Family HQ (Family Command Center)
		r.Get("/family", family.HandleSettings)
		r.Get("/family/invite", family.HandleInviteLink)

		// Budgets
		r.Get("/budgets", budgetsHandler.HandleIndex)
		r.Post("/budgets", budgetsHandler.HandleSave)
		r.Post("/budgets/category", budgetsHandler.HandleAddCategory)
		r.Post("/budgets/requests", budgetsHandler.HandleCreateRequest)
		r.Post("/budgets/vote", budgetsHandler.HandleVote)

		// Notifications
		r.Get("/notifications", notificationsHandler.HandleGetNotifications)
		r.Get("/notifications/count", notificationsHandler.HandleGetCount)
		r.Post("/notifications/read/{id}", notificationsHandler.HandleMarkRead)
		r.Post("/notifications/read-all", notificationsHandler.HandleMarkAllRead)
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 BudgetMate Web starting on http://localhost:%s", port)
	log.Printf("🏠 Landing: http://localhost:%s/", port)
	log.Printf("📊 Dashboard: http://localhost:%s/app", port)

	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
