#!/bin/bash
# Script to start the Anna's Archive MCP HTTP server
# Supports both basic HTTP mode and Kindle email mode (auto-detected from .env)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

# Load environment variables from .env file
if [ -f "$PROJECT_ROOT/.env" ]; then
    set -a
    source "$PROJECT_ROOT/.env"
    set +a
fi

# Set default download path if not set
export ANNAS_DOWNLOAD_PATH="${ANNAS_DOWNLOAD_PATH:-$(pwd)/downloads}"

# Check if API key is set
if [ -z "$ANNAS_SECRET_KEY" ]; then
    echo "⚠️  Warning: ANNAS_SECRET_KEY is not set"
    echo "   Downloads will fail. Set it in .env file or export it:"
    echo "   export ANNAS_SECRET_KEY=\"your-key\""
    echo ""
    echo "   To set up .env file:"
    echo "   cp .env.example .env"
    echo "   # Then edit .env with your values"
    echo ""
fi

# Determine server mode and display info
if [ -n "$FROM_EMAIL" ] && [ -n "$KINDLE_EMAIL" ]; then
    echo "🚀 Starting MCP server with Kindle email support..."
    echo "   From: $FROM_EMAIL"
    echo "   To: $KINDLE_EMAIL"
else
    echo "🚀 Starting MCP HTTP server..."
    echo "   (Email not configured - books will be downloaded locally)"
fi
echo ""

# Check if binary exists, build if not
if [ ! -f "./annas-mcp" ]; then
    echo "📦 Binary not found, building..."
    if ! go build -o annas-mcp ./cmd/annas-mcp 2>&1; then
        echo "❌ Build failed. Please ensure Go is installed and dependencies are available."
        exit 1
    fi
    echo "✅ Build successful"
    echo ""
fi

# Start the HTTP server on port 8080
PORT="${PORT:-8080}"
./annas-mcp http --port "$PORT"

