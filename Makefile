# ================================
# BudgetMate Makefile
# ================================

.PHONY: run build clean docker help

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

## run: Run the application locally (development)
run:
	@echo "🚀 Starting BudgetMate..."
	@templ generate
	@go run ./cmd/server

## build: Build the production binary
build:
	@echo "🔨 Building BudgetMate binary..."
	@templ generate
	@CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/$(APP_NAME) ./cmd/server
	@echo "✅ Build complete: bin/$(APP_NAME)"

## clean: Remove build artifacts
clean:
	@echo "🧹 Cleaning up..."
	@rm -rf bin/
	@rm -f *.db
	@rm -f *.db-*
	@echo "✅ Cleaned"

## docker: Build the production Docker image
docker:
	@echo "🐳 Building Docker image..."
	@docker build -t $(DOCKER_IMAGE) .
	@echo "✅ Image built: $(DOCKER_IMAGE)"
