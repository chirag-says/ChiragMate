package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

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
	DB, err = sql.Open("sqlite", dbPath)
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
	return token, err
}

func GetUserBySession(token string) (*User, error) {
	u := &User{}
	err := DB.QueryRow(`
        SELECT u.id, u.email, u.name, u.avatar_url, u.family_id, u.role 
        FROM sessions s
        JOIN users u ON s.user_id = u.id
        WHERE s.token = ? AND s.expires_at > CURRENT_TIMESTAMP
    `, token).Scan(&u.ID, &u.Email, &u.Name, &u.AvatarURL, &u.FamilyID, &u.Role)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func DeleteSession(token string) error {
	_, err := DB.Exec("DELETE FROM sessions WHERE token = ?", token)
	return err
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
	code, err := GenerateInviteCode()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().Add(24 * time.Hour) // 24 hour expiry

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

func Close() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}
