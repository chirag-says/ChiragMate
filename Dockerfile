# ================================
# BudgetMate - Enterprise Production Dockerfile
# ================================

# --------------------------------
# Stage 1: Builder
# --------------------------------
FROM golang:1.24-alpine AS builder

# Install build dependencies
# git: for fetching dependencies
# make: for build scripts
# nodejs/npm: for Tailwind CSS v4 build
RUN apk add --no-cache git make nodejs npm

# Set working directory
WORKDIR /app

# Install Templ CLI for template generation
RUN go install github.com/a-h/templ/cmd/templ@latest

# Optimize dependency caching: Copy go.mod and go.sum first
COPY go.mod go.sum ./
RUN go mod download

# Copy package.json for npm dependencies
COPY package.json package-lock.json* ./
RUN npm install

# Copy Tailwind config and CSS input
COPY tailwind.config.js ./
COPY assets/css/input.css ./assets/css/

# Copy source code
COPY . .

# Generate Tailwind CSS v4 (production minified)
RUN npx @tailwindcss/cli -i ./assets/css/input.css -o ./assets/css/styles.css --minify

# Generate templ templates
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
# Switched to Alpine for better runtime compatibility and directory management
FROM alpine:latest

# Install CA certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create the data directory explicitly to prevent runtime crashes
RUN mkdir -p /data

# Set environment variables
ENV PORT=8080
ENV DB_PATH=/data/budgetmate.db

# Copy binary from builder
COPY --from=builder /budgetmate /budgetmate

# Copy assets (CSS, Images)
COPY --from=builder /app/assets /assets

# Expose the application port
EXPOSE 8080

# Run the application
ENTRYPOINT ["/budgetmate"]
