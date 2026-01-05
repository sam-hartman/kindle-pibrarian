#!/bin/bash
# Deployment script to run directly on Raspberry Pi
# SSH into your Pi and run: bash deploy-on-pi.sh

set -e

PI_HOME="$HOME"
PROJECT_DIR="$PI_HOME/annas-mcp-server"
CLOUDFLARE_API_TOKEN="g_OsV75zb3bGHdhrjoRnVRTJVokNpve0O8KxG5mL"
TUNNEL_NAME="annas-mcp"

echo "🚀 Deploying Anna's Archive MCP Server"
echo ""

# Check if running as root
if [ "$EUID" -eq 0 ]; then
    echo "❌ Please run this script as a regular user (not root)"
    exit 1
fi

echo "📦 Step 1: Cloning/updating repository..."
cd "$PI_HOME"
if [ -d annas-mcp-server ]; then
    echo "Repository exists, pulling updates..."
    cd annas-mcp-server
    git pull || echo "⚠️  Git pull failed, continuing with existing code..."
else
    echo "Cloning repository..."
    # Try SSH first, then HTTPS
    git clone git@github.com:sam-hartman-mistral/annas-mcp-server.git 2>/dev/null || \
    git clone https://github.com/sam-hartman-mistral/annas-mcp-server.git 2>/dev/null || {
        echo "⚠️  Git clone failed. If repo exists, continuing..."
        mkdir -p annas-mcp-server
    }
    cd annas-mcp-server
fi

echo ""
echo "🔨 Step 2: Installing Go and building application..."
# Check architecture
ARCH=$(uname -m)
echo "Detected architecture: $ARCH"

