#!/bin/bash
# Test script for the one-liner installer
# Run this on the target machine (e.g., Pi) to verify installation works
#
# Usage: 
#   ./tests/test-installer.sh              # Full test (with Tailscale)
#   ./tests/test-installer.sh --quick      # Quick test (skip Tailscale)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load .env for API key
if [ -f "$PROJECT_ROOT/.env" ]; then
    set -a
    source "$PROJECT_ROOT/.env"
    set +a
fi

QUICK_MODE=0
if [ "$1" = "--quick" ] || [ "$1" = "-q" ]; then
    QUICK_MODE=1
fi

echo "=============================================="
echo "  Installer Test Script"
echo "=============================================="
echo ""

# Step 1: Clean up existing installation
echo "Step 1: Cleaning up existing installation..."
sudo systemctl stop annas-mcp 2>/dev/null || true
sudo systemctl disable annas-mcp 2>/dev/null || true
sudo rm -f /etc/systemd/system/annas-mcp.service
sudo systemctl daemon-reload
rm -rf "$HOME/annas-mcp-server"
echo "  Done"
echo ""

# Step 2: Run installer
echo "Step 2: Running installer..."
if [ $QUICK_MODE -eq 1 ]; then
    echo "  (Quick mode: skipping Tailscale)"
    export SKIP_TAILSCALE=1
fi

# Pass API key via environment
export ANNAS_SECRET_KEY="${ANNAS_SECRET_KEY:-test-key-for-testing}"

# Run the installer
bash "$PROJECT_ROOT/install.sh"

echo ""

# Step 3: Verify installation
echo "Step 3: Verifying installation..."
TESTS_PASSED=0
TESTS_FAILED=0

# Check binary exists
if [ -x "$HOME/annas-mcp-server/annas-mcp" ]; then
    echo "  [PASS] Binary installed"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo "  [FAIL] Binary not found"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

# Check .env exists
if [ -f "$HOME/annas-mcp-server/.env" ]; then
    echo "  [PASS] .env created"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo "  [FAIL] .env not found"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

# Check service is running
if systemctl is-active annas-mcp >/dev/null 2>&1; then
    echo "  [PASS] Service is running"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo "  [FAIL] Service not running"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

# Check HTTP endpoint responds
sleep 2
if curl -s http://localhost:8081/ping | grep -q "ok"; then
    echo "  [PASS] HTTP endpoint responds"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo "  [FAIL] HTTP endpoint not responding"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

# Check Tailscale (unless quick mode)
if [ $QUICK_MODE -eq 0 ]; then
    if tailscale funnel status 2>/dev/null | grep -q "8081"; then
        echo "  [PASS] Tailscale Funnel configured"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo "  [WARN] Tailscale Funnel not configured (may need manual setup)"
    fi
fi

echo ""
echo "=============================================="
echo "  Results: $TESTS_PASSED passed, $TESTS_FAILED failed"
echo "=============================================="

if [ $TESTS_FAILED -eq 0 ]; then
    echo "All tests passed!"
    exit 0
else
    echo "Some tests failed."
    exit 1
fi
