#!/usr/bin/env bash
# Setup script for development environment

set -e

echo "Setting up development environment..."

# Check Go
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed"
    exit 1
fi

# Install dependencies
echo "Installing dependencies..."
go mod download

# Install tools
echo "Installing development tools..."
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Copy environment file
if [ ! -f ".env" ] && [ -f ".env.example" ]; then
    echo "Creating .env file..."
    cp .env.example .env
    echo "⚠️  Please update .env with your configuration"
fi

echo "✓ Setup complete!"
echo ""
echo "Run 'make dev' or 'go run .' to start"
