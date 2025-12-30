# ================================
# BudgetMate - Enterprise Production Dockerfile
# ================================

# --------------------------------
# Stage 1: Builder
# --------------------------------
FROM golang:1.22-alpine AS builder

# Install build dependencies
# git: for fetching dependencies
# make: if needed for build scripts
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Install Templ CLI for template generation
RUN go install github.com/a-h/templ/cmd/templ@latest

# Optimize dependency caching: Copy go.mod and go.sum first
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Generate templates
RUN templ generate

# Build the binary
# -ldflags="-s -w": Strip DWARF symbol table and debug info for smaller size
# CGO_ENABLED=0: Ensure static binary (required for Distroless)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -o /budgetmate ./cmd/server/main.go

# --------------------------------
# Stage 2: Production Runner
# --------------------------------
# Use Google's Distroless image for maximum security (no shell, no package manager)
FROM gcr.io/distroless/static-debian12

# Metadata
LABEL org.opencontainers.image.title="BudgetMate"
LABEL org.opencontainers.image.description="Secure Family Finance Dashboard"

# Set environment variables
ENV PORT=8080
ENV DB_PATH=/data/budgetmate.db

# Define volume for persistent SQLite storage
# Note: Railway handles this via external configuration, so we only need the path to exist
# VOLUME ["/data"] <-- Banned by Railway


# Copy binary from builder
COPY --from=builder /budgetmate /budgetmate

# Copy assets (CSS, Images)
COPY --from=builder /app/assets /assets

# Expose the application port
EXPOSE 8080

# Run the application
ENTRYPOINT ["/budgetmate"]
