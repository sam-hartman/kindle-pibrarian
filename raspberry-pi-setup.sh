#!/bin/bash
# Setup script to create systemd services for Raspberry Pi
# Run this on your Raspberry Pi to make the server and tunnel start automatically

set -e

USER_HOME=$(eval echo ~$USER)
PROJECT_DIR="$USER_HOME/annas-mcp-server"
CLOUDFLARED_PATH="/tmp/cloudflared"

echo "Setting up systemd services for Anna's Archive MCP Server"
echo "Project directory: $PROJECT_DIR"
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
User=$SUDO_USER
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

# Create systemd service for Cloudflare tunnel
echo "Creating Cloudflare tunnel service..."

# Download cloudflared if not already present
if [ ! -f "$CLOUDFLARED_PATH" ]; then
    echo "Downloading cloudflared..."
    wget -q https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm -O "$CLOUDFLARED_PATH"
    chmod +x "$CLOUDFLARED_PATH"
fi

cat > /etc/systemd/system/cloudflared-tunnel.service << EOF
[Unit]
Description=Cloudflare Tunnel for Anna's Archive MCP
After=network.target annas-mcp.service
Requires=annas-mcp.service

[Service]
Type=simple
User=$SUDO_USER
ExecStart=$CLOUDFLARED_PATH tunnel --url http://localhost:8081
Restart=always
RestartSec=10

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=cloudflared-tunnel

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd
echo "Reloading systemd..."
systemctl daemon-reload

# Enable services
echo "Enabling services to start on boot..."
systemctl enable annas-mcp.service
systemctl enable cloudflared-tunnel.service

echo ""
echo "✅ Services created successfully!"
echo ""
echo "To start the services now:"
echo "  sudo systemctl start annas-mcp"
echo "  sudo systemctl start cloudflared-tunnel"
echo ""
echo "To check status:"
echo "  sudo systemctl status annas-mcp"
echo "  sudo systemctl status cloudflared-tunnel"
echo ""
echo "To view logs:"
echo "  sudo journalctl -u annas-mcp -f"
echo "  sudo journalctl -u cloudflared-tunnel -f"
echo ""
echo "To stop services:"
echo "  sudo systemctl stop annas-mcp"
echo "  sudo systemctl stop cloudflared-tunnel"
echo ""
echo "To disable auto-start:"
echo "  sudo systemctl disable annas-mcp"
echo "  sudo systemctl disable cloudflared-tunnel"

