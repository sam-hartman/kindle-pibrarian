#!/bin/bash
# Quick deployment script for the updated binary with StructuredContent support

set -e

PI_HOST="192.168.1.201"
PI_USER="pi"
PI_PASS="test"
PROJECT_DIR="/home/pi/annas-mcp-server"

echo "🚀 Deploying updated annas-mcp binary..."

# Function to run commands on Pi via SSH
ssh_pi() {
    sshpass -p "$PI_PASS" ssh -o StrictHostKeyChecking=no "$PI_USER@$PI_HOST" "$@"
}

# Function to copy files to Pi
scp_pi() {
    sshpass -p "$PI_PASS" scp -o StrictHostKeyChecking=no "$@"
}

# Copy binary to Pi
echo "📦 Copying binary to Pi..."
scp_pi annas-mcp-linux-arm "$PI_USER@$PI_HOST:/tmp/annas-mcp"

# Move to project directory and restart service
echo "⚙️  Installing binary and restarting service..."
ssh_pi "sudo mv /tmp/annas-mcp $PROJECT_DIR/annas-mcp && sudo chmod +x $PROJECT_DIR/annas-mcp && sudo systemctl restart annas-mcp"

echo "✅ Deployment complete! Service restarted."
echo ""
echo "🧪 Testing search with hash verification..."
echo "Run: curl -s -X POST https://dental-mirror-cross-hub.trycloudflare.com/mcp -H 'Content-Type: application/json' -d '{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"search\",\"arguments\":{\"term\":\"python\"}}}' | python3 -m json.tool"

