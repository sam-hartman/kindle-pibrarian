# Anna's Archive MCP Server (and CLI Tool)

[An MCP server](https://modelcontextprotocol.io/introduction) and CLI tool for searching and downloading documents from [Anna's Archive](https://annas-archive.org)

## ⚠️ Legal Disclaimer

**IMPORTANT: This software is intended for LEGAL USE ONLY.**

This tool is designed to help you access and download materials that are:
- **Public domain** works
- **Creative Commons** licensed materials
- **Open access** academic papers and publications
- Other materials that are **legally available** for download

**DO NOT use this software to:**
- Download copyrighted materials without proper authorization
- Access or distribute content in violation of copyright laws
- Bypass any legal restrictions on content access

**By using this software, you agree to:**
- Use it only for legally permissible purposes
- Respect intellectual property rights
- Comply with all applicable copyright laws in your jurisdiction
- Accept full responsibility for your use of this tool

The authors and contributors of this software:
- Do not endorse or condone copyright infringement
- Are not responsible for any illegal use of this software
- Provide this tool "as-is" for educational and legal purposes only

**If you are unsure whether a work is in the public domain or legally available, do not download it. Consult with legal counsel if necessary.**

## Available Operations

| Operation                                                                      | MCP Tool   | CLI Command |
| ------------------------------------------------------------------------------ | ---------- | ----------- |
| Search Anna's Archive for documents matching specified terms                   | `search`   | `search`    |
| Download a specific document that was previously returned by the `search` tool | `download` | `download`  |

## Requirements

- [A donation to Anna's Archive](https://annas-archive.org/donate), which grants JSON API access
- [An API key](https://annas-archive.org/faq#api)
- Go 1.23+ (if building from source)

For MCP server functionality, you also need an MCP client, such as:
- [Claude Desktop](https://claude.ai/download)
- [Mistral Le Chat](https://lechat.mistral.ai/)

## Quick Start

### 1. Clone and Build

```bash
git clone <repository-url>
cd annas-mcp
go build -o annas-mcp ./cmd/annas-mcp
```

### 2. Configure

Create a `.env` file from the template:

```bash
cp .env.example .env
```

Edit `.env` and fill in your values:

```bash
# Required: Anna's Archive API key
ANNAS_SECRET_KEY=your-api-key-here

# Optional: Download path (defaults to temp directory)
ANNAS_DOWNLOAD_PATH=/path/to/downloads

# Optional: Email settings for Kindle (see KINDLE_EMAIL_SETUP.md)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-email@gmail.com
SMTP_PASSWORD=your-app-password
FROM_EMAIL=your-email@gmail.com
KINDLE_EMAIL=your-kindle-email@kindle.com
```

### 3. Start the Server

**Option A: Using the convenience script (recommended)**

```bash
./start-http-server.sh
```

**Option B: Direct command**

```bash
./annas-mcp http --port 8080
```

The server will start on `http://localhost:8080`

### 4. Connect Your MCP Client

#### For Mistral Le Chat

See [LE_CHAT_SETUP.md](LE_CHAT_SETUP.md) for detailed instructions.

Quick setup:
1. Open Le Chat → Intelligence → Connectors
2. Add Custom MCP Connector
3. Set **Connector Server** to: `http://localhost:8080/mcp`
4. Click Connect

#### For Claude Desktop

Add to your MCP configuration file:

```json
{
  "mcpServers": {
    "annas-mcp": {
      "command": "/path/to/annas-mcp",
      "args": ["mcp"],
      "env": {
        "ANNAS_SECRET_KEY": "your-api-key",
        "ANNAS_DOWNLOAD_PATH": "/path/to/downloads"
      }
    }
  }
}
```

## Usage

### As an HTTP Server

The server exposes these endpoints:

- `GET /health` - Health check
- `POST /search` - Search for books
- `POST /download` - Download a book
- `POST /mcp` - MCP protocol endpoint (for MCP clients)

### As a CLI Tool

```bash
# Search for books
./annas-mcp search "python programming"

# Download a book
./annas-mcp download <hash> <filename>
```

## Features

- 🔍 Search Anna's Archive for books and documents
- 📥 Download books directly to your device
- 📧 **Email books directly to your Kindle** (see [KINDLE_EMAIL_SETUP.md](KINDLE_EMAIL_SETUP.md))
- 🔌 MCP server support for AI assistants
- 🌐 HTTP server mode for web-based clients
- 🔒 Secure configuration via `.env` file

## Documentation

- [LE_CHAT_SETUP.md](LE_CHAT_SETUP.md) - Setup guide for Mistral Le Chat
- [KINDLE_EMAIL_SETUP.md](KINDLE_EMAIL_SETUP.md) - Guide for emailing books to Kindle

## Demo

### As an MCP Server

<img src="screenshots/claude.png" width="600px"/>

### As a CLI Tool

<img src="screenshots/cli.png" width="400px"/>
