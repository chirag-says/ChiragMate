package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

var DB *sql.DB

// --- Session Cache for High-Latency Cloud Environments ---
// Uses sync.Map for thread-safe in-memory caching of session lookups
// This eliminates repeated DB round-trips for auth middleware

var sessionCache sync.Map // map[token]cachedSession

const sessionCacheTTL = 5 * time.Minute

type cachedSession struct {
	User      *User
	CachedAt  time.Time
	ExpiresAt time.Time // DB session expiry
}

// --- Types ---

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	Name         string
	AvatarURL    string
	FamilyID     int64
	Role         string // "admin", "member"
	CreatedAt    time.Time
}

type Family struct {
	ID               int64
	Name             string
	SubscriptionTier string // "free", "premium"
	CreatedAt        time.Time
}

type Session struct {
	Token     string
	UserID    int64
	ExpiresAt time.Time
}

// Transaction represents a financial transaction
type Transaction struct {
	ID          int64
	Amount      float64
	Category    string
	Date        time.Time
	Description string
	Type        string // "income" or "expense"
	UserID      int64
	FamilyID    int64
}

// Notification represents a user notification
type Notification struct {
	ID        int64
	UserID    int64
	Type      string // "invite", "system"
	Message   string
	Data      string // JSON or metadata (e.g., family_id for invites)
	IsRead    bool
	CreatedAt time.Time
}

type Invite struct {
	Code      string
	FamilyID  int64
	CreatedBy int64
	ExpiresAt time.Time
}

// --- Initialization ---

