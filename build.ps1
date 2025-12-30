param (
    [string]$Target = "help"
)

function Show-Help {
    Write-Host "BudgetMate Enterprise Build Script (Windows)"
    Write-Host "=========================================="
    Write-Host "Usage: .\build.ps1 <target>"
    Write-Host ""
    Write-Host "Targets:"
    Write-Host "  run       : Run locally (go run)"
    Write-Host "  build     : Compile optimized binary to bin/"
    Write-Host "  docker    : Build Production Docker image"
    Write-Host "  clean     : Remove artifacts"
}

function Run-Dev {
    Write-Host "🚀 Starting BudgetMate..."
    templ generate
    go run ./cmd/server
}

function Run-Build {
    Write-Host "🔨 Building Optimized Binary..."
    templ generate
    $env:CGO_ENABLED = "0"
    if (!(Test-Path "bin")) { New-Item -ItemType Directory -Path "bin" | Out-Null }
    # Added -ldflags="-s -w" to match Dockerfile optimization
    go build -ldflags '-s -w' -o bin/budgetmate.exe ./cmd/server
    Write-Host "✅ Built: bin\budgetmate.exe"
}

function Run-Docker {
    Write-Host "🐳 Building Docker Image..."
    docker build -t budgetmate:latest .
}

function Run-Clean {
    Write-Host "🧹 Cleaning..."
    if (Test-Path "bin") { Remove-Item -Recurse -Force "bin" }
    if (Test-Path "tmp") { Remove-Item -Recurse -Force "tmp" }
    Get-ChildItem -Filter "*.db" | Remove-Item -Force
    Get-ChildItem -Filter "*.db-*" | Remove-Item -Force
    Write-Host "✅ Cleaned"
}

switch ($Target) {
    "run" { Run-Dev }
    "dev" { Run-Dev }
    "build" { Run-Build }
    "docker" { Run-Docker }
    "clean" { Run-Clean }
    default { Show-Help }
}