# Check if Go is installed
if ! command -v go >/dev/null 2>&1; then
    echo "Installing Go..."
    # Determine correct Go version based on architecture
    if [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
        GO_VERSION="go1.23.4.linux-arm64.tar.gz"
    elif [ "$ARCH" = "armv7l" ] || [ "$ARCH" = "armv6l" ]; then
        GO_VERSION="go1.23.4.linux-armv6l.tar.gz"
    else
        echo "⚠️  Unknown architecture, trying arm64..."
        GO_VERSION="go1.23.4.linux-arm64.tar.gz"
    fi
    
    echo "Downloading $GO_VERSION..."
    wget -q "https://go.dev/dl/$GO_VERSION" -O /tmp/go.tar.gz
    sudo rm -rf /usr/local/go
    sudo tar -C /usr/local -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    export PATH=$PATH:/usr/local/go/bin
else
    echo "Go is already installed: $(go version)"
fi

# Ensure Go is in PATH
export PATH=$PATH:/usr/local/go/bin

# Build the application
cd "$PROJECT_DIR"
export PATH=$PATH:/usr/local/go/bin
go build -o annas-mcp ./cmd/annas-mcp

echo ""
echo "⚙️  Step 3: Setting up .env file..."
cat > "$PROJECT_DIR/.env" << 'ENVEOF'
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

echo ""
echo "📥 Step 4: Downloading cloudflared..."
wget -q https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm -O /tmp/cloudflared
chmod +x /tmp/cloudflared

echo ""
echo "🛑 Step 5: Stopping any existing services..."
sudo systemctl stop annas-mcp cloudflared-tunnel 2>/dev/null || true
pkill -f annas-mcp || true
pkill -f cloudflared || true
sleep 2

echo ""
echo "🔐 Step 6: Setting up Cloudflare tunnel..."
mkdir -p ~/.cloudflared

# Try to create tunnel using cloudflared with API token
echo "Creating tunnel '$TUNNEL_NAME'..."
export CLOUDFLARE_API_TOKEN="$CLOUDFLARE_API_TOKEN"
TUNNEL_CREATE_OUTPUT=$(/tmp/cloudflared tunnel create "$TUNNEL_NAME" 2>&1 || true)

echo "$TUNNEL_CREATE_OUTPUT"

# Check if tunnel exists or was created
TUNNEL_EXISTS=false
if echo "$TUNNEL_CREATE_OUTPUT" | grep -q "already exists\|Created tunnel\|Tunnel created"; then
    TUNNEL_EXISTS=true
    echo "✅ Tunnel '$TUNNEL_NAME' exists or was created"
elif echo "$TUNNEL_CREATE_OUTPUT" | grep -q "error\|Error\|ERROR"; then
    echo "⚠️  Tunnel creation had issues, checking if it already exists..."
    TUNNEL_LIST=$(/tmp/cloudflared tunnel list 2>&1 || echo "")
    if echo "$TUNNEL_LIST" | grep -q "$TUNNEL_NAME"; then
        TUNNEL_EXISTS=true
        echo "✅ Tunnel '$TUNNEL_NAME' already exists"
    fi
fi

# Get tunnel ID
TUNNEL_ID=$(/tmp/cloudflared tunnel list 2>&1 | grep "$TUNNEL_NAME" | awk '{print $1}' | head -1 || echo "")

if [ -z "$TUNNEL_ID" ]; then
    # Try to get from credentials files
    TUNNEL_ID=$(ls ~/.cloudflared/*.json 2>/dev/null | head -1 | xargs basename | sed 's/.json//' || echo "")
fi

if [ -z "$TUNNEL_ID" ] && [ "$TUNNEL_EXISTS" = true ]; then
    echo "⚠️  Could not extract tunnel ID, but tunnel exists. Will use tunnel name instead."
    TUNNEL_ID="$TUNNEL_NAME"
fi

if [ -z "$TUNNEL_ID" ]; then
    echo "❌ Could not determine tunnel ID. Falling back to quick tunnel mode."
    TUNNEL_MODE="quick"
else
    echo "Using Tunnel ID: $TUNNEL_ID"
    TUNNEL_MODE="named"
    
    # Create config file for named tunnel
    echo "Creating tunnel configuration..."
    cat > ~/.cloudflared/config.yml << CONFEOF
tunnel: $TUNNEL_ID
credentials-file: $HOME/.cloudflared/${TUNNEL_ID}.json

ingress:
  - service: http://localhost:8081
CONFEOF
fi

echo ""
echo "🔧 Step 7: Setting up systemd services..."
# Run raspberry-pi-setup.sh to create MCP server service
sudo bash "$PROJECT_DIR/raspberry-pi-setup.sh"

# Update cloudflared-tunnel service based on tunnel mode
if [ "$TUNNEL_MODE" = "named" ]; then
    echo "Updating systemd service to use named tunnel..."
    sudo bash -c "cat > /etc/systemd/system/cloudflared-tunnel.service << 'EOF'
[Unit]
Description=Cloudflare Tunnel for Anna's Archive MCP
After=network.target annas-mcp.service
Requires=annas-mcp.service

[Service]
Type=simple
User=$USER
Environment=\"CLOUDFLARE_API_TOKEN=${CLOUDFLARE_API_TOKEN}\"
ExecStart=/tmp/cloudflared tunnel run $TUNNEL_ID
Restart=always
RestartSec=10

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=cloudflared-tunnel

[Install]
WantedBy=multi-user.target
EOF"
else
    # Use quick tunnel mode
    sudo bash -c "cat > /etc/systemd/system/cloudflared-tunnel.service << 'EOF'
[Unit]
Description=Cloudflare Tunnel for Anna's Archive MCP
After=network.target annas-mcp.service
Requires=annas-mcp.service

[Service]
Type=simple
User=$USER
ExecStart=/tmp/cloudflared tunnel --url http://localhost:8081
Restart=always
RestartSec=10

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=cloudflared-tunnel

[Install]
WantedBy=multi-user.target
EOF"
fi

# Reload systemd
sudo systemctl daemon-reload

echo ""
echo "▶️  Step 8: Starting services..."
sudo systemctl enable annas-mcp
sudo systemctl enable cloudflared-tunnel
sudo systemctl start annas-mcp
sleep 3
sudo systemctl start cloudflared-tunnel
sleep 5

echo ""
echo "✅ Deployment complete!"
echo ""
echo "Checking server status..."
sudo systemctl status annas-mcp --no-pager -l | head -15
echo ""
echo "Checking tunnel status..."
sudo systemctl status cloudflared-tunnel --no-pager -l | head -15

echo ""
echo "🔍 Getting Cloudflare tunnel URL..."
if [ "$TUNNEL_MODE" = "named" ]; then
    echo "For named tunnel, check Cloudflare dashboard:"
    echo "  https://one.dash.cloudflare.com -> Networks -> Tunnels -> $TUNNEL_NAME"
    echo ""
    echo "Or check tunnel info:"
    export CLOUDFLARE_API_TOKEN="$CLOUDFLARE_API_TOKEN"
    /tmp/cloudflared tunnel info "$TUNNEL_ID" 2>&1 | head -20
else
    echo "Quick tunnel URL:"
    sudo journalctl -u cloudflared-tunnel -n 50 --no-pager | grep 'trycloudflare.com' | tail -1
fi

echo ""
echo "📋 Useful commands:"
echo "  Check server status: sudo systemctl status annas-mcp"
echo "  Check tunnel status: sudo systemctl status cloudflared-tunnel"
echo "  View server logs: sudo journalctl -u annas-mcp -f"
echo "  View tunnel logs: sudo journalctl -u cloudflared-tunnel -f"
if [ "$TUNNEL_MODE" = "named" ]; then
    echo "  Get tunnel hostname: export CLOUDFLARE_API_TOKEN='${CLOUDFLARE_API_TOKEN}' && /tmp/cloudflared tunnel info $TUNNEL_ID"
else
    echo "  Get tunnel URL: sudo journalctl -u cloudflared-tunnel -n 50 | grep trycloudflare.com"
fi