// Init initializes the SQLite database connection and runs migrations
func Init(dbPath string) error {
	var err error

	// Check for Remote Database URL (Turso)
	dbURL := os.Getenv("DB_URL")
	driverName := "sqlite"
	dsn := dbPath

	if strings.Contains(dbURL, "libsql://") || strings.Contains(dbURL, "wss://") || strings.Contains(dbURL, "https://") {
		driverName = "libsql"
		dsn = dbURL
		log.Println("☁️  Connecting to Cloud Database (Turso)...")
	} else {
		log.Println("💾 Using Local Offline Database (SQLite)...")
	}

	DB, err = sql.Open(driverName, dsn)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	if err := DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	if err := migrate(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func migrate() error {
	// Enable foreign keys
	if _, err := DB.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return err
	}

	queries := []string{
		`CREATE TABLE IF NOT EXISTS families (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            subscription_tier TEXT DEFAULT 'free',
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP
        );`,
		`CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            email TEXT UNIQUE NOT NULL,
            password_hash TEXT NOT NULL,
            name TEXT NOT NULL,
            avatar_url TEXT,
            family_id INTEGER,
            role TEXT DEFAULT 'member',
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(family_id) REFERENCES families(id)
        );`,
		`CREATE TABLE IF NOT EXISTS sessions (
            token TEXT PRIMARY KEY,
            user_id INTEGER NOT NULL,
            expires_at DATETIME NOT NULL,
            FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
        );`,
		`CREATE TABLE IF NOT EXISTS transactions_new (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            amount REAL NOT NULL,
            category TEXT NOT NULL,
            date TEXT NOT NULL,
            description TEXT NOT NULL,
            type TEXT NOT NULL CHECK(type IN ('income', 'expense')),
            user_id INTEGER,
            family_id INTEGER,
            FOREIGN KEY(user_id) REFERENCES users(id),
            FOREIGN KEY(family_id) REFERENCES families(id)
        );`,
		`CREATE TABLE IF NOT EXISTS notifications (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL,
            type TEXT NOT NULL,
            message TEXT NOT NULL,
            data TEXT,
            is_read BOOLEAN DEFAULT 0,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
        );`,
		`CREATE TABLE IF NOT EXISTS invites (
            code TEXT PRIMARY KEY,
            family_id INTEGER NOT NULL,
            created_by INTEGER NOT NULL,
            expires_at DATETIME NOT NULL,
            FOREIGN KEY(family_id) REFERENCES families(id) ON DELETE CASCADE,
            FOREIGN KEY(created_by) REFERENCES users(id)
        );`,
		`CREATE TABLE IF NOT EXISTS budgets (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            family_id INTEGER NOT NULL,
            category TEXT NOT NULL,
            amount REAL NOT NULL,
            month TEXT NOT NULL,
            UNIQUE(family_id, category, month),
            FOREIGN KEY(family_id) REFERENCES families(id) ON DELETE CASCADE
        );`,
		`CREATE TABLE IF NOT EXISTS purchase_requests (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            family_id INTEGER NOT NULL,
            user_id INTEGER NOT NULL,
            item_name TEXT NOT NULL,
            amount REAL NOT NULL,
            status TEXT DEFAULT 'pending' CHECK(status IN ('pending', 'approved', 'rejected')),
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(family_id) REFERENCES families(id) ON DELETE CASCADE,
            FOREIGN KEY(user_id) REFERENCES users(id)
        );`,
		`CREATE TABLE IF NOT EXISTS votes (
            request_id INTEGER NOT NULL,
            user_id INTEGER NOT NULL,
            vote TEXT NOT NULL CHECK(vote IN ('approve', 'reject')),
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            PRIMARY KEY(request_id, user_id),
            FOREIGN KEY(request_id) REFERENCES purchase_requests(id) ON DELETE CASCADE,
            FOREIGN KEY(user_id) REFERENCES users(id)
        );`,
		`CREATE TABLE IF NOT EXISTS goals (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            family_id INTEGER NOT NULL,
            name TEXT NOT NULL,
            target_amount REAL NOT NULL,
            current_amount REAL DEFAULT 0,
            icon TEXT DEFAULT 'target',
            deadline DATE,
            color TEXT DEFAULT '#10B981',
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY(family_id) REFERENCES families(id) ON DELETE CASCADE
        );`,
	}

	for _, query := range queries {
		if _, err := DB.Exec(query); err != nil {
			return fmt.Errorf("migration failed: %s \nError: %w", query, err)
		}
	}

	// Simple migration strategy for transactions
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='transactions'").Scan(&count)
	if err == nil && count > 0 {
		var colCount int
		err = DB.QueryRow("SELECT COUNT(*) FROM pragma_table_info('transactions') WHERE name='family_id'").Scan(&colCount)
		if colCount == 0 {
			log.Println("Migrating legacy transactions table...")
			DB.Exec("ALTER TABLE transactions RENAME TO transactions_legacy_backup")
			DB.Exec("ALTER TABLE transactions_new RENAME TO transactions")
			DB.Exec("CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions(date DESC);")
			DB.Exec("CREATE INDEX IF NOT EXISTS idx_transactions_family ON transactions(family_id);")
		} else {
			DB.Exec("DROP TABLE IF EXISTS transactions_new")
		}
	} else {
		DB.Exec("ALTER TABLE transactions_new RENAME TO transactions")
		DB.Exec("CREATE INDEX IF NOT EXISTS idx_transactions_date ON transactions(date DESC);")
		DB.Exec("CREATE INDEX IF NOT EXISTS idx_transactions_family ON transactions(family_id);")
	}

	return nil
}

// --- Auth Functions ---

func CreateFamily(name string) (int64, error) {
	res, err := DB.Exec("INSERT INTO families (name, subscription_tier) VALUES (?, 'free')", name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func CreateUser(email, password, name, avatar string, familyID int64, role string) (*User, error) {
	// Hash password using the new security package
	hashed, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	if avatar == "" {
		avatar = "https://ui-avatars.com/api/?name=" + name + "&background=random"
	}

	res, err := DB.Exec(`
        INSERT INTO users (email, password_hash, name, avatar_url, family_id, role)
        VALUES (?, ?, ?, ?, ?, ?)
    `, email, hashed, name, avatar, familyID, role)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()

	return &User{
		ID: id, Email: email, Name: name, FamilyID: familyID, Role: role, AvatarURL: avatar,
	}, nil
}

// RegisterFamilyAdmin creates a new family and its admin user transactionally
func RegisterFamilyAdmin(name, email, password string) (*User, error) {
	tx, err := DB.Begin()
	if err != nil {
		return nil, fmt.Errorf("transaction begin failed: %w", err)
	}
	defer tx.Rollback()

	// 1. Create Family
	res, err := tx.Exec("INSERT INTO families (name, subscription_tier) VALUES (?, 'free')", "The "+name+"s")
	if err != nil {
		return nil, fmt.Errorf("failed to create family: %w", err)
	}
	familyID, _ := res.LastInsertId()

	// 2. Hash Password
	hashed, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// 3. Create Admin User
	avatar := "https://ui-avatars.com/api/?name=" + name + "&background=random"
	res, err = tx.Exec(`
        INSERT INTO users (email, password_hash, name, avatar_url, family_id, role)
        VALUES (?, ?, ?, ?, ?, ?)
    `, email, hashed, name, avatar, familyID, "admin")
	if err != nil {
		return nil, fmt.Errorf("failed to create admin user: %w", err)
	}
	userID, _ := res.LastInsertId()

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("transaction commit failed: %w", err)
	}

	return &User{
		ID: userID, Email: email, Name: name, FamilyID: familyID, Role: "admin", AvatarURL: avatar,
	}, nil
}

func GetUserByEmail(email string) (*User, error) {
	u := &User{}
	err := DB.QueryRow("SELECT id, email, password_hash, name, avatar_url, family_id, role FROM users WHERE email = ?", email).
		Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.AvatarURL, &u.FamilyID, &u.Role)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func CreateSession(userID int64) (string, error) {
	token, err := GenerateSecureToken()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().Add(24 * time.Hour * 7) // 7 days

	_, err = DB.Exec("INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)", token, userID, expiresAt)
	if err != nil {
		return "", err
	}

	// Pre-populate cache with the user data for instant subsequent lookups
	user, err := GetUserByID(userID)
	if err == nil {
		sessionCache.Store(token, cachedSession{
			User:      user,
			CachedAt:  time.Now(),
			ExpiresAt: expiresAt,
		})
	}

	return token, nil
}

// GetUserBySession retrieves a user by session token with in-memory caching
// Step A: Check cache first (0ms latency)
// Step B: If cache miss, query DB and populate cache
func GetUserBySession(token string) (*User, error) {
	// Step A: Check in-memory cache first
	if cached, ok := sessionCache.Load(token); ok {
		cs := cached.(cachedSession)
		// Validate cache TTL and session expiry
		if time.Since(cs.CachedAt) < sessionCacheTTL && time.Now().Before(cs.ExpiresAt) {
			return cs.User, nil // Cache HIT - 0ms latency!
		}
		// Cache expired, remove it
		sessionCache.Delete(token)
	}

	// Step B: Cache MISS - Query database (Turso)
	u := &User{}
	var expiresAt time.Time
	err := DB.QueryRow(`
        SELECT u.id, u.email, u.name, u.avatar_url, u.family_id, u.role, s.expires_at
        FROM sessions s
        JOIN users u ON s.user_id = u.id
        WHERE s.token = ? AND s.expires_at > CURRENT_TIMESTAMP
    `, token).Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.FamilyID, &u.Role, &expiresAt)
	if err != nil {
		return nil, err
	}

	// Store in cache for future requests
	sessionCache.Store(token, cachedSession{
		User:      u,
		CachedAt:  time.Now(),
		ExpiresAt: expiresAt,
	})

	return u, nil
}

func DeleteSession(token string) error {
	// Remove from cache first
	sessionCache.Delete(token)

	// Then remove from database
	_, err := DB.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
}

// InvalidateUserSessions removes all cached sessions for a user
// Call this when user data changes (e.g., profile update, password change)
func InvalidateUserSessions(userID int64) {
	sessionCache.Range(func(key, value interface{}) bool {
		cs := value.(cachedSession)
		if cs.User.ID == userID {
			sessionCache.Delete(key)
		}
		return true
	})
}

// UpdateUser updates a user's profile information
func UpdateUser(id int64, name, email string) error {
	// Update avatar URL if name changed
	avatar := "https://ui-avatars.com/api/?name=" + name + "&background=random"
	_, err := DB.Exec(`
        UPDATE users SET name = ?, email = ?, avatar_url = ? WHERE id = ?
    `, name, email, avatar, id)
	return err
}

// UpdatePassword updates a user's password hash
func UpdatePassword(userID int64, newHash string) error {
	_, err := DB.Exec("UPDATE users SET password_hash = ? WHERE id = ?", newHash, userID)
	return err
}

// VerifyPassword checks if the provided password matches the stored hash
func VerifyPassword(userID int64, plainPassword string) bool {
	var storedHash string
	err := DB.QueryRow("SELECT password_hash FROM users WHERE id = ?", userID).Scan(&storedHash)
	if err != nil {
		return false
	}
	return CheckPasswordHash(plainPassword, storedHash)
}

// RevokeOtherSessions deletes all sessions for a user except the current one
func RevokeOtherSessions(userID int64, currentToken string) error {
	_, err := DB.Exec("DELETE FROM sessions WHERE user_id = ? AND token != ?", userID, currentToken)
	return err
}

// GetUserByID retrieves a user by their ID
func GetUserByID(id int64) (*User, error) {
	u := &User{}
	err := DB.QueryRow(`
        SELECT id, email, password_hash, name, avatar_url, family_id, role 
        FROM users WHERE id = ?
    `, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.AvatarURL, &u.FamilyID, &u.Role)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func GetFamilyByID(id int64) (*Family, error) {
	f := &Family{}
	err := DB.QueryRow("SELECT id, name, subscription_tier, created_at FROM families WHERE id = ?", id).
		Scan(&f.ID, &f.Name, &f.SubscriptionTier, &f.CreatedAt)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func GetFamilyMembers(familyID int64) ([]User, error) {
	rows, err := DB.Query("SELECT id, name, avatar_url, role, email FROM users WHERE family_id = ?", familyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.AvatarURL, &u.Role, &u.Email); err == nil {
			users = append(users, u)
		}
	}
	return users, nil
}

// UpdateUserFamily updates the family ID for a user
func UpdateUserFamily(userID int64, familyID int64) error {
	_, err := DB.Exec("UPDATE users SET family_id = ? WHERE id = ?", familyID, userID)
	return err
}

// --- Invite Functions ---

func CreateInvite(familyID, userID int64) (string, error) {
	// Generate secure 32-char hex token
	code, err := GenerateSecureToken()
	if err != nil {
		return "", err
	}
	// Use only first 32 characters for shorter URLs
	code = code[:32]
	expiresAt := time.Now().Add(7 * 24 * time.Hour) // 7 day expiry

	_, err = DB.Exec("INSERT INTO invites (code, family_id, created_by, expires_at) VALUES (?, ?, ?, ?)",
		code, familyID, userID, expiresAt)
	if err != nil {
		return "", err
	}
	return code, nil
}

func GetInvite(code string) (*Invite, error) {
	i := &Invite{}
	err := DB.QueryRow("SELECT code, family_id, created_by, expires_at FROM invites WHERE code = ? AND expires_at > CURRENT_TIMESTAMP", code).
		Scan(&i.Code, &i.FamilyID, &i.CreatedBy, &i.ExpiresAt)
	if err != nil {
		return nil, err // Returns error if expired or not found
	}
	return i, nil
}

// --- Transaction Functions ---

// GetTransaction returns a single transaction by ID
func GetTransaction(id int64) (*Transaction, error) {
	var t Transaction
	var dateStr string
	err := DB.QueryRow(`
        SELECT id, amount, category, date, description, type, user_id, family_id
        FROM transactions 
        WHERE id = ?
    `, id).Scan(&t.ID, &t.Amount, &t.Category, &dateStr, &t.Description, &t.Type, &t.UserID, &t.FamilyID)
	if err != nil {
		return nil, err
	}
	t.Date, _ = time.Parse("2006-01-02", dateStr)
	return &t, nil
}

func UpdateTransaction(t *Transaction) error {
	_, err := DB.Exec(`
        UPDATE transactions 
        SET amount = ?, category = ?, date = ?, description = ?, type = ?
        WHERE id = ?
    `, t.Amount, t.Category, t.Date.Format("2006-01-02"), t.Description, t.Type, t.ID)
	return err
}

func GetAllTransactions(familyID int64) ([]Transaction, error) {
	rows, err := DB.Query(`
        SELECT id, amount, category, date, description, type, user_id, family_id
        FROM transactions 
        WHERE family_id = ?
        ORDER BY date DESC
    `, familyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		var dateStr string
		err := rows.Scan(&t.ID, &t.Amount, &t.Category, &dateStr, &t.Description, &t.Type, &t.UserID, &t.FamilyID)
		if err != nil {
			return nil, err
		}
		t.Date, _ = time.Parse("2006-01-02", dateStr)
		transactions = append(transactions, t)
	}
	return transactions, nil
}

func GetRecentTransactions(familyID int64, limit int) ([]Transaction, error) {
	rows, err := DB.Query(`
        SELECT id, amount, category, date, description, type, user_id, family_id
        FROM transactions 
        WHERE family_id = ?
        ORDER BY date DESC
        LIMIT ?
    `, familyID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		var dateStr string
		err := rows.Scan(&t.ID, &t.Amount, &t.Category, &dateStr, &t.Description, &t.Type, &t.UserID, &t.FamilyID)
		if err != nil {
			return nil, err
		}
		t.Date, _ = time.Parse("2006-01-02", dateStr)
		transactions = append(transactions, t)
	}
	return transactions, nil
}

// GetRecentTransactionsForDays returns transactions from the last N days
// Optimized for insight generation without fetching all historical data
func GetRecentTransactionsForDays(familyID int64, days int) ([]Transaction, error) {
	rows, err := DB.Query(`
        SELECT id, amount, category, date, description, type, user_id, family_id
        FROM transactions 
        WHERE family_id = ? AND date >= date('now', '-' || ? || ' days')
        ORDER BY date DESC
    `, familyID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		var dateStr string
		err := rows.Scan(&t.ID, &t.Amount, &t.Category, &dateStr, &t.Description, &t.Type, &t.UserID, &t.FamilyID)
		if err != nil {
			return nil, err
		}
		t.Date, _ = time.Parse("2006-01-02", dateStr)
		transactions = append(transactions, t)
	}
	return transactions, nil
}

func GetTotalBalance(familyID int64) (float64, error) {
	var income, expense float64
	DB.QueryRow(`SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE family_id = ? AND type = 'income'`, familyID).Scan(&income)
	DB.QueryRow(`SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE family_id = ? AND type = 'expense'`, familyID).Scan(&expense)
	return income - expense, nil
}

func GetTotalIncome(familyID int64) (float64, error) {
	var total float64
	err := DB.QueryRow(`SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE family_id = ? AND type = 'income'`, familyID).Scan(&total)
	return total, err
}

func GetTotalExpenses(familyID int64) (float64, error) {
	var total float64
	err := DB.QueryRow(`SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE family_id = ? AND type = 'expense'`, familyID).Scan(&total)
	return total, err
}

func GetCategoryBreakdown(familyID int64) (map[string]float64, error) {
	rows, err := DB.Query(`
        SELECT category, SUM(amount) as total 
        FROM transactions 
        WHERE family_id = ? AND type = 'expense' 
        GROUP BY category 
        ORDER BY total DESC
    `, familyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	breakdown := make(map[string]float64)
	for rows.Next() {
		var category string
		var total float64
		if err := rows.Scan(&category, &total); err == nil {
			breakdown[category] = total
		}
	}
	return breakdown, nil
}

func InsertTransaction(t *Transaction) error {
	_, err := DB.Exec(`
        INSERT INTO transactions (amount, category, date, description, type, user_id, family_id)
        VALUES (?, ?, ?, ?, ?, ?, ?)
    `, t.Amount, t.Category, t.Date.Format("2006-01-02"), t.Description, t.Type, t.UserID, t.FamilyID)
	return err
}

func BulkInsertTransactions(transactions []Transaction) (int, error) {
	if len(transactions) == 0 {
		return 0, nil
	}
	tx, err := DB.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
        INSERT INTO transactions (amount, category, date, description, type, user_id, family_id)
        VALUES (?, ?, ?, ?, ?, ?, ?)
    `)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	count := 0
	for _, t := range transactions {
		if _, err := stmt.Exec(t.Amount, t.Category, t.Date.Format("2006-01-02"), t.Description, t.Type, t.UserID, t.FamilyID); err == nil {
			count++
		}
	}
	return count, tx.Commit()
}

// --- Notification Functions ---

func CreateNotification(userID int64, nType, message, data string) error {
	_, err := DB.Exec(`
        INSERT INTO notifications (user_id, type, message, data)
        VALUES (?, ?, ?, ?)
    `, userID, nType, message, data)
	return err
}

func GetUnreadNotifications(userID int64) ([]Notification, error) {
	rows, err := DB.Query(`
        SELECT id, user_id, type, message, data, is_read, created_at
        FROM notifications
        WHERE user_id = ? AND is_read = 0
        ORDER BY created_at DESC
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifications []Notification
	for rows.Next() {
		var n Notification

		// Wait, current setup uses DATETIME DEFAULT CURRENT_TIMESTAMP.
		// It's safest to scan into time.Time if driver supports it or string.
		// modernc.org/sqlite usually handles time.Time fine if parsed.
		// But for safety let's scan as generic types or string.
		err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Message, &n.Data, &n.IsRead, &n.CreatedAt)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, n)
	}
	return notifications, nil
}

func GetNotification(id int64) (*Notification, error) {
	var n Notification
	err := DB.QueryRow(`
        SELECT id, user_id, type, message, data, is_read, created_at
        FROM notifications
        WHERE id = ?
    `, id).Scan(&n.ID, &n.UserID, &n.Type, &n.Message, &n.Data, &n.IsRead, &n.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func MarkNotificationRead(id int64) error {
	_, err := DB.Exec("UPDATE notifications SET is_read = 1 WHERE id = ?", id)
	return err
}

func MarkAllNotificationsRead(userID int64) error {
	_, err := DB.Exec("UPDATE notifications SET is_read = 1 WHERE user_id = ?", userID)
	return err
}

// --- Budget Functions ---

// Budget represents a monthly budget for a category
type Budget struct {
	ID       int64
	FamilyID int64
	Category string
	Amount   float64
	Month    string
}

// SetBudget creates or updates a budget for a category/month
func SetBudget(familyID int64, category, month string, amount float64) error {
	_, err := DB.Exec(`
        INSERT INTO budgets (family_id, category, amount, month)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(family_id, category, month) 
        DO UPDATE SET amount = excluded.amount
    `, familyID, category, amount, month)
	return err
}

// GetMonthlyBudgets returns all budgets for a family in a given month
func GetMonthlyBudgets(familyID int64, month string) (map[string]float64, error) {
	rows, err := DB.Query(`
        SELECT category, amount FROM budgets
        WHERE family_id = ? AND month = ?
    `, familyID, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	budgets := make(map[string]float64)
	for rows.Next() {
		var category string
		var amount float64
		if err := rows.Scan(&category, &amount); err == nil {
			budgets[category] = amount
		}
	}
	return budgets, nil
}

// GetCategorySpendingForMonth returns spending by category for a specific month
func GetCategorySpendingForMonth(familyID int64, month string) (map[string]float64, error) {
	rows, err := DB.Query(`
        SELECT category, SUM(amount) as total 
        FROM transactions 
        WHERE family_id = ? AND type = 'expense' AND strftime('%Y-%m', date) = ?
        GROUP BY category 
        ORDER BY total DESC
    `, familyID, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	breakdown := make(map[string]float64)
	for rows.Next() {
		var category string
		var total float64
		if err := rows.Scan(&category, &total); err == nil {
			breakdown[category] = total
		}
	}
	return breakdown, nil
}

// GetAllCategories returns all unique expense categories for a family
func GetAllCategories(familyID int64) ([]string, error) {
	rows, err := DB.Query(`
        SELECT DISTINCT category FROM transactions 
        WHERE family_id = ? AND type = 'expense'
        UNION
        SELECT DISTINCT category FROM budgets WHERE family_id = ?
        ORDER BY category
    `, familyID, familyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var cat string
		if err := rows.Scan(&cat); err == nil {
			categories = append(categories, cat)
		}
	}
	return categories, nil
}

// --- Purchase Request Functions ---

// PurchaseRequest represents a family purchase request
type PurchaseRequest struct {
	ID           int64
	FamilyID     int64
	UserID       int64
	UserName     string
	UserAvatar   string
	ItemName     string
	Amount       float64
	Status       string
	CreatedAt    time.Time
	ApproveVotes int
	RejectVotes  int
	TotalVoters  int
	UserVoted    bool
	UserVote     string
}

// CreatePurchaseRequest creates a new purchase request and notifies family
func CreatePurchaseRequest(familyID, userID int64, itemName string, amount float64) (int64, error) {
	res, err := DB.Exec(`
        INSERT INTO purchase_requests (family_id, user_id, item_name, amount)
        VALUES (?, ?, ?, ?)
    `, familyID, userID, itemName, amount)
	if err != nil {
		return 0, err
	}

	requestID, _ := res.LastInsertId()

	// Get requestor name
	var userName string
	DB.QueryRow("SELECT name FROM users WHERE id = ?", userID).Scan(&userName)

	// Notify other family members
	message := fmt.Sprintf("%s requested: %s (%s)", userName, itemName, FormatINR(amount))
	notifyFamilyExcept(familyID, userID, "purchase_request", message, fmt.Sprintf("%d", requestID))

	return requestID, nil
}

// CastVote records a vote on a purchase request and sends notifications
func CastVote(requestID, userID int64, vote string) error {
	_, err := DB.Exec(`
        INSERT INTO votes (request_id, user_id, vote)
        VALUES (?, ?, ?)
        ON CONFLICT(request_id, user_id) 
        DO UPDATE SET vote = excluded.vote
    `, requestID, userID, vote)
	if err != nil {
		return err
	}

	// Get voter name and request details
	var voterName, itemName string
	var requestorID int64
	DB.QueryRow("SELECT name FROM users WHERE id = ?", userID).Scan(&voterName)
	DB.QueryRow("SELECT user_id, item_name FROM purchase_requests WHERE id = ?", requestID).Scan(&requestorID, &itemName)

	// Notify the requestor about the vote
	voteEmoji := "approved"
	if vote == "reject" {
		voteEmoji = "rejected"
	}
	if requestorID != userID {
		message := fmt.Sprintf("%s %s your request: %s", voterName, voteEmoji, itemName)
		CreateNotification(requestorID, "vote", message, fmt.Sprintf("%d", requestID))
	}

	return nil
}

// NotifyRequestStatusChange notifies all family members when a request is approved/rejected
func NotifyRequestStatusChange(requestID int64, status string) {
	var familyID int64
	var itemName string
	DB.QueryRow("SELECT family_id, item_name FROM purchase_requests WHERE id = ?", requestID).Scan(&familyID, &itemName)

	var statusText string
	if status == "approved" {
		statusText = "approved"
	} else {
		statusText = "rejected"
	}

	message := fmt.Sprintf("Request for %s was %s!", itemName, statusText)
	notifyFamily(familyID, "request_status", message, fmt.Sprintf("%d", requestID))
}

// notifyFamily sends a notification to all members of a family
func notifyFamily(familyID int64, nType, message, data string) {
	rows, err := DB.Query("SELECT id FROM users WHERE family_id = ?", familyID)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var userID int64
		if rows.Scan(&userID) == nil {
			CreateNotification(userID, nType, message, data)
		}
	}
}

// notifyFamilyExcept sends a notification to all family members except one
func notifyFamilyExcept(familyID, exceptUserID int64, nType, message, data string) {
	rows, err := DB.Query("SELECT id FROM users WHERE family_id = ? AND id != ?", familyID, exceptUserID)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var userID int64
		if rows.Scan(&userID) == nil {
			CreateNotification(userID, nType, message, data)
		}
	}
}

// GetUnreadNotificationCount returns the count of unread notifications
func GetUnreadNotificationCount(userID int64) int {
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = ? AND is_read = 0", userID).Scan(&count)
	return count
}

// FormatINR formats a number as Indian Rupees
func FormatINR(amount float64) string {
	if amount >= 100000 {
		return fmt.Sprintf("₹%.1fL", amount/100000)
	} else if amount >= 1000 {
		return fmt.Sprintf("₹%.0fK", amount/1000)
	}
	return fmt.Sprintf("₹%.0f", amount)
}

// GetFamilyRequests returns all pending purchase requests for a family
func GetFamilyRequests(familyID, currentUserID int64) ([]PurchaseRequest, error) {
	// Get total family members for vote progress
	var totalVoters int
	DB.QueryRow("SELECT COUNT(*) FROM users WHERE family_id = ?", familyID).Scan(&totalVoters)

	rows, err := DB.Query(`
        SELECT 
            pr.id, pr.family_id, pr.user_id, u.name, u.avatar_url,
            pr.item_name, pr.amount, pr.status, pr.created_at,
            (SELECT COUNT(*) FROM votes WHERE request_id = pr.id AND vote = 'approve') as approve_votes,
            (SELECT COUNT(*) FROM votes WHERE request_id = pr.id AND vote = 'reject') as reject_votes,
            (SELECT vote FROM votes WHERE request_id = pr.id AND user_id = ?) as user_vote
        FROM purchase_requests pr
        JOIN users u ON pr.user_id = u.id
        WHERE pr.family_id = ? AND pr.status = 'pending'
        ORDER BY pr.created_at DESC
    `, currentUserID, familyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var requests []PurchaseRequest
	for rows.Next() {
		var r PurchaseRequest
		var userVote sql.NullString
		err := rows.Scan(
			&r.ID, &r.FamilyID, &r.UserID, &r.UserName, &r.UserAvatar,
			&r.ItemName, &r.Amount, &r.Status, &r.CreatedAt,
			&r.ApproveVotes, &r.RejectVotes, &userVote,
		)
		if err != nil {
			continue
		}
		r.TotalVoters = totalVoters
		r.UserVoted = userVote.Valid
		r.UserVote = userVote.String
		requests = append(requests, r)
	}
	return requests, nil
}

// GetPurchaseRequest retrieves a single request by ID
func GetPurchaseRequest(requestID, currentUserID int64) (*PurchaseRequest, error) {
	var r PurchaseRequest
	var userVote sql.NullString

	err := DB.QueryRow(`
        SELECT 
            pr.id, pr.family_id, pr.user_id, u.name, u.avatar_url,
            pr.item_name, pr.amount, pr.status, pr.created_at,
            (SELECT COUNT(*) FROM votes WHERE request_id = pr.id AND vote = 'approve') as approve_votes,
            (SELECT COUNT(*) FROM votes WHERE request_id = pr.id AND vote = 'reject') as reject_votes,
            (SELECT vote FROM votes WHERE request_id = pr.id AND user_id = ?) as user_vote
        FROM purchase_requests pr
        JOIN users u ON pr.user_id = u.id
        WHERE pr.id = ?
    `, currentUserID, requestID).Scan(
		&r.ID, &r.FamilyID, &r.UserID, &r.UserName, &r.UserAvatar,
		&r.ItemName, &r.Amount, &r.Status, &r.CreatedAt,
		&r.ApproveVotes, &r.RejectVotes, &userVote,
	)
	if err != nil {
		return nil, err
	}

	// Get total voters
	DB.QueryRow("SELECT COUNT(*) FROM users WHERE family_id = ?", r.FamilyID).Scan(&r.TotalVoters)
	r.UserVoted = userVote.Valid
	r.UserVote = userVote.String

	return &r, nil
}

// UpdateRequestStatus updates the status of a purchase request
func UpdateRequestStatus(requestID int64, status string) error {
	_, err := DB.Exec("UPDATE purchase_requests SET status = ? WHERE id = ?", status, requestID)
	return err
}

// --- Goal Functions ---

// Goal represents a family savings goal
type Goal struct {
	ID            int64
	FamilyID      int64
	Name          string
	TargetAmount  float64
	CurrentAmount float64
	Icon          string
	Deadline      *time.Time
	Color         string
	CreatedAt     time.Time
	Percentage    float64
}

// CreateGoal creates a new savings goal for a family
func CreateGoal(familyID int64, name string, targetAmount float64, icon, color string, deadline *time.Time) (int64, error) {
	if icon == "" {
		icon = "target"
	}
	if color == "" {
		color = "#10B981"
	}

	var res sql.Result
	var err error

	if deadline != nil {
		res, err = DB.Exec(`
            INSERT INTO goals (family_id, name, target_amount, icon, color, deadline)
            VALUES (?, ?, ?, ?, ?, ?)
        `, familyID, name, targetAmount, icon, color, deadline.Format("2006-01-02"))
	} else {
		res, err = DB.Exec(`
            INSERT INTO goals (family_id, name, target_amount, icon, color)
            VALUES (?, ?, ?, ?, ?)
        `, familyID, name, targetAmount, icon, color)
	}

	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ContributeToGoal adds funds to a goal's current amount
func ContributeToGoal(goalID int64, amount float64) error {
	_, err := DB.Exec(`
        UPDATE goals 
        SET current_amount = CASE 
            WHEN current_amount + ? > target_amount THEN target_amount
            ELSE current_amount + ?
        END
        WHERE id = ?
    `, amount, amount, goalID)
	return err
}

// GetFamilyGoals fetches all goals for a family
func GetFamilyGoals(familyID int64) ([]Goal, error) {
	rows, err := DB.Query(`
        SELECT id, family_id, name, target_amount, current_amount, icon, deadline, color, created_at
        FROM goals
        WHERE family_id = ?
        ORDER BY created_at DESC
    `, familyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var goals []Goal
	for rows.Next() {
		var g Goal
		var deadline sql.NullString
		err := rows.Scan(&g.ID, &g.FamilyID, &g.Name, &g.TargetAmount, &g.CurrentAmount, &g.Icon, &deadline, &g.Color, &g.CreatedAt)
		if err != nil {
			continue
		}
		if deadline.Valid {
			t, _ := time.Parse("2006-01-02", deadline.String)
			g.Deadline = &t
		}
		// Calculate percentage
		if g.TargetAmount > 0 {
			g.Percentage = (g.CurrentAmount / g.TargetAmount) * 100
		}
		goals = append(goals, g)
	}
	return goals, nil
}

// GetGoalByID retrieves a single goal by ID
func GetGoalByID(goalID int64) (*Goal, error) {
	var g Goal
	var deadline sql.NullString
	err := DB.QueryRow(`
        SELECT id, family_id, name, target_amount, current_amount, icon, deadline, color, created_at
        FROM goals WHERE id = ?
    `, goalID).Scan(&g.ID, &g.FamilyID, &g.Name, &g.TargetAmount, &g.CurrentAmount, &g.Icon, &deadline, &g.Color, &g.CreatedAt)
	if err != nil {
		return nil, err
	}
	if deadline.Valid {
		t, _ := time.Parse("2006-01-02", deadline.String)
		g.Deadline = &t
	}
	if g.TargetAmount > 0 {
		g.Percentage = (g.CurrentAmount / g.TargetAmount) * 100
	}
	return &g, nil
}

// DeleteGoal deletes a goal by ID
func DeleteGoal(goalID int64) error {
	_, err := DB.Exec("DELETE FROM goals WHERE id = ?", goalID)
	return err
}

func Close() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}
