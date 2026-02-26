#!/bin/bash
# One-liner installer for Anna's Archive MCP Server
# Usage: curl -fsSL https://raw.githubusercontent.com/sam-hartman/kindle-pibrarian/main/install.sh | bash
#
# Works on both Linux (Raspberry Pi, Ubuntu, etc.) and macOS

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

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
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
ARCHIVE_NAME="kindle-pibrarian_${LATEST_VERSION#v}_${OS}_${ARCH_NAME}.tar.xz"
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
    echo "  cd kindle-pibrarian"
    echo "  go build -o annas-mcp ./cmd/annas-mcp"
    exit 1
fi

# Extract
echo "Extracting..."
tar -xf /tmp/annas-mcp.tar.xz --strip-components=1
rm /tmp/annas-mcp.tar.xz

# Rename binary to annas-mcp (goreleaser names it kindle-pibrarian)
if [ -f "kindle-pibrarian" ]; then
    mv kindle-pibrarian annas-mcp
fi
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
    read -p "Enter your API key (or press Enter to skip): " ANNAS_KEY </dev/tty

    # Create .env
    cat > "$INSTALL_DIR/.env" << EOF
# Anna's Archive MCP Configuration

# Required: Anna's Archive API key
ANNAS_SECRET_KEY=${ANNAS_KEY:-your-api-key-here}

# Optional: Download path
ANNAS_DOWNLOAD_PATH=$HOME/Downloads/Anna's Archive

# Optional: Email to Kindle (configure later if needed)
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

# Set up service based on OS
echo ""
echo "=============================================="
if [ "$OS" = "darwin" ]; then
    echo "  Setting up launchd service (macOS)"
else
    echo "  Setting up systemd service (Linux)"
fi
echo "=============================================="
echo ""

if [ "$OS" = "darwin" ]; then
    # macOS: use launchd
    PLIST_PATH="$HOME/Library/LaunchAgents/com.annas-mcp.plist"
    mkdir -p "$HOME/Library/LaunchAgents"

    cat > "$PLIST_PATH" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.annas-mcp</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/annas-mcp</string>
        <string>http</string>
        <string>--port</string>
        <string>8081</string>
    </array>
    <key>WorkingDirectory</key>
    <string>${INSTALL_DIR}</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>HOME</key>
        <string>${HOME}</string>
    </dict>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>${INSTALL_DIR}/annas-mcp.log</string>
    <key>StandardErrorPath</key>
    <string>${INSTALL_DIR}/annas-mcp.log</string>
</dict>
</plist>
EOF

    # Load the service
    launchctl unload "$PLIST_PATH" 2>/dev/null || true
    launchctl load "$PLIST_PATH"
    echo "Service started!"
else
    # Linux: use systemd
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
fi

# Tailscale setup
echo ""
echo "=============================================="
echo "  Tailscale Funnel Setup"
echo "=============================================="
echo ""

if ! command -v tailscale >/dev/null 2>&1; then
    echo "Installing Tailscale..."
    if [ "$OS" = "darwin" ]; then
        echo "Please install Tailscale from: https://tailscale.com/download/mac"
        echo "Or via Homebrew: brew install --cask tailscale"
        echo ""
        echo "After installing, run this script again."
        exit 0
    else
        curl -fsSL https://tailscale.com/install.sh | sh
    fi
fi

# Check if Tailscale is connected
if ! tailscale status >/dev/null 2>&1; then
    echo ""
    echo "Tailscale needs authentication."
    if [ "$OS" = "darwin" ]; then
        echo "Please open Tailscale app and sign in."
        echo "Then run: tailscale funnel 8081"
    else
        echo "Running 'sudo tailscale up'..."
        sudo tailscale up
    fi
fi

# Enable Funnel
echo ""
echo "Enabling Tailscale Funnel..."
if [ "$OS" = "darwin" ]; then
    # macOS doesn't need sudo for tailscale
    if tailscale funnel 8081 2>&1 | grep -q "not enabled"; then
        echo ""
        echo "Funnel needs to be enabled on your tailnet."
        echo "Visit: https://login.tailscale.com/admin/dns"
        echo "Then run: tailscale funnel 8081"
    fi
else
    if sudo tailscale funnel --bg 8081 2>&1 | grep -q "not enabled"; then
        echo ""
        echo "Funnel needs to be enabled on your tailnet."
        echo "Visit the URL above to enable it, then run:"
        echo "  sudo tailscale funnel --bg 8081"
    else
        echo ""
        sudo tailscale funnel status
    fi
fi

# Done
echo ""
echo "=============================================="
echo "  Installation Complete!"
echo "=============================================="
echo ""

if [ "$OS" = "darwin" ]; then
    echo "Server status:"
    if launchctl list | grep -q "com.annas-mcp"; then
        echo "  Running"
    else
        echo "  Not running"
    fi
    echo ""
    echo "Your MCP URL (use this in Le Chat):"
    FUNNEL_URL=$(tailscale funnel status 2>/dev/null | grep -oE 'https://[a-z0-9.-]+\.ts\.net' | head -1)
    if [ -n "$FUNNEL_URL" ]; then
        echo "  ${FUNNEL_URL}/mcp"
    else
        echo "  (Run 'tailscale funnel 8081' to get your URL)"
    fi
    echo ""
    echo "Useful commands:"
    echo "  View logs:      tail -f $INSTALL_DIR/annas-mcp.log"
    echo "  Restart:        launchctl kickstart -k gui/\$(id -u)/com.annas-mcp"
    echo "  Stop:           launchctl unload ~/Library/LaunchAgents/com.annas-mcp.plist"
    echo "  Edit config:    nano $INSTALL_DIR/.env"
    echo "  Funnel status:  tailscale funnel status"
else
    echo "Server status:"
    sudo systemctl status annas-mcp --no-pager | head -5
    echo ""
    echo "Your MCP URL (use this in Le Chat):"
    FUNNEL_URL=$(tailscale funnel status 2>/dev/null | grep -oE 'https://[a-z0-9.-]+\.ts\.net' | head -1)
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
fi
echo ""
