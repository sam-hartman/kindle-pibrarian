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
   - **Connector Server**: `https://your-pi-hostname.tailnet-name.ts.net/mcp` (your Tailscale Funnel URL)
   - **Description**: (optional) `Search and download books from Anna's Archive`
   - **Authentication Method**: `No Authentication`
6. Click **Connect**

**Important**: Make sure to include the `/mcp` path at the end of the URL. This is the MCP protocol endpoint.

### 4. Use the Connector

1. In a Le Chat conversation, click the **Tools** button
2. Under **Connectors**, enable **Anna's Archive MCP**
3. You can now ask Le Chat to search for books or download them!

## Remote Access with Tailscale Funnel

For Le Chat to access your server, you need to expose it to the internet. We use Tailscale Funnel:

```bash
# Install Tailscale (on your server/Pi)
curl -fsSL https://tailscale.com/install.sh | sh
sudo tailscale up

# Enable Funnel
sudo tailscale funnel --bg 8081
```

Your URL will be: `https://your-hostname.tailnet-name.ts.net`

Use this URL in Le Chat: `https://your-hostname.tailnet-name.ts.net/mcp`

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
   - Correct: `https://your-hostname.ts.net/mcp`
   - Wrong: `https://your-hostname.ts.net`

2. **Test the endpoint manually**: Use the curl commands above to verify the server is responding correctly.

3. **Check server logs**: The server logs all MCP requests. Look for any error messages when Le Chat tries to connect.

4. **Using Tailscale Funnel**: Make sure:
   - Tailscale is connected: `tailscale status`
   - Funnel is running: `tailscale funnel status`
   - Test with curl: `curl https://your-hostname.ts.net/health`

5. **CORS Issues**: The server has CORS enabled, but if you still see CORS errors, check that the server is running and accessible.
