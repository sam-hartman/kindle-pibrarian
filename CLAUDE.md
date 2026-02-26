# Claude Instructions for kindle-pibrarian

## Project Overview
MCP server for searching and downloading books from Anna's Archive, with Kindle email support. Deployed on Raspberry Pi with Tailscale Funnel for remote access.

**Repository:** https://github.com/sam-hartman/kindle-pibrarian

## Key Files
- `internal/anna/anna.go` - Core search/download/email logic
- `internal/modes/mcpserver.go` - MCP server implementation
- `scripts/deploy-on-pi.sh` - Main deployment script (includes Tailscale setup)

## Development Workflow
```bash
# Build locally
go build -o annas-mcp ./cmd/annas-mcp

# Build for Pi
GOOS=linux GOARCH=arm GOARM=7 go build -o annas-mcp-arm ./cmd/annas-mcp

# Deploy to Pi
sshpass -p 'test' scp annas-mcp-arm samuelhartman@192.168.1.201:~/annas-mcp-server/annas-mcp
sshpass -p 'test' ssh samuelhartman@192.168.1.201 "sudo systemctl restart annas-mcp"

# Test via Tailscale Funnel
curl https://raspberrypi.tailddbc27.ts.net/health
```

## Release Workflow
When publishing a new version:
```bash
git tag v1.x.x
git push --tags
# GitHub Actions will auto-build binaries via goreleaser
```

## Install Script
`install.sh` is a one-liner installer. Update it when:
- `.env` format changes
- Setup steps change
- Tailscale setup process changes
- Repository URL changes

## TODO: Test and Release
**Status**: Repository moved to `sam-hartman/kindle-pibrarian`. All import paths updated.

**What's left**:
1. Make repo **public** (required for one-liner to work without auth)
2. Test the one-liner:
   ```bash
   curl -fsSL https://raw.githubusercontent.com/sam-hartman/kindle-pibrarian/main/install.sh | bash
   ```
3. Tag a new release (`git tag v1.0.2 && git push --tags`)

**Why public?** The one-liner uses GitHub's public API to fetch releases. Private repos require authentication tokens.

## Important Reminders
1. **Module path** - Current: `github.com/sam-hartman/kindle-pibrarian`

2. **Tailscale Funnel URL** - Current: `https://raspberrypi.tailddbc27.ts.net`

3. **Pi credentials** - Check `.env` file for PI_HOST, PI_USER, PI_PASS

4. **No Cloudflare** - We removed Cloudflare tunnel in favor of Tailscale Funnel. Don't add it back.

5. **Keep it simple** - Avoid over-engineering. This is a personal tool.

## Code Patterns
- Email fallback logic is in `checkEmailFallback()` helper
- Download retry logic is in `downloadFileData()` helper
- Tool definitions use constants at top of mcpserver.go
- Memory cleanup for downloadTracker runs in background goroutine

## Testing
```bash
# Search
curl -X POST https://raspberrypi.tailddbc27.ts.net/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"search","arguments":{"term":"test"}}}'

# Health
curl https://raspberrypi.tailddbc27.ts.net/health
```
