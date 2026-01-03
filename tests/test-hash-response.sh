#!/bin/bash
# Test script to verify hash is returned in search results

TUNNEL_URL="https://dental-mirror-cross-hub.trycloudflare.com/mcp"

echo "🧪 Testing search response for hash inclusion..."
echo ""

# Test 1: Search for a book
echo "Test 1: Searching for 'python'..."
RESPONSE=$(curl -s -X POST "$TUNNEL_URL" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search","arguments":{"term":"python"}}}')

# Check if structuredContent exists
if echo "$RESPONSE" | python3 -c "import sys, json; data=json.load(sys.stdin); exit(0 if 'structuredContent' in data.get('result', {}) else 1)" 2>/dev/null; then
    echo "✅ structuredContent field is present!"
    
    # Extract and display hashes
    echo ""
    echo "📋 Book hashes found:"
    echo "$RESPONSE" | python3 -c "
import sys, json
data = json.load(sys.stdin)
structured = data.get('result', {}).get('structuredContent', [])
if structured:
    for i, book in enumerate(structured, 1):
        hash_val = book.get('hash', 'N/A')
        title = book.get('title', 'N/A')[:50]
        print(f\"  {i}. Hash: {hash_val}\")
        print(f\"     Title: {title}\")
else:
    print('  No books in structuredContent')
" 2>/dev/null || echo "  (Could not parse structuredContent)"
else
    echo "❌ structuredContent field is NOT present"
    echo "   This means the update hasn't been deployed yet."
fi

echo ""
echo "📄 Full response (first 500 chars):"
echo "$RESPONSE" | head -c 500
echo "..."

echo ""
echo "Test 2: Checking text content for hash..."
if echo "$RESPONSE" | grep -q "Hash:"; then
    echo "✅ Hash found in text content"
    echo "$RESPONSE" | grep -o "Hash: [a-f0-9]*" | head -3
else
    echo "⚠️  No hash found in text (might be 'No books found')"
fi

