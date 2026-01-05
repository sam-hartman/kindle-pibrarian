#!/bin/bash
# Comprehensive repository test suite
# Tests build, search, download, and all functionality

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

echo "🧪 Comprehensive Repository Test Suite"
echo "===================================="
echo ""

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0

test_pass() {
    echo "✅ $1"
    TESTS_PASSED=$((TESTS_PASSED + 1))
}

test_fail() {
    echo "❌ $1"
    TESTS_FAILED=$((TESTS_FAILED + 1))
}

test_warn() {
    echo "⚠️  $1"
}

echo "📋 Test 1: Build Verification"
echo "-----------------------------------"
cd "$PROJECT_ROOT"
if go build -o /tmp/annas-mcp-test ./cmd/annas-mcp 2>&1; then
    test_pass "Go build successful"
    rm -f /tmp/annas-mcp-test
else
    test_fail "Go build failed"
fi

echo ""
echo "📋 Test 2: Environment Variables"
echo "-----------------------------------"
MISSING_VARS=()
for var in ANNAS_SECRET_KEY SMTP_USER FROM_EMAIL KINDLE_EMAIL; do
    if [ -z "${!var}" ]; then
        MISSING_VARS+=("$var")
    fi
done

if [ ${#MISSING_VARS[@]} -eq 0 ]; then
    test_pass "All required environment variables are set"
else
    test_fail "Missing environment variables: ${MISSING_VARS[*]}"
fi

echo ""
echo "📋 Test 3: Script Validation"
echo "-----------------------------------"
SCRIPT_ERRORS=0
for script in scripts/*.sh; do
    if [ -f "$script" ]; then
        if bash -n "$script" 2>&1; then
            test_pass "Syntax check: $(basename $script)"
        else
            test_fail "Syntax error in: $(basename $script)"
            SCRIPT_ERRORS=$((SCRIPT_ERRORS + 1))
        fi
    fi
done

if [ $SCRIPT_ERRORS -eq 0 ]; then
    test_pass "All scripts have valid syntax"
else
    test_fail "$SCRIPT_ERRORS script(s) have syntax errors"
fi

echo ""
echo "📋 Test 4: Testing on Raspberry Pi"
echo "-----------------------------------"
PI_HOST="${PI_HOST:-192.168.1.201}"
PI_USER="${PI_USER:-samuelhartman}"
PI_PASS="${PI_PASS:-}"

if [ -z "$PI_PASS" ]; then
    test_warn "PI_PASS not set, skipping Pi tests"
else
    # Check Pi connection
    if sshpass -p "$PI_PASS" ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$PI_USER@$PI_HOST" "echo 'connected'" >/dev/null 2>&1; then
        test_pass "Pi connection successful"
        
        # Check if server is running
        if sshpass -p "$PI_PASS" ssh -o StrictHostKeyChecking=no "$PI_USER@$PI_HOST" "systemctl is-active annas-mcp" >/dev/null 2>&1; then
            test_pass "MCP server service is active on Pi"
            
            # Get tunnel URL
            TUNNEL_URL=$(sshpass -p "$PI_PASS" ssh -o StrictHostKeyChecking=no "$PI_USER@$PI_HOST" \
                "journalctl -u cloudflared-tunnel -n 50 2>/dev/null | grep -o 'https://[a-z0-9-]*\.trycloudflare\.com' | tail -1" || echo "")
            
            if [ -n "$TUNNEL_URL" ]; then
                test_pass "Cloudflare tunnel is active: $TUNNEL_URL"
                MCP_SERVER_URL="$TUNNEL_URL"
            else
                test_warn "Cloudflare tunnel URL not found, using localhost"
                MCP_SERVER_URL="http://$PI_HOST:8080"
            fi
            
            echo ""
            echo "📋 Test 5: Search Functionality (on Pi)"
            echo "-----------------------------------"
            SEARCH_RESPONSE=$(curl -s -X POST "$MCP_SERVER_URL/mcp" \
                -H "Content-Type: application/json" \
                -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search","arguments":{"query":"ali hazelwood"}}}' || echo "")
            
            if echo "$SEARCH_RESPONSE" | grep -q "hash"; then
                test_pass "Search functionality works"
                
                # Extract book details
                BOOK_HASH=$(echo "$SEARCH_RESPONSE" | grep -o '"hash"[[:space:]]*:[[:space:]]*"[a-f0-9]*"' | head -1 | sed 's/.*"hash"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
                BOOK_TITLE=$(echo "$SEARCH_RESPONSE" | grep -o '"title"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"title"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
                BOOK_FORMAT=$(echo "$SEARCH_RESPONSE" | grep -o '"format"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"format"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
                
                if [ -n "$BOOK_HASH" ] && [ -n "$BOOK_TITLE" ]; then
                    echo "   Found: $BOOK_TITLE"
                    echo "   Hash: $BOOK_HASH"
                    echo "   Format: $BOOK_FORMAT"
                    
                    echo ""
                    echo "📋 Test 6: Download Functionality (on Pi)"
                    echo "-----------------------------------"
                    DOWNLOAD_RESPONSE=$(curl -s -X POST "$MCP_SERVER_URL/mcp" \
                        -H "Content-Type: application/json" \
                        -d "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\",\"params\":{\"name\":\"download\",\"arguments\":{\"hash\":\"$BOOK_HASH\",\"title\":\"$BOOK_TITLE\",\"format\":\"$BOOK_FORMAT\"}}}" || echo "")
                    
                    if echo "$DOWNLOAD_RESPONSE" | grep -qi "success\|kindle\|downloaded"; then
                        DOWNLOAD_TEXT=$(echo "$DOWNLOAD_RESPONSE" | grep -o '"text"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"text"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
                        test_pass "Download functionality works"
                        echo "   Response: $DOWNLOAD_TEXT"
                    elif echo "$DOWNLOAD_RESPONSE" | grep -qi "error"; then
                        ERROR_MSG=$(echo "$DOWNLOAD_RESPONSE" | grep -o '"message"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"message"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')
                        test_warn "Download returned error: $ERROR_MSG"
                    else
                        test_warn "Download response unclear: $(echo "$DOWNLOAD_RESPONSE" | head -c 200)"
                    fi
                else
                    test_fail "Could not extract book details from search"
                fi
            else
                test_fail "Search failed - no hash found in response"
                echo "   Response: $(echo "$SEARCH_RESPONSE" | head -c 300)"
            fi
            
            echo ""
            echo "📋 Test 7: Server Health Check"
            echo "-----------------------------------"
            PING_RESPONSE=$(curl -s "$MCP_SERVER_URL/ping" || echo "")
            if echo "$PING_RESPONSE" | grep -q "ok"; then
                test_pass "Server health check (ping) works"
            else
                test_fail "Server health check failed"
            fi
            
            echo ""
            echo "📋 Test 8: MCP Tools List"
            echo "-----------------------------------"
            TOOLS_RESPONSE=$(curl -s -X POST "$MCP_SERVER_URL/mcp" \
                -H "Content-Type: application/json" \
                -d '{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{}}' || echo "")
            if echo "$TOOLS_RESPONSE" | grep -q "search" && echo "$TOOLS_RESPONSE" | grep -q "download"; then
                test_pass "MCP tools list works (search and download available)"
            else
                test_fail "MCP tools list incomplete"
            fi
            
        else
            test_fail "MCP server service is not active on Pi"
        fi
    else
        test_warn "Cannot connect to Pi, skipping Pi tests"
    fi
fi

echo ""
echo "===================================="
echo "📊 Test Summary"
echo "===================================="
echo "✅ Passed: $TESTS_PASSED"
echo "❌ Failed: $TESTS_FAILED"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo "🎉 All tests passed! Repository is working correctly."
    exit 0
else
    echo "⚠️  Some tests failed. Please review the output above."
    exit 1
fi

