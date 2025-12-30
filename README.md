# BudgetMate Premium
> **Status:** 🟢 Production Ready (v1.0.0)  
> **Stack:** ![Go](https://img.shields.io/badge/Go-1.22-00ADD8?style=flat-square&logo=go&logoColor=white) ![Templ](https://img.shields.io/badge/Templ-UI-black?style=flat-square) ![HTMX](https://img.shields.io/badge/HTMX-Interactive-3D58F2?style=flat-square) ![SQLite](https://img.shields.io/badge/SQLite-ModernC-003B57?style=flat-square) ![Docker](https://img.shields.io/badge/Docker-Distroless-2496ED?style=flat-square)

BudgetMate is a high-performance, privacy-first family finance dashboard designed for Indian households. Built with a "Zero-Maintenance" philosophy, it delivers an app-like experience without the complexity of modern frontend frameworks.

---

## 1. Executive Summary

**For Business Stakeholders:**

BudgetMate solves the problem of financial fragmentation with a secure, local-first approach. Unlike competitors that sell user data, BudgetMate operates entirely on the user's infrastructure (or personal device).

*   **Privacy-First Architecture:** Financial data never leaves the server. There are no third-party tracking scripts or external API dependencies for data storage.
*   **Zero-Maintenance Cost:** By utilizing **SQLite** and a **Single Binary (Go)** architecture, the application requires no separate unnecessary database servers (like Postgres/MySQL) to manage. It runs anywhere—from a $5 Raspberry Pi to an Enterprise Cloud Server.
*   **Key Features:**
    *   **Zero-Knowledge CSV Import:** Drag-and-drop bank statements are processed entirely in memory.
    *   **Calm AI Nudges:** Non-intrusive financial insights that respect user attention.
    *   **Family Sync Ready:** Designed for multi-user household access (roadmap).

---

## 2. Architecture Decision Record (ADR)

We chose a "boring" but robust technology stack to ensure long-term stability and performance.

| Decision | Technology | Rationale |
| :--- | :--- | :--- |
| **Language** | **Go (Golang)** | Selected for its compiled performance, type safety, and ultra-low memory footprint. It allows us to ship a single, self-contained executable. |
| **Frontend** | **Templ + HTMX** | Delivers a Single Page Application (SPA) feel (smooth transitions, no page reloads) *without* the complexity of React/Next.js. This ensures instant load times and perfect SEO. |
| **Database** | **SQLite (ModernC)** | A serverless, zero-latency/zero-configuration database engine. We use `modernc.org/sqlite` (pure Go driver) to avoid CGO headaches, making cross-compilation trivial. |
| **Container** | **Distroless Docker** | We package the app in Google's **Distroless** image. It contains *only* the application binary—no shell, no package manager, no OS bloat—reducing the security attack surface by ~95%. |

---

## 3. Developer Guide

### Prerequisites
*   **Go 1.22+**
*   **Make** (or PowerShell on Windows)
*   **Docker** (for production builds)

### Quick Start
To start the development server with hot-reload (if `air` is installed) or standard run:

**Mac/Linux:**
```bash
make run
```

**Windows:**
```powershell
.\build.ps1 run
```
*This compiles templates and starts the server at `http://localhost:8080`.*

### Production Build
Create a highly optimized, stripped Docker image:

**Mac/Linux:**
```bash
make docker
```

**Windows:**
```powershell
.\build.ps1 docker
```

### Directory Structure
```text
/cmd/server       # Entry point (Main application initialization)
/internal
  /features       # Vertical Slices (Each feature owns its logic & UI)
    /dashboard    # Dashboard Logic + .templ views
    /transactions # Transaction Logic + .templ views
  /shared         # Reusable UI components (Layouts, Cards, Charts)
  /database       # SQLite connectivity
/assets           # Static files (CSS, Images)
```

---

## 4. API Reference (Internal)

BudgetMate uses a Hypermedia-Driven API (HATEOAS). Most endpoints return **HTML Fragments**, not JSON.

*   `GET /app`
    *   **Description:** The main dashboard view. Renders the full application frame.
*   `GET /transactions/import/form`
    *   **Type:** HTMX Partial
    *   **Description:** Returns the drag-and-drop upload form modal.
*   `POST /transactions/import`
    *   **Type:** Multipart Form Data
    *   **Description:** Accepts CSV file uploads. Parses content in-memory and redirects to the categorization view upon success.
    *   **Response:** HTML (Success Toast) or Error Block.
