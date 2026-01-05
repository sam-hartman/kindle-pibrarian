#!/bin/bash
# Script to start the MCP server with email configuration for Kindle

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Load environment variables from .env file (preferred method)
if [ -f "$SCRIPT_DIR/load-env.sh" ]; then
    source "$SCRIPT_DIR/load-env.sh"
fi

# Fallback: try loading from setup-email-env.sh for backwards compatibility
if [ -f "$SCRIPT_DIR/setup-email-env.sh" ]; then
    source "$SCRIPT_DIR/setup-email-env.sh"
fi

# Set default download path if not set
export ANNAS_DOWNLOAD_PATH="${ANNAS_DOWNLOAD_PATH:-/Users/samuelhartman/Downloads/Anna's Archive}"

# Check if API key is set
if [ -z "$ANNAS_SECRET_KEY" ]; then
    echo "⚠️  Warning: ANNAS_SECRET_KEY is not set"
    echo "   Set it in .env file or export it:"
    echo "   export ANNAS_SECRET_KEY='your-key'"
fi

# Start the HTTP server
echo "🚀 Starting MCP server with Kindle email support..."
if [ -n "$FROM_EMAIL" ]; then
    echo "   From: $FROM_EMAIL"
fi
if [ -n "$KINDLE_EMAIL" ]; then
    echo "   To: $KINDLE_EMAIL"
fi
echo ""
./annas-mcp http --port 8080

