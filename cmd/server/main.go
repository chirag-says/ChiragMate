package main

import (
	"log"
	"net/http"
	"os"

	"github.com/budgetmate/web/internal/database"
	"github.com/budgetmate/web/internal/features/dashboard"
	"github.com/budgetmate/web/internal/features/landing"
	"github.com/budgetmate/web/internal/features/transactions"
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

	// =====================
	// PUBLIC ROUTES (Marketing)
	// =====================
	r.Get("/", landingHandler.HandleIndex)

	// =====================
	// APP ROUTES (Authenticated area)
	// =====================
	r.Route("/app", func(r chi.Router) {
		// Dashboard
		r.Get("/", dashboardHandler.HandleIndex)

		// Transactions
		r.Get("/transactions", transactionsHandler.HandleList)
		r.Get("/transactions/{id}/edit", transactionsHandler.HandleGetEdit)
		r.Get("/transactions/{id}/view", transactionsHandler.HandleGetView)
		r.Post("/transactions/{id}", transactionsHandler.HandleUpdate)

		// CSV Import
		r.Get("/transactions/import/form", transactionsHandler.HandleShowImportForm)
		r.Get("/transactions/import/cancel", transactionsHandler.HandleHideImportForm)
		r.Post("/transactions/import", transactionsHandler.HandleImport)

		// Settings placeholder
		r.Get("/settings", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><head><title>Settings</title></head><body><h1>Settings Coming Soon</h1><a href="/app">Back to Dashboard</a></body></html>`))
		})
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 BudgetMate Web starting on http://localhost:%s", port)
	log.Printf("🏠 Landing: http://localhost:%s/", port)
	log.Printf("📊 Dashboard: http://localhost:%s/app", port)
	log.Printf("💰 Transactions: http://localhost:%s/app/transactions", port)

	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
