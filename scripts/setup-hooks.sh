#!/bin/bash
# Setup script for development environment
# Usage: ./scripts/setup-hooks.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo "Setting up helm-tui development environment..."

# Check for pre-commit framework
if command -v pre-commit &> /dev/null; then
    echo "Installing pre-commit hooks (using pre-commit framework)..."
    cd "$PROJECT_ROOT"
    pre-commit install
    echo "Pre-commit hooks installed via pre-commit framework"
else
    echo "pre-commit framework not found, installing git hook manually..."
    echo "(Install pre-commit for better experience: pip install pre-commit)"

    # Install manual hook
    cp "$SCRIPT_DIR/pre-commit" "$PROJECT_ROOT/.git/hooks/pre-commit"
    chmod +x "$PROJECT_ROOT/.git/hooks/pre-commit"
    echo "Manual pre-commit hook installed"
fi

# Install golangci-lint if not present
if ! command -v golangci-lint &> /dev/null; then
    echo ""
    echo "Installing golangci-lint..."
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
    echo "golangci-lint installed"
fi

echo ""
echo "Setup complete! Pre-commit hooks will run on every commit."
echo ""
echo "To run hooks manually:"
echo "  pre-commit run --all-files  (if using pre-commit framework)"
echo "  ./scripts/pre-commit        (manual hook)"
echo ""
echo "To skip hooks temporarily:"
echo "  git commit --no-verify"
