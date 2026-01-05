# Mistral Le Chat Setup Guide

## Quick Start

### 1. Set Up Configuration

Create a `.env` file from the template:

```bash
cp .env.example .env
```

Then edit `.env` and fill in your values:
- `ANNAS_SECRET_KEY` - Your Anna's Archive API key (required for downloads)
- `ANNAS_DOWNLOAD_PATH` - Where to save downloaded books (optional)
- Email settings (optional - only needed if emailing to Kindle)

**Note**: The `.env` file is gitignored and will not be committed. All sensitive values should go here.

### 2. Start the HTTP Server

```bash
cd "/Users/samuelhartman/annas mcp/annas-mcp"
./annas-mcp http --port 8080
```

Or use the convenience script:
```bash
./scripts/start-server.sh
```

The server will start on `http://localhost:8080`

### 3. Configure in Mistral Le Chat

1. Open Le Chat
2. Navigate to **Intelligence** → **Connectors**
3. Click **+ Add Connector**
4. Select **Custom MCP Connector** tab
5. Fill in:
   - **Connector Name**: `Anna's Archive MCP`
   - **Connector Server**: `http://localhost:8080/mcp` (or use Cloudflare URL: `https://your-tunnel-url.trycloudflare.com/mcp`)
   - **Description**: (optional) `Search and download books from Anna's Archive`
   - **Authentication Method**: `No Authentication`
6. Click **Connect**

**Important**: Make sure to include the `/mcp` path at the end of the URL. This is the MCP protocol endpoint.

### 4. Use the Connector

1. In a Le Chat conversation, click the **Tools** button
2. Under **Connectors**, enable **Anna's Archive MCP**
3. You can now ask Le Chat to search for books or download them!

## Available Endpoints

- `GET /health` - Health check endpoint
- `POST /search` - Search for books
  ```json
  {
    "term": "python programming"
  }
  ```
- `POST /download` - Download a book
  ```json
  {
    "hash": "md5-hash-here",
    "title": "Book Title",
    "format": "pdf"
  }
  ```

## Testing the Server

```bash
# Health check
curl http://localhost:8080/health

# Test MCP endpoint (what Le Chat uses)
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'

# List available tools
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'

# Search (alternative endpoint)
curl -X POST http://localhost:8080/search \
  -H "Content-Type: application/json" \
  -d '{"term":"python"}'
```

## Troubleshooting

### Connection Issues

If Le Chat cannot connect:

1. **Check the URL format**: Make sure you're using the full path with `/mcp`:
   - ✅ Correct: `http://localhost:8080/mcp`
   - ❌ Wrong: `http://localhost:8080`

2. **Test the endpoint manually**: Use the curl commands above to verify the server is responding correctly.

3. **Check server logs**: The server logs all MCP requests. Look for any error messages when Le Chat tries to connect.

4. **Using Cloudflare Tunnel**: If using a Cloudflare tunnel, make sure to:
   - Use the full URL: `https://your-tunnel-url.trycloudflare.com/mcp`
   - The tunnel must be running and forwarding to `http://localhost:8080`
   - Test the Cloudflare URL with curl first to ensure it's accessible

5. **CORS Issues**: The server has CORS enabled, but if you still see CORS errors, check that the server is running and accessible.

