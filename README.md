# BudgetMate Web

A privacy-centric family finance dashboard for the Indian market, built with Go + Templ + HTMX.

## 🛠️ Tech Stack

- **Backend:** Go 1.22+ with Chi Router
- **UI Engine:** Templ (Type-safe HTML)
- **Interactivity:** HTMX (SPA-like feel without JSON APIs)
- **Styling:** Tailwind CSS (CDN for dev)
- **Database:** SQLite (local-first, privacy-focused)
- **Architecture:** Feature-based modular structure

## 📁 Project Structure

```
BuddyMate/
├── cmd/
│   └── server/
│       └── main.go           # Application entry point
├── internal/
│   ├── database/
│   │   └── db.go             # SQLite connection & migrations
│   ├── features/
│   │   ├── dashboard/        # Dashboard feature
│   │   │   ├── handler.go
│   │   │   └── view.templ
│   │   └── transactions/     # Transactions feature
│   │       ├── handler.go
│   │       └── view.templ
│   └── shared/
│       └── components/       # Reusable UI components
│           ├── layout.templ
│           └── cards.templ
├── assets/
│   └── css/
│       └── styles.css        # Custom styles
├── go.mod
└── README.md
```

## 🚀 Quick Start

### Prerequisites

- Go 1.22+
- Templ CLI

### 1. Install Templ CLI

```bash
go install github.com/a-h/templ/cmd/templ@latest
```

### 2. Install Dependencies

```bash
go mod tidy
```

### 3. Generate Templ Files

```bash
templ generate
```

### 4. Run the Server

```bash
go run ./cmd/server
```

Visit [http://localhost:8080](http://localhost:8080) to see the dashboard.

## 🎨 Design System – "Calm UI"

This project follows a calming, accessible design philosophy:

| Element | Class/Color |
|---------|-------------|
| Background | `bg-slate-50` (Soft White) |
| Text | `text-slate-800` (Softer Black) |
| Positive | `text-emerald-600` (Calm Green) |
| Negative | `text-rose-500` (Soft Red) |
| Warning | `text-amber-500` (Curiosity) |
| Corners | `rounded-2xl` |

**Strict Rule:** No harsh colors (pure red/green). All colors are carefully chosen for calmness.

## ✨ Features

### Dashboard
- Total Balance at a glance
- Income/Expense summary cards
- Recent transactions list
- Click-to-edit transactions (HTMX)

### Transactions
- Full transaction ledger
- Summary statistics
- Inline editing with HTMX

## 🔧 Development

### Watch Mode (Auto-reload)

Terminal 1 - Templ watcher:
```bash
templ generate --watch
```

Terminal 2 - Go server:
```bash
go run ./cmd/server
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `DB_PATH` | `./budgetmate.db` | SQLite database path |

## 📊 Database Schema

```sql
CREATE TABLE transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    amount REAL NOT NULL,
    category TEXT NOT NULL,
    date TEXT NOT NULL,
    description TEXT NOT NULL,
    type TEXT NOT NULL CHECK(type IN ('income', 'expense'))
);
```

## 🇮🇳 Indian Market Focus

- Currency formatted in INR (₹)
- Mock data with Indian brands (Swiggy, Blinkit, Ola, etc.)
- UPI payment references

## 📄 License

MIT
