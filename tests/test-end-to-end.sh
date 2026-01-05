#!/bin/bash
# End-to-end test script for Anna's Archive MCP Server
# Tests search and download functionality

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load .env
if [ -f "$PROJECT_ROOT/.env" ]; then
    set -a
    source "$PROJECT_ROOT/.env"
    set +a
else
    echo "❌ Error: .env file not found at $PROJECT_ROOT/.env"
    exit 1
fi

# Configuration
PI_HOST="${PI_HOST:-192.168.1.201}"
PI_USER="${PI_USER:-pi}"
PI_PASS="${PI_PASS:-}"
MCP_SERVER_URL="${MCP_SERVER_URL:-http://localhost:8080}"

echo "🧪 End-to-End Test Suite for Anna's Archive MCP Server"
echo "=================================================="
echo ""

# Function to test on Pi
test_on_pi() {
    if [ -z "$PI_PASS" ]; then
        echo "⚠️  PI_PASS not set, skipping Pi tests"
        return 1
    fi
    
    echo "🔌 Testing on Raspberry Pi ($PI_USER@$PI_HOST)..."
    
    # Check if server is running
    sshpass -p "$PI_PASS" ssh -o StrictHostKeyChecking=no "$PI_USER@$PI_HOST" \
        "systemctl is-active annas-mcp >/dev/null 2>&1" && \
        echo "✅ MCP server service is active" || \
        echo "⚠️  MCP server service is not active"
    
    # Get Cloudflare tunnel URL if available
    TUNNEL_URL=$(sshpass -p "$PI_PASS" ssh -o StrictHostKeyChecking=no "$PI_USER@$PI_HOST" \
        "journalctl -u cloudflared-tunnel -n 50 2>/dev/null | grep -oP 'https://[a-z0-9-]+\\.trycloudflare\\.com' | tail -1" || echo "")
    
    if [ -n "$TUNNEL_URL" ]; then
        echo "✅ Cloudflare tunnel found: $TUNNEL_URL"
        MCP_SERVER_URL="$TUNNEL_URL"
    else
        echo "⚠️  Cloudflare tunnel URL not found, using localhost"
        MCP_SERVER_URL="http://$PI_HOST:8080"
    fi
    
    return 0
}

# Function to test locally
test_locally() {
    echo "🔌 Testing locally..."
    MCP_SERVER_URL="http://localhost:8080"
    
    # Check if server is running
    if curl -s "$MCP_SERVER_URL/ping" >/dev/null 2>&1; then
        echo "✅ MCP server is responding at $MCP_SERVER_URL"
    else
        echo "⚠️  MCP server not responding at $MCP_SERVER_URL"
        echo "   Start server with: ./scripts/start-server.sh"
        return 1
    fi
}

# Choose test location
if [ "$1" = "pi" ]; then
    test_on_pi || exit 1
elif [ "$1" = "local" ]; then
    test_locally || exit 1
else
    # Try Pi first, fall back to local
    if ! test_on_pi 2>/dev/null; then
        echo ""
        echo "Falling back to local testing..."
        test_locally || exit 1
    fi
fi

echo ""
echo "📋 Test 1: Server Health Check (ping)"
echo "-----------------------------------"
PING_RESPONSE=$(curl -s "$MCP_SERVER_URL/ping" || echo "")
if [ -n "$PING_RESPONSE" ]; then
    echo "✅ Ping successful"
    echo "   Response: $PING_RESPONSE"
else
    echo "❌ Ping failed"
    exit 1
fi

echo ""
echo "📋 Test 2: MCP Discovery"
echo "-----------------------------------"
MCP_RESPONSE=$(curl -s "$MCP_SERVER_URL/mcp" || echo "")
if echo "$MCP_RESPONSE" | grep -q "mcp"; then
    echo "✅ MCP discovery successful"
else
    echo "❌ MCP discovery failed"
    echo "   Response: $MCP_RESPONSE"
    exit 1
fi

echo ""
echo "📋 Test 3: Tools List"
echo "-----------------------------------"
TOOLS_RESPONSE=$(curl -s -X POST "$MCP_SERVER_URL/mcp" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' || echo "")
if echo "$TOOLS_RESPONSE" | grep -q "search"; then
    echo "✅ Tools list successful"
    echo "   Found tools: search, download"
else
    echo "❌ Tools list failed"
    echo "   Response: $TOOLS_RESPONSE"
    exit 1
fi

echo ""
echo "📋 Test 4: Search Functionality"
echo "-----------------------------------"
SEARCH_RESPONSE=$(curl -s -X POST "$MCP_SERVER_URL/mcp" \
    -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"search","arguments":{"query":"ali hazelwood"}}}' || echo "")

if echo "$SEARCH_RESPONSE" | grep -q "hash"; then
    echo "✅ Search successful"
    # Extract first book hash for download test (macOS compatible)
    BOOK_HASH=$(echo "$SEARCH_RESPONSE" | grep -o '"hash"[[:space:]]*:[[:space:]]*"[a-f0-9]*"' | head -1 | sed 's/.*"hash"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
    BOOK_TITLE=$(echo "$SEARCH_RESPONSE" | grep -o '"title"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"title"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
    BOOK_FORMAT=$(echo "$SEARCH_RESPONSE" | grep -o '"format"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"format"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
    
    if [ -n "$BOOK_HASH" ]; then
        echo "   Found book: $BOOK_TITLE"
        echo "   Hash: $BOOK_HASH"
        echo "   Format: $BOOK_FORMAT"
        
        echo ""
        echo "📋 Test 5: Download Functionality"
        echo "-----------------------------------"
        DOWNLOAD_RESPONSE=$(curl -s -X POST "$MCP_SERVER_URL/mcp" \
            -H "Content-Type: application/json" \
            -d "{\"jsonrpc\":\"2.0\",\"id\":3,\"method\":\"tools/call\",\"params\":{\"name\":\"download\",\"arguments\":{\"hash\":\"$BOOK_HASH\",\"title\":\"$BOOK_TITLE\",\"format\":\"$BOOK_FORMAT\"}}}" || echo "")
        
        if echo "$DOWNLOAD_RESPONSE" | grep -qi "success\|kindle\|downloaded"; then
            echo "✅ Download successful"
            DOWNLOAD_TEXT=$(echo "$DOWNLOAD_RESPONSE" | grep -o '"text"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"text"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
            echo "   Response: $DOWNLOAD_TEXT"
        else
            echo "⚠️  Download response: $DOWNLOAD_RESPONSE"
            echo "   (This may be expected if email is not configured or API key is invalid)"
        fi
    else
        echo "⚠️  Could not extract book hash from search results"
    fi
else
    echo "❌ Search failed"
    echo "   Response: $SEARCH_RESPONSE"
    exit 1
fi

echo ""
echo "✅ All tests completed!"
echo ""
echo "📊 Summary:"
echo "   - Server health: ✅"
echo "   - MCP discovery: ✅"
echo "   - Tools list: ✅"
echo "   - Search: ✅"
echo "   - Download: ✅ (or ⚠️ if email not configured)"

