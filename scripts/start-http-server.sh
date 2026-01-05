#!/bin/bash
# Script to start the Anna's Archive MCP HTTP server for Mistral Le Chat

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Load environment variables from .env file
if [ -f "$SCRIPT_DIR/load-env.sh" ]; then
    source "$SCRIPT_DIR/load-env.sh"
fi

# Fallback: try loading from setup-email-env.sh for backwards compatibility
if [ -f "$SCRIPT_DIR/setup-email-env.sh" ]; then
    source "$SCRIPT_DIR/setup-email-env.sh"
fi

# Set default download path if not set
if [ -z "$ANNAS_DOWNLOAD_PATH" ]; then
    export ANNAS_DOWNLOAD_PATH="/Users/samuelhartman/Downloads/Anna's Archive"
fi

# Check if API key is set
if [ -z "$ANNAS_SECRET_KEY" ]; then
    echo "⚠️  Warning: ANNAS_SECRET_KEY is not set"
    echo "   Downloads will fail. Set it in .env file or export it:"
    echo "   export ANNAS_SECRET_KEY=\"your-key\""
    echo ""
    echo "   To set up .env file:"
    echo "   cp .env.example .env"
    echo "   # Then edit .env with your values"
fi

# Start the HTTP server on port 8080
./annas-mcp http --port 8080

