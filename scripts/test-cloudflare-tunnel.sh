#!/bin/bash
# Test Cloudflare tunnel connection for Anna's Archive MCP Server
# Usage: ./scripts/test-cloudflare-tunnel.sh [tunnel-url]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load .env if it exists
if [ -f "$PROJECT_ROOT/.env" ]; then
    set -a
    source "$PROJECT_ROOT/.env"
    set +a
fi

# Configuration
PI_HOST="${PI_HOST:-192.168.1.201}"
PI_USER="${PI_USER:-pi}"
PI_PASS="${PI_PASS:-}"
TUNNEL_URL="${1:-}"

echo "🔍 Testing Cloudflare Tunnel Connection"
echo "========================================"
echo ""

# Function to test tunnel URL
test_tunnel_url() {
    local url="$1"
    echo "Testing tunnel URL: $url"
    echo ""
    
    # Test 1: Health check
    echo "📋 Test 1: Health Check"
    echo "-----------------------------------"
    HEALTH_RESPONSE=$(curl -s -w "\n%{http_code}" "$url/health" 2>&1 || echo "ERROR")
    HTTP_CODE=$(echo "$HEALTH_RESPONSE" | tail -1)
    BODY=$(echo "$HEALTH_RESPONSE" | sed '$d')
    
    if [ "$HTTP_CODE" = "200" ]; then
        echo "✅ Health check successful"
        echo "$BODY" | jq . 2>/dev/null || echo "$BODY"
    else
        echo "❌ Health check failed (HTTP $HTTP_CODE)"
        echo "$BODY"
        return 1
    fi
    
    echo ""
    
    # Test 2: MCP Discovery
    echo "📋 Test 2: MCP Discovery"
    echo "-----------------------------------"
    MCP_RESPONSE=$(curl -s -w "\n%{http_code}" "$url/mcp" 2>&1 || echo "ERROR")
    HTTP_CODE=$(echo "$MCP_RESPONSE" | tail -1)
    BODY=$(echo "$MCP_RESPONSE" | sed '$d')
    
    if [ "$HTTP_CODE" = "200" ]; then
        echo "✅ MCP discovery successful"
        echo "$BODY" | jq . 2>/dev/null || echo "$BODY"
    else
        echo "❌ MCP discovery failed (HTTP $HTTP_CODE)"
        echo "$BODY"
        return 1
    fi
    
    echo ""
    
    # Test 3: Initialize
    echo "📋 Test 3: MCP Initialize"
    echo "-----------------------------------"
    INIT_RESPONSE=$(curl -s -X POST "$url/mcp" \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}' 2>&1 || echo "ERROR")
    
    if echo "$INIT_RESPONSE" | grep -q "jsonrpc"; then
        echo "✅ Initialize successful"
        echo "$INIT_RESPONSE" | jq . 2>/dev/null || echo "$INIT_RESPONSE"
    else
        echo "❌ Initialize failed"
        echo "$INIT_RESPONSE"
        return 1
    fi
    
    echo ""
    
    # Test 4: Tools List
    echo "📋 Test 4: Tools List"
    echo "-----------------------------------"
    TOOLS_RESPONSE=$(curl -s -X POST "$url/mcp" \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' 2>&1 || echo "ERROR")
    
    if echo "$TOOLS_RESPONSE" | grep -q "search"; then
        echo "✅ Tools list successful"
        echo "$TOOLS_RESPONSE" | jq '.result.tools[] | {name: .name, description: .description}' 2>/dev/null || echo "$TOOLS_RESPONSE"
    else
        echo "❌ Tools list failed"
        echo "$TOOLS_RESPONSE"
        return 1
    fi
    
    echo ""
    
    # Test 5: Search Tool
    echo "📋 Test 5: Search Tool"
    echo "-----------------------------------"
    SEARCH_RESPONSE=$(curl -s -X POST "$url/mcp" \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search","arguments":{"term":"test"}}}' 2>&1 || echo "ERROR")
    
    if echo "$SEARCH_RESPONSE" | grep -q "hash"; then
        echo "✅ Search tool working"
        RESULT_COUNT=$(echo "$SEARCH_RESPONSE" | jq '.result.structuredContent.items | length' 2>/dev/null || echo "?")
        echo "   Found $RESULT_COUNT results"
    else
        echo "⚠️  Search tool response:"
        echo "$SEARCH_RESPONSE" | jq . 2>/dev/null | head -20 || echo "$SEARCH_RESPONSE" | head -20
    fi
    
    echo ""
    echo "✅ All tests completed!"
    return 0
}

# Try to get tunnel URL from Pi if not provided
if [ -z "$TUNNEL_URL" ]; then
    if [ -n "$PI_PASS" ]; then
        echo "🔌 Attempting to get tunnel URL from Raspberry Pi..."
        echo ""
        
        # Check if server is running
        if sshpass -p "$PI_PASS" ssh -o StrictHostKeyChecking=no "$PI_USER@$PI_HOST" \
            "systemctl is-active annas-mcp >/dev/null 2>&1" 2>/dev/null; then
            echo "✅ MCP server service is active"
        else
            echo "⚠️  MCP server service is not active"
        fi
        
        # Get Cloudflare tunnel URL from logs
        TUNNEL_URL=$(sshpass -p "$PI_PASS" ssh -o StrictHostKeyChecking=no "$PI_USER@$PI_HOST" \
            "journalctl -u cloudflared-tunnel -n 100 2>/dev/null | grep -oP 'https://[a-z0-9-]+\\.trycloudflare\\.com' | tail -1" 2>/dev/null || echo "")
        
        if [ -n "$TUNNEL_URL" ]; then
            echo "✅ Found Cloudflare tunnel URL: $TUNNEL_URL"
            echo ""
        else
            echo "⚠️  Could not find quick tunnel URL in logs"
            echo ""
            echo "Checking for named tunnel..."
            # Try to get named tunnel info
            if [ -n "$CLOUDFLARE_API_TOKEN" ]; then
                TUNNEL_INFO=$(sshpass -p "$PI_PASS" ssh -o StrictHostKeyChecking=no "$PI_USER@$PI_HOST" \
                    "export CLOUDFLARE_API_TOKEN='${CLOUDFLARE_API_TOKEN}' && /tmp/cloudflared tunnel list 2>&1" 2>/dev/null || echo "")
                
                if echo "$TUNNEL_INFO" | grep -q "annas-mcp"; then
                    echo "✅ Named tunnel 'annas-mcp' found"
                    echo "   Check Cloudflare dashboard for hostname:"
                    echo "   https://one.dash.cloudflare.com -> Networks -> Tunnels"
                    echo ""
                    echo "   Or provide the tunnel URL manually:"
                    echo "   ./scripts/test-cloudflare-tunnel.sh https://your-tunnel-hostname.com"
                    exit 0
                fi
            fi
            
            echo "❌ Could not determine tunnel URL automatically"
            echo ""
            echo "Please provide the tunnel URL manually:"
            echo "  ./scripts/test-cloudflare-tunnel.sh https://your-tunnel-url.trycloudflare.com"
            echo ""
            echo "Or check Cloudflare dashboard for named tunnel hostname"
            exit 1
        fi
    else
        echo "❌ No tunnel URL provided and PI_PASS not set"
        echo ""
        echo "Usage:"
        echo "  ./scripts/test-cloudflare-tunnel.sh https://your-tunnel-url.trycloudflare.com"
        echo ""
        echo "Or set PI_PASS in .env to auto-detect from Pi"
        exit 1
    fi
fi

# Ensure URL doesn't have trailing slash
TUNNEL_URL="${TUNNEL_URL%/}"

# Ensure URL has https:// prefix
if [[ ! "$TUNNEL_URL" =~ ^https?:// ]]; then
    TUNNEL_URL="https://$TUNNEL_URL"
fi

# Test the tunnel URL
test_tunnel_url "$TUNNEL_URL"

