#!/bin/bash
# One-liner installer for Anna's Archive MCP Server
# Usage: curl -fsSL https://raw.githubusercontent.com/sam-hartman/kindle-pibrarian/main/install.sh | bash
#
# This script:
# 1. Downloads pre-built binary from GitHub Releases
# 2. Creates directory and .env file
# 3. Sets up systemd service
# 4. Installs Tailscale and configures Funnel

set -e

REPO="sam-hartman/kindle-pibrarian"
INSTALL_DIR="$HOME/annas-mcp-server"

echo "=============================================="
echo "  Anna's Archive MCP Server Installer"
echo "=============================================="
echo ""

# Check if running as root
if [ "$EUID" -eq 0 ]; then
    echo "Please run as regular user (not root)"
    exit 1
fi

# Detect architecture
ARCH=$(uname -m)
case $ARCH in
    x86_64)
        ARCH_NAME="amd64"
        ;;
    aarch64|arm64)
        ARCH_NAME="arm64"
        ;;
    armv7l|armv6l)
        ARCH_NAME="arm"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
echo "Detected: ${OS}/${ARCH_NAME}"

# Get latest release version
echo ""
echo "Fetching latest release..."
LATEST_VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_VERSION" ]; then
    echo "Could not fetch latest version. Using 'latest'..."
    LATEST_VERSION="latest"
fi

echo "Version: $LATEST_VERSION"

# Download binary
ARCHIVE_NAME="annas-mcp-server_${LATEST_VERSION#v}_${OS}_${ARCH_NAME}.tar.xz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${LATEST_VERSION}/${ARCHIVE_NAME}"

echo ""
echo "Downloading ${ARCHIVE_NAME}..."
mkdir -p "$INSTALL_DIR"
cd "$INSTALL_DIR"

if ! curl -fsSL "$DOWNLOAD_URL" -o /tmp/annas-mcp.tar.xz; then
    echo ""
    echo "Download failed. The release may not exist yet."
    echo ""
    echo "Alternative: Build from source"
    echo "  git clone https://github.com/${REPO}.git"
    echo "  cd annas-mcp-server"
    echo "  go build -o annas-mcp ./cmd/annas-mcp"
    exit 1
fi

# Extract
echo "Extracting..."
tar -xf /tmp/annas-mcp.tar.xz --strip-components=1
rm /tmp/annas-mcp.tar.xz
chmod +x annas-mcp

echo "Binary installed to: $INSTALL_DIR/annas-mcp"

# Create .env if it doesn't exist
if [ ! -f "$INSTALL_DIR/.env" ]; then
    echo ""
    echo "=============================================="
    echo "  Configuration"
    echo "=============================================="
    echo ""

    # Prompt for API key
    echo "You need an Anna's Archive API key."
    echo "Get one at: https://annas-archive.li/faq#api"
    echo ""
    read -p "Enter your API key (or press Enter to skip): " ANNAS_KEY

    # Create .env
    cat > "$INSTALL_DIR/.env" << EOF
# Anna's Archive MCP Configuration

# Required: Anna's Archive API key
ANNAS_SECRET_KEY=${ANNAS_KEY:-your-api-key-here}

# Optional: Download path
ANNAS_DOWNLOAD_PATH=$HOME/Downloads/Anna's Archive

# Optional: Email to Kindle (configure later if needed)
# See docs/KINDLE_EMAIL_SETUP.md for instructions
#SMTP_HOST=smtp.gmail.com
#SMTP_PORT=587
#SMTP_USER=your-email@gmail.com
#SMTP_PASSWORD=your-app-password
#FROM_EMAIL=your-email@gmail.com
#KINDLE_EMAIL=your-kindle-email@kindle.com
EOF

    echo ""
    echo "Configuration saved to: $INSTALL_DIR/.env"
    if [ -z "$ANNAS_KEY" ] || [ "$ANNAS_KEY" = "your-api-key-here" ]; then
        echo "Remember to edit .env and add your API key!"
    fi
fi

# Set up systemd service
echo ""
echo "=============================================="
echo "  Setting up systemd service"
echo "=============================================="
echo ""

sudo tee /etc/systemd/system/annas-mcp.service > /dev/null << EOF
[Unit]
Description=Anna's Archive MCP Server
After=network.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/annas-mcp http --port 8081
Restart=always
RestartSec=10
EnvironmentFile=$INSTALL_DIR/.env

StandardOutput=journal
StandardError=journal
SyslogIdentifier=annas-mcp

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable annas-mcp
sudo systemctl start annas-mcp

echo "Service started!"

# Tailscale setup
echo ""
echo "=============================================="
echo "  Tailscale Funnel Setup"
echo "=============================================="
echo ""

if ! command -v tailscale >/dev/null 2>&1; then
    echo "Installing Tailscale..."
    curl -fsSL https://tailscale.com/install.sh | sh
fi

# Check if Tailscale is connected
if ! tailscale status >/dev/null 2>&1; then
    echo ""
    echo "Tailscale needs authentication."
    echo "Running 'sudo tailscale up'..."
    echo ""
    sudo tailscale up
fi

# Enable Funnel
echo ""
echo "Enabling Tailscale Funnel..."
if sudo tailscale funnel --bg 8081 2>&1 | grep -q "not enabled"; then
    echo ""
    echo "Funnel needs to be enabled on your tailnet."
    echo "Visit the URL above to enable it, then run:"
    echo "  sudo tailscale funnel --bg 8081"
else
    echo ""
    sudo tailscale funnel status
fi

# Done
echo ""
echo "=============================================="
echo "  Installation Complete!"
echo "=============================================="
echo ""
echo "Server status:"
sudo systemctl status annas-mcp --no-pager | head -5
echo ""
echo "Your MCP URL (use this in Le Chat):"
FUNNEL_URL=$(tailscale funnel status 2>/dev/null | grep -oP 'https://[a-z0-9.-]+\.ts\.net' | head -1)
if [ -n "$FUNNEL_URL" ]; then
    echo "  ${FUNNEL_URL}/mcp"
else
    echo "  (Funnel URL will appear after enabling Funnel)"
fi
echo ""
echo "Useful commands:"
echo "  View logs:      sudo journalctl -u annas-mcp -f"
echo "  Restart:        sudo systemctl restart annas-mcp"
echo "  Edit config:    nano $INSTALL_DIR/.env"
echo "  Funnel status:  sudo tailscale funnel status"
echo ""
