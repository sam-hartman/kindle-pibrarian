#!/bin/bash
# Setup script to create systemd service for Raspberry Pi
# Run this on your Raspberry Pi to make the server start automatically

set -e

# Get the actual user who invoked sudo
ACTUAL_USER=${SUDO_USER:-$USER}
if [ "$ACTUAL_USER" = "root" ]; then
    ACTUAL_USER="pi"
fi
USER_HOME=$(eval echo ~$ACTUAL_USER)
PROJECT_DIR="$USER_HOME/annas-mcp-server"

echo "Setting up systemd service for Anna's Archive MCP Server"
echo "Project directory: $PROJECT_DIR"
echo "User: $ACTUAL_USER"
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
    echo "This script needs sudo privileges. Please run with sudo."
    exit 1
fi

# Create systemd service for MCP server
echo "Creating MCP server service..."
cat > /etc/systemd/system/annas-mcp.service << EOF
[Unit]
Description=Anna's Archive MCP Server
After=network.target

[Service]
Type=simple
User=$ACTUAL_USER
WorkingDirectory=$PROJECT_DIR
ExecStart=$PROJECT_DIR/annas-mcp http --port 8081
Restart=always
RestartSec=10
EnvironmentFile=$PROJECT_DIR/.env

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=annas-mcp

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd
echo "Reloading systemd..."
systemctl daemon-reload

# Enable service
echo "Enabling service to start on boot..."
systemctl enable annas-mcp.service

echo ""
echo "✅ Service created successfully!"
echo ""
echo "To start the service now:"
echo "  sudo systemctl start annas-mcp"
echo ""
echo "To check status:"
echo "  sudo systemctl status annas-mcp"
echo ""
echo "To view logs:"
echo "  sudo journalctl -u annas-mcp -f"
echo ""
echo "To stop service:"
echo "  sudo systemctl stop annas-mcp"
echo ""
echo "To disable auto-start:"
echo "  sudo systemctl disable annas-mcp"
echo ""
echo "For internet access, use Tailscale Funnel:"
echo "  sudo tailscale funnel --bg 8081"

