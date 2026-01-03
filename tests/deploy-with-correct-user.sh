#!/bin/bash
# Deployment script using correct username: samuelhartman

set -e

PI_HOST="192.168.1.201"
PI_USER="samuelhartman"
PROJECT_DIR="/home/samuelhartman/annas-mcp-server"

echo "🚀 Deploying updated annas-mcp binary..."
echo "Using: $PI_USER@$PI_HOST"

# Copy binary to Pi
echo "📦 Copying binary..."
scp annas-mcp-linux-arm "$PI_USER@$PI_HOST:/tmp/annas-mcp"

# Move to project directory and restart service
echo "⚙️  Installing binary and restarting service..."
ssh "$PI_USER@$PI_HOST" "sudo mv /tmp/annas-mcp $PROJECT_DIR/annas-mcp && sudo chmod +x $PROJECT_DIR/annas-mcp && sudo systemctl restart annas-mcp"

echo "✅ Deployment complete!"
echo ""
echo "🧪 Testing..."
sleep 2
./test-hash-response.sh
