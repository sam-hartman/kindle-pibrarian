# Anna's Archive MCP Server (and CLI Tool)

[An MCP server](https://modelcontextprotocol.io/introduction) and CLI tool for searching and downloading documents from [Anna's Archive](https://annas-archive.se)

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

**Note:** The `download` tool supports an optional `kindle_email` parameter. If not provided, it uses the default `KINDLE_EMAIL` from your `.env` file.

## Requirements

- [A donation to Anna's Archive](https://annas-archive.se/donate), which grants JSON API access
- [An API key](https://annas-archive.se/faq#api)
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

Create a `.env` file (see `.env.example` for template):

```bash
# Copy the example if it exists, or create manually
cp .env.example .env 2>/dev/null || touch .env
```

Edit `.env` and fill in your values:

```bash
# Required: Anna's Archive API key
ANNAS_SECRET_KEY=your-api-key-here

# Optional: Download path (defaults to temp directory)
ANNAS_DOWNLOAD_PATH=/path/to/downloads

# Optional: Email settings for Kindle (see detailed instructions below)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-email@gmail.com
SMTP_PASSWORD=your-app-password
FROM_EMAIL=your-email@gmail.com
KINDLE_EMAIL=your-kindle-email@kindle.com
```

### 2.1. Setting Up Kindle Email (Optional)

If you want to email books directly to your Kindle, follow these steps:

#### Step 1: Get a Gmail App Password

1. **Enable 2-Factor Authentication** on your Google account (required for app passwords):
   - Go to https://myaccount.google.com/security
   - Under "Signing in to Google", click "2-Step Verification"
   - Follow the prompts to enable it

2. **Create an App Password**:
   - Go to https://myaccount.google.com/apppasswords
   - You may need to sign in again
   - Under "Select app", choose **"Mail"**
   - Under "Select device", choose **"Other (Custom name)"**
   - Enter a name like "Anna's Archive MCP" and click "Generate"
   - **Copy the 16-character password** (it will look like: `abcd efgh ijkl mnop`)
   - This is your `SMTP_PASSWORD` - use it exactly as shown (with or without spaces, both work)

#### Step 2: Find Your Kindle Email Address

1. Go to [Amazon's Manage Your Content and Devices page](https://www.amazon.com/hz/mycd/digital-console/alldevices)
2. Sign in with your Amazon account
3. Click on **"Settings"** (or go to **"Preferences"** → **"Personal Document Settings"**)
4. Scroll down to **"Send-to-Kindle Email Settings"**
5. You'll see your Kindle email address(es) listed (e.g., `yourname_123@kindle.com` or `yourname_123@free.kindle.com`)
6. **Copy this email address** - this is your `KINDLE_EMAIL`

**Note:** If you have multiple Kindles, each will have its own email address. Use the one for the device you want to receive books.

#### Step 3: Whitelist Your Email Address in Kindle Settings

**Important:** Amazon will reject emails from addresses that aren't whitelisted. You must add your sending email to the approved list.

1. On the same **"Personal Document Settings"** page (from Step 2)
2. Scroll to **"Approved Personal Document E-mail List"**
3. Click **"Add a new approved e-mail address"**
4. Enter the email address you'll use to send books (your `FROM_EMAIL` - typically your Gmail address)
5. Click **"Add Address"**
6. You should see a confirmation message

**Note:** It may take a few minutes for the whitelist to take effect. If emails are rejected, wait 5-10 minutes and try again.

#### Step 4: Update Your `.env` File

After completing the above steps, update your `.env` file with:

```bash
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your-email@gmail.com
SMTP_PASSWORD=abcd efgh ijkl mnop  # Your 16-character app password
FROM_EMAIL=your-email@gmail.com    # Must match the whitelisted email
KINDLE_EMAIL=yourname_123@kindle.com  # Your Kindle email from Step 2
```

### 3. Start the Server

**Option A: Using the convenience script (recommended)**

```bash
./scripts/start-server.sh
```

**Option B: Direct command**

```bash
./annas-mcp http --port 8080
```

The server will start on `http://localhost:8080`

### 4. Connect Your MCP Client

#### For Mistral Le Chat

See [docs/LE_CHAT_SETUP.md](docs/LE_CHAT_SETUP.md) for detailed instructions.

Quick setup:
1. Open Le Chat → Intelligence → Connectors
2. Add Custom MCP Connector
3. Set **Connector Server** to: `http://localhost:8080/mcp`
4. Set **Authentication Method** to "None" (or "API Token" if required)
5. Click Create

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

- `GET /` - Server info and discovery
- `GET /mcp` - MCP protocol discovery
- `POST /mcp` - MCP protocol endpoint (JSON-RPC 2.0)
  - `initialize` - Initialize MCP connection
  - `ping` - Health check
  - `tools/list` - List available tools
  - `tools/call` - Execute a tool (search or download)
- `POST /search` - Direct search endpoint (legacy)
- `POST /download` - Direct download endpoint (legacy)

### As a CLI Tool

```bash
# Search for books
./annas-mcp search "python programming"

# Download a book
./annas-mcp download <hash> <filename>
```

## Features

- 🔍 **Search Anna's Archive** for books and documents with structured results
- 📥 **Download books** directly to your device or send to Kindle
- 📧 **Email books directly to your Kindle** with optional per-request email address
- 🔌 **MCP server support** for AI assistants (Claude Desktop, Mistral Le Chat)
- 🌐 **HTTP server mode** for web-based clients
- 📊 **Structured content** - Search results include book metadata in JSON format
- 🔒 **Secure configuration** via `.env` file
- 🛡️ **Duplicate protection** - Prevents sending the same book to the same Kindle multiple times

## MCP Tools

### `search`
Search for books on Anna's Archive. Returns a list of books with metadata including:
- Title, authors, publisher
- Format (epub, mobi, pdf, etc.)
- Language, size
- **MD5 hash** (required for download)

**Response includes:**
- Text content with formatted book list
- `structuredContent` field with JSON array of book objects (wrapped in `{"items": [...]}` for Le Chat compatibility)

### `download`
Download a book and send it to a Kindle email address.

**Parameters:**
- `hash` (required) - MD5 hash from search results
- `title` (required) - Book title
- `format` (required) - Book format (epub, mobi, pdf, etc.)
- `kindle_email` (optional) - Kindle email address. If not provided, uses `KINDLE_EMAIL` from `.env`

**Behavior:**
- Downloads book from Anna's Archive
- Saves locally as backup (if `ANNAS_DOWNLOAD_PATH` is set)
- Emails to specified Kindle email (or default if not specified)
- Falls back to local download only if email is not configured

## Documentation

- [docs/LE_CHAT_SETUP.md](docs/LE_CHAT_SETUP.md) - Setup guide for Mistral Le Chat
- [docs/KINDLE_EMAIL_SETUP.md](docs/KINDLE_EMAIL_SETUP.md) - Guide for emailing books to Kindle
- [docs/PROJECT_ORGANIZATION.md](docs/PROJECT_ORGANIZATION.md) - Project structure and organization standards
- [docs/SECURITY_AUDIT.md](docs/SECURITY_AUDIT.md) - Security considerations

## Project Structure

### File Structure

```
annas-mcp/
├── cmd/
│   └── annas-mcp/
│       └── main.go              # Application entry point
│
├── internal/                     # Internal packages (not exported)
│   ├── anna/                    # Anna's Archive API integration
│   │   ├── anna.go             # Search, download, email functionality
│   │   └── structs.go          # Book and API response structs
│   ├── logger/                  # Logging utilities
│   │   └── logger.go           # Structured logging setup
│   ├── modes/                   # Application modes (MCP, CLI, HTTP)
│   │   ├── mcpserver.go        # MCP server implementation
│   │   ├── cli.go              # CLI command handlers
│   │   ├── env.go              # Environment variable loading
│   │   └── params.go           # Tool parameter definitions
│   └── version/                 # Version management
│       ├── version.go          # Version retrieval
│       └── version.txt         # Version string
│
├── scripts/                      # Deployment and utility scripts
│   ├── deploy-on-pi.sh         # Deploy directly on Raspberry Pi
│   ├── deploy-to-pi.sh         # Deploy from Mac to Pi
│   ├── deploy-with-tunnel.sh   # Deploy with Cloudflare tunnel
│   ├── raspberry-pi-setup.sh   # Systemd service configuration
│   ├── setup-email-env.sh      # Email configuration helper
│   └── start-server.sh          # Start HTTP server (auto-detects email config)
│
├── docs/                         # Documentation
│   ├── LE_CHAT_SETUP.md        # Mistral Le Chat setup guide
│   ├── KINDLE_EMAIL_SETUP.md   # Kindle email configuration
│   ├── PI_TROUBLESHOOTING.md   # Raspberry Pi troubleshooting
│   ├── PROJECT_ORGANIZATION.md # File organization standards
│   ├── SECURITY_AUDIT.md       # Security considerations
│   └── AGENT_SYSTEM_PROMPT.md  # Agent system prompt
│
├── tests/                        # Test scripts and utilities
│   ├── test-repo-comprehensive.sh  # Comprehensive test suite
│   ├── test-end-to-end.sh       # End-to-end testing
│   └── diagnose-pi.sh           # Pi connectivity diagnostics
│
├── .env.example                  # Environment variable template
├── .gitignore                    # Git ignore rules
├── go.mod                        # Go module definition
├── go.sum                        # Go module checksums
└── README.md                     # This file
```

### Code Structure

**Entry Point** (`cmd/annas-mcp/main.go`):
- Parses command-line arguments
- Routes to appropriate mode (MCP, CLI, HTTP)

**Core Packages**:

- **`internal/anna/`** - Anna's Archive integration
  - `FindBook()` - Web scraping search functionality
  - `Download()` - Download books via API
  - `EmailToKindle()` - Email books to Kindle devices

- **`internal/modes/`** - Application modes
  - `StartMCPServer()` - MCP protocol server (stdio)
  - `StartMCPHTTPServer()` - HTTP-based MCP server
  - `SearchTool()` - MCP search tool implementation
  - `DownloadTool()` - MCP download tool implementation

- **`internal/logger/`** - Structured logging with zap

- **`internal/version/`** - Version management

**Data Flow**:
1. MCP client → `mcpserver.go` → `SearchTool()` → `anna.FindBook()`
2. Search results → `structuredContent` (wrapped in `{"items": [...]}`)
3. Download request → `DownloadTool()` → `anna.EmailToKindle()` or `anna.Download()`

## Deployment

For deployment scripts and setup guides, see:
- `scripts/deploy-on-pi.sh` - Deploy directly on Raspberry Pi
- `scripts/deploy-with-tunnel.sh` - Deploy with Cloudflare tunnel from Mac
- `scripts/raspberry-pi-setup.sh` - Systemd service setup
