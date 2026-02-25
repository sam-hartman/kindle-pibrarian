#!/bin/bash
# Deployment script to run directly on Raspberry Pi
# SSH into your Pi and run: bash deploy-on-pi.sh
#
# This script loads sensitive values from .env file if it exists.
# If .env doesn't exist, it will create a template that you must edit.

set -e

PI_HOME="$HOME"
PROJECT_DIR="$PI_HOME/annas-mcp-server"

# Default values
ANNAS_SECRET_KEY="${ANNAS_SECRET_KEY:-}"
SMTP_USER="${SMTP_USER:-}"
SMTP_PASSWORD="${SMTP_PASSWORD:-}"
FROM_EMAIL="${FROM_EMAIL:-}"
KINDLE_EMAIL="${KINDLE_EMAIL:-}"

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
go build -o annas-mcp ./cmd/annas-mcp

echo ""
echo "⚙️  Step 3: Setting up .env file..."
if [ -f "$PROJECT_DIR/.env" ]; then
    echo "✅ .env file already exists, loading values..."
    set -a
    source "$PROJECT_DIR/.env"
    set +a
else
    echo "⚠️  .env file not found. Creating template..."
    echo "   Please edit $PROJECT_DIR/.env with your actual values!"
cat > "$PROJECT_DIR/.env" << 'ENVEOF'
# Anna's Archive MCP Configuration
# Edit these values with your actual credentials

# Required: Anna's Archive API key (get from https://annas-archive.li/faq#api)
ANNAS_SECRET_KEY=your-api-key-here

# Optional: Download path
ANNAS_DOWNLOAD_PATH=/home/pi/Downloads/Anna's Archive

# Email configuration for Kindle (optional)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-email@gmail.com
SMTP_PASSWORD=your-app-password
FROM_EMAIL=your-email@gmail.com
KINDLE_EMAIL=your-kindle-email@kindle.com
ENVEOF
    echo ""
    echo "❌ Please edit $PROJECT_DIR/.env with your actual values before continuing!"
    echo "   Then run this script again."
    exit 1
fi

echo ""
echo "🛑 Step 4: Stopping any existing services..."
sudo systemctl stop annas-mcp 2>/dev/null || true
pkill -f annas-mcp || true
sleep 2

echo ""
echo "🔧 Step 5: Setting up systemd service..."
sudo bash "$PROJECT_DIR/scripts/raspberry-pi-setup.sh"

echo ""
echo "📡 Step 6: Setting up Tailscale Funnel..."
# Check if Tailscale is installed
if ! command -v tailscale >/dev/null 2>&1; then
    echo "Installing Tailscale..."
    curl -fsSL https://tailscale.com/install.sh | sh
fi

# Check if Tailscale is connected
if ! tailscale status >/dev/null 2>&1; then
    echo ""
    echo "⚠️  Tailscale is not connected. Please run:"
    echo "   sudo tailscale up"
    echo ""
    echo "Then visit the URL shown to authenticate."
    echo "After authenticating, run this script again."
    exit 1
fi

# Enable Funnel
echo "Enabling Tailscale Funnel on port 8081..."
sudo tailscale funnel --bg 8081 2>/dev/null || {
    echo ""
    echo "⚠️  Funnel may need to be enabled on your tailnet."
    echo "   Visit the URL shown above to enable it, then run this script again."
    exit 1
}

echo ""
echo "▶️  Step 7: Starting services..."
sudo systemctl enable annas-mcp
sudo systemctl start annas-mcp
sleep 3

echo ""
echo "✅ Deployment complete!"
echo ""
echo "Checking server status..."
sudo systemctl status annas-mcp --no-pager -l | head -15

echo ""
echo "🔍 Tailscale Funnel URL:"
sudo tailscale funnel status

echo ""
echo "📋 Useful commands:"
echo "  Check server status: sudo systemctl status annas-mcp"
echo "  Check Funnel status: sudo tailscale funnel status"
echo "  View server logs: sudo journalctl -u annas-mcp -f"
echo "  Stop Funnel: sudo tailscale funnel --https=443 off"

