#!/bin/bash
# Diagnostic script for Raspberry Pi connectivity issues

PI_HOST="192.168.1.201"
PI_USER="pi"
PI_PASS="test"

echo "🔍 Raspberry Pi Connectivity Diagnostics"
echo "=========================================="
echo ""

echo "1️⃣  Testing basic connectivity..."
if ping -c 2 "$PI_HOST" &>/dev/null; then
    echo "   ✅ Pi responds to ping"
else
    echo "   ❌ Pi does NOT respond to ping"
    echo "      - Pi might be powered off"
    echo "      - Pi might be on a different network"
    echo "      - Firewall might be blocking ICMP"
fi

echo ""
echo "2️⃣  Testing SSH port (22)..."
if nc -zv -w 3 "$PI_HOST" 22 &>/dev/null; then
    echo "   ✅ Port 22 is open"
else
    echo "   ❌ Port 22 is NOT accessible"
    echo "      - SSH service might be disabled"
    echo "      - Firewall might be blocking port 22"
    echo "      - SSH might be on a different port"
fi

echo ""
echo "3️⃣  Testing SSH connection..."
if sshpass -p "$PI_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "$PI_USER@$PI_HOST" "echo 'Connected'" &>/dev/null; then
    echo "   ✅ SSH connection successful!"
else
    echo "   ❌ SSH connection failed"
    echo "      Error: $(sshpass -p "$PI_PASS" ssh -o StrictHostKeyChecking=no -o ConnectTimeout=5 "$PI_USER@$PI_HOST" "echo 'Connected'" 2>&1 | head -1)"
fi

echo ""
echo "4️⃣  Checking Cloudflare tunnel status..."
TUNNEL_URL="https://dental-mirror-cross-hub.trycloudflare.com/mcp"
if curl -s -X POST "$TUNNEL_URL" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","id":1,"method":"ping"}' | grep -q "result"; then
    echo "   ✅ Cloudflare tunnel is working"
    echo "      This means:"
    echo "      - Pi is powered on"
    echo "      - Pi has internet connectivity"
    echo "      - annas-mcp service is running"
    echo "      - cloudflared service is running"
else
    echo "   ❌ Cloudflare tunnel is NOT working"
fi

echo ""
echo "5️⃣  Possible solutions:"
echo ""
echo "   Option A: Pi is on a different network/VLAN"
echo "   → Check if Pi and Mac are on the same network"
echo "   → Try connecting Mac to same WiFi/Ethernet as Pi"
echo ""
echo "   Option B: SSH is disabled or firewall blocking"
echo "   → Physically access Pi (HDMI/keyboard) or use another method"
echo "   → Check: sudo systemctl status ssh"
echo "   → Enable SSH: sudo systemctl enable ssh && sudo systemctl start ssh"
echo "   → Check firewall: sudo ufw status"
echo ""
echo "   Option C: IP address changed"
echo "   → Check router DHCP leases"
echo "   → Or scan network: nmap -sn 192.168.1.0/24"
echo ""
echo "   Option D: Deploy manually on Pi"
echo "   → Copy annas-mcp-linux-arm to Pi via USB/SD card"
echo "   → Or use deploy-on-pi.sh script directly on the Pi"
echo ""
echo "6️⃣  Alternative: Deploy via USB/SD card"
echo "   → Copy annas-mcp-linux-arm to USB drive"
echo "   → Plug into Pi"
echo "   → On Pi: sudo cp /media/usb/annas-mcp-linux-arm /home/pi/annas-mcp-server/"
echo "   → On Pi: sudo systemctl restart annas-mcp"

