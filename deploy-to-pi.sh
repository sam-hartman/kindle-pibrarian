#!/bin/bash
# Deployment script to set up MCP server on Raspberry Pi
# Run this from your Mac

PI_HOST="192.168.1.201"
PI_USER="pi"
PI_PASS="test"
PI_HOME="/home/pi"
PROJECT_DIR="$PI_HOME/annas-mcp-server"

echo "🚀 Deploying Anna's Archive MCP Server to Raspberry Pi"
echo "Host: $PI_USER@$PI_HOST"
echo ""

# Function to run commands on Pi via SSH
ssh_pi() {
    sshpass -p "$PI_PASS" ssh -o StrictHostKeyChecking=no "$PI_USER@$PI_HOST" "$@"
}

# Function to copy files to Pi
scp_pi() {
    sshpass -p "$PI_PASS" scp -o StrictHostKeyChecking=no "$@"
}

echo "📦 Step 1: Cloning repository on Raspberry Pi..."
ssh_pi "cd $PI_HOME && [ -d annas-mcp-server ] && echo 'Repository exists, pulling updates...' && cd annas-mcp-server && git pull || (echo 'Cloning repository...' && git clone https://github.com/sam-hartman-mistral/annas-mcp-server.git)"

echo ""
echo "🔨 Step 2: Installing Go and building application..."
# Check if Go is installed, install if not
ssh_pi "command -v go >/dev/null 2>&1 || (echo 'Installing Go...' && wget -q https://go.dev/dl/go1.23.4.linux-armv6l.tar.gz && sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.23.4.linux-armv6l.tar.gz && rm go1.23.4.linux-armv6l.tar.gz && echo 'export PATH=\$PATH:/usr/local/go/bin' >> ~/.bashrc && export PATH=\$PATH:/usr/local/go/bin)"
# Build the application
ssh_pi "cd $PROJECT_DIR && export PATH=\$PATH:/usr/local/go/bin && go build -o annas-mcp ./cmd/annas-mcp"

echo ""
echo "⚙️  Step 3: Setting up .env file..."
# Create .env file on Pi
ssh_pi "cat > $PROJECT_DIR/.env << 'ENVEOF'
# Anna's Archive MCP Configuration
ANNAS_SECRET_KEY=75qvjCeMrSR6LgmR167oG2Wk4uDe5
ANNAS_DOWNLOAD_PATH=/home/pi/Downloads/Anna's Archive
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=sam.c.hartman@gmail.com
SMTP_PASSWORD=qztx aonu afxr qfsg
FROM_EMAIL=sam.c.hartman@gmail.com
KINDLE_EMAIL=sam.c.hartman_wvmqMN@kindle.com
ENVEOF
"

echo ""
echo "📥 Step 4: Downloading cloudflared..."
ssh_pi "wget -q https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm -O /tmp/cloudflared && chmod +x /tmp/cloudflared"

echo ""
echo "🛑 Step 5: Stopping any existing services..."
ssh_pi "sudo systemctl stop annas-mcp cloudflared-tunnel 2>/dev/null || true"
ssh_pi "pkill -f annas-mcp || true"
ssh_pi "pkill -f cloudflared || true"

echo ""
echo "🔧 Step 6: Setting up systemd services..."
ssh_pi "sudo bash $PROJECT_DIR/raspberry-pi-setup.sh"

echo ""
echo "▶️  Step 7: Starting services..."
ssh_pi "sudo systemctl start annas-mcp"
sleep 3
ssh_pi "sudo systemctl start cloudflared-tunnel"
sleep 5

echo ""
echo "✅ Deployment complete!"
echo ""
echo "Checking status..."
ssh_pi "sudo systemctl status annas-mcp --no-pager -l | head -10"
echo ""
echo "Getting Cloudflare tunnel URL..."
ssh_pi "sudo journalctl -u cloudflared-tunnel -n 50 --no-pager | grep 'trycloudflare.com' | tail -1"

echo ""
echo "📋 Useful commands:"
echo "  Check server status: ssh $PI_USER@$PI_HOST 'sudo systemctl status annas-mcp'"
echo "  Check tunnel status: ssh $PI_USER@$PI_HOST 'sudo systemctl status cloudflared-tunnel'"
echo "  View server logs: ssh $PI_USER@$PI_HOST 'sudo journalctl -u annas-mcp -f'"
echo "  View tunnel logs: ssh $PI_USER@$PI_HOST 'sudo journalctl -u cloudflared-tunnel -f'"
echo "  Get tunnel URL: ssh $PI_USER@$PI_HOST 'sudo journalctl -u cloudflared-tunnel -n 50 | grep trycloudflare.com'"

