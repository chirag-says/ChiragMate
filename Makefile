# ================================
# BudgetMate Makefile
# ================================

.PHONY: run build clean docker help css dev

# Variables
APP_NAME := budgetmate
DOCKER_IMAGE := budgetmate:latest

# Default target
.DEFAULT_GOAL := help

## help: Show this help message
help:
	@echo "BudgetMate Enterprise Management"
	@echo "================================"
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' Makefile | sed 's/## /  /'

## css: Build production Tailwind CSS (v4)
css:
	@echo "🎨 Building Tailwind CSS v4..."
	@npx @tailwindcss/cli -i ./assets/css/input.css -o ./assets/css/styles.css --minify
	@echo "✅ CSS built: assets/css/styles.css"

## css-watch: Watch and rebuild CSS on changes (development)
css-watch:
	@echo "👀 Watching CSS changes..."
	@npx @tailwindcss/cli -i ./assets/css/input.css -o ./assets/css/styles.css --watch

## dev: Run development server with hot reload
dev:
	@echo "🚀 Starting BudgetMate (Development Mode)..."
	@make css
	@templ generate
	@go run ./cmd/server

## run: Run the application locally (uses existing CSS)
run:
	@echo "🚀 Starting BudgetMate..."
	@templ generate
	@go run ./cmd/server

## build: Build the production binary (includes CSS)
build:
	@echo "🔨 Building BudgetMate binary..."
	@make css
	@templ generate
	@CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/$(APP_NAME) ./cmd/server
	@echo "✅ Build complete: bin/$(APP_NAME)"

## clean: Remove build artifacts
clean:
	@echo "🧹 Cleaning up..."
	@rm -rf bin/
	@rm -f *.db
	@rm -f *.db-*
	@rm -f assets/css/styles.css
	@echo "✅ Cleaned"

## docker: Build the production Docker image
docker:
	@echo "🐳 Building Docker image..."
	@docker build -t $(DOCKER_IMAGE) .
	@echo "✅ Image built: $(DOCKER_IMAGE)"
