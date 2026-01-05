#!/bin/bash
# Helper script to load environment variables from .env file
# This can be sourced by other scripts: source load-env.sh

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="$SCRIPT_DIR/.env"

if [ -f "$ENV_FILE" ]; then
    # Export variables from .env file
    # Use set -a to automatically export all variables
    set -a
    # Source the file, but skip comments and empty lines
    while IFS= read -r line || [ -n "$line" ]; do
        # Skip empty lines and comments
        trimmed=$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
        if [ -z "$trimmed" ] || [ "${trimmed#\#}" != "$trimmed" ]; then
            continue
        fi
        # Export the line (bash will handle key=value)
        eval "export $line" 2>/dev/null || true
    done < "$ENV_FILE"
    set +a
    echo "✅ Loaded environment variables from .env file"
else
    echo "⚠️  Warning: .env file not found at $ENV_FILE"
    echo "   Copy .env.example to .env and fill in your values"
fi
