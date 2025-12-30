package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

// Transaction represents a financial transaction
type Transaction struct {
	ID          int64
	Amount      float64
	Category    string
	Date        time.Time
	Description string
	Type        string // "income" or "expense"
}

// Init initializes the SQLite database connection and runs migrations
func Init(dbPath string) error {
	var err error
	DB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Run migrations
	if err := migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	// Seed initial data
	if err := seed(); err != nil {
		return fmt.Errorf("failed to seed data: %w", err)
	}

	log.Println("✓ Database initialized successfully")
	return nil
}

// migrate creates the database schema
func migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS transactions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		amount REAL NOT NULL,
		category TEXT NOT NULL,
		date TEXT NOT NULL,
		description TEXT NOT NULL,
		type TEXT NOT NULL CHECK(type IN ('income', 'expense'))
	);
	
	CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions(date DESC);
	CREATE INDEX IF NOT EXISTS idx_transactions_type ON transactions(type);
	`

	_, err := DB.Exec(schema)
	return err
}

// seed populates the database with mock Indian transactions
func seed() error {
	// Check if data already exists
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM transactions").Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return nil // Already seeded
	}

	mockTransactions := []Transaction{
		{
			Amount:      249.00,
			Category:    "Food & Dining",
			Date:        time.Now().AddDate(0, 0, -1),
			Description: "Swiggy - Biryani Order",
			Type:        "expense",
		},
		{
			Amount:      187.00,
			Category:    "Groceries",
			Date:        time.Now().AddDate(0, 0, -2),
			Description: "Blinkit - Fruits & Veggies",
			Type:        "expense",
		},
		{
			Amount:      50000.00,
			Category:    "Salary",
			Date:        time.Now().AddDate(0, 0, -5),
			Description: "Monthly Salary - UPI Credit",
			Type:        "income",
		},
		{
			Amount:      1299.00,
			Category:    "Shopping",
			Date:        time.Now().AddDate(0, 0, -3),
			Description: "Amazon - Phone Charger",
			Type:        "expense",
		},
		{
			Amount:      500.00,
			Category:    "Transport",
			Date:        time.Now().AddDate(0, 0, -1),
			Description: "Ola Auto - Office Commute",
			Type:        "expense",
		},
	}

	stmt, err := DB.Prepare(`
		INSERT INTO transactions (amount, category, date, description, type)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, t := range mockTransactions {
		_, err := stmt.Exec(t.Amount, t.Category, t.Date.Format("2006-01-02"), t.Description, t.Type)
		if err != nil {
			return err
		}
	}

	log.Println("✓ Database seeded with mock transactions")
	return nil
}

// GetAllTransactions returns all transactions ordered by date
func GetAllTransactions() ([]Transaction, error) {
	rows, err := DB.Query(`
		SELECT id, amount, category, date, description, type 
		FROM transactions 
		ORDER BY date DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		var dateStr string
		err := rows.Scan(&t.ID, &t.Amount, &t.Category, &dateStr, &t.Description, &t.Type)
		if err != nil {
			return nil, err
		}
		t.Date, _ = time.Parse("2006-01-02", dateStr)
		transactions = append(transactions, t)
	}

	return transactions, rows.Err()
}

// GetRecentTransactions returns the n most recent transactions
func GetRecentTransactions(limit int) ([]Transaction, error) {
	rows, err := DB.Query(`
		SELECT id, amount, category, date, description, type 
		FROM transactions 
		ORDER BY date DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		var dateStr string
		err := rows.Scan(&t.ID, &t.Amount, &t.Category, &dateStr, &t.Description, &t.Type)
		if err != nil {
			return nil, err
		}
		t.Date, _ = time.Parse("2006-01-02", dateStr)
		transactions = append(transactions, t)
	}

	return transactions, rows.Err()
}

// GetTransaction returns a single transaction by ID
func GetTransaction(id int64) (*Transaction, error) {
	var t Transaction
	var dateStr string
	err := DB.QueryRow(`
		SELECT id, amount, category, date, description, type 
		FROM transactions 
		WHERE id = ?
	`, id).Scan(&t.ID, &t.Amount, &t.Category, &dateStr, &t.Description, &t.Type)
	if err != nil {
		return nil, err
	}
	t.Date, _ = time.Parse("2006-01-02", dateStr)
	return &t, nil
}

// GetTotalBalance calculates total income minus expenses
func GetTotalBalance() (float64, error) {
	var income, expense float64

	err := DB.QueryRow(`SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE type = 'income'`).Scan(&income)
	if err != nil {
		return 0, err
	}

	err = DB.QueryRow(`SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE type = 'expense'`).Scan(&expense)
	if err != nil {
		return 0, err
	}

	return income - expense, nil
}

// GetTotalIncome returns the sum of all income transactions
func GetTotalIncome() (float64, error) {
	var total float64
	err := DB.QueryRow(`SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE type = 'income'`).Scan(&total)
	return total, err
}

// GetTotalExpenses returns the sum of all expense transactions
func GetTotalExpenses() (float64, error) {
	var total float64
	err := DB.QueryRow(`SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE type = 'expense'`).Scan(&total)
	return total, err
}

// GetCategoryBreakdown returns spending grouped by category
func GetCategoryBreakdown() (map[string]float64, error) {
	rows, err := DB.Query(`
		SELECT category, SUM(amount) as total 
		FROM transactions 
		WHERE type = 'expense' 
		GROUP BY category 
		ORDER BY total DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	breakdown := make(map[string]float64)
	for rows.Next() {
		var category string
		var total float64
		if err := rows.Scan(&category, &total); err != nil {
			return nil, err
		}
		breakdown[category] = total
	}

	return breakdown, rows.Err()
}

// UpdateTransaction updates a transaction
func UpdateTransaction(t *Transaction) error {
	_, err := DB.Exec(`
		UPDATE transactions 
		SET amount = ?, category = ?, date = ?, description = ?, type = ?
		WHERE id = ?
	`, t.Amount, t.Category, t.Date.Format("2006-01-02"), t.Description, t.Type, t.ID)
	return err
}

// BulkInsertTransactions inserts multiple transactions in a single database transaction
// Returns the number of successfully inserted records and any error
func BulkInsertTransactions(transactions []Transaction) (int, error) {
	if len(transactions) == 0 {
		return 0, nil
	}

	// Begin database transaction
	tx, err := DB.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if not committed

	stmt, err := tx.Prepare(`
		INSERT INTO transactions (amount, category, date, description, type)
		VALUES (?, ?, ?, ?, ?)
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	inserted := 0
	for _, t := range transactions {
		_, err := stmt.Exec(t.Amount, t.Category, t.Date.Format("2006-01-02"), t.Description, t.Type)
		if err != nil {
			// Log the error but continue with other records
			log.Printf("Warning: Failed to insert transaction '%s': %v", t.Description, err)
			continue
		}
		inserted++
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("✓ Bulk inserted %d transactions", inserted)
	return inserted, nil
}

// Close closes the database connection
func Close() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}
