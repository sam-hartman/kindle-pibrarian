# Claude Instructions for annas-mcp-server

## Project Overview
MCP server for searching and downloading books from Anna's Archive, with Kindle email support. Deployed on Raspberry Pi with Tailscale Funnel for remote access.

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

## TODO: Finish One-Liner Installer Setup
**Status**: GitHub Actions workflow is set up and working. Releases (v1.0.0, v1.0.1) have been built.

**What's left**:
1. Move repo to personal GitHub account (currently on `sam-hartman-mistral`)
2. Make repo **public** (required for one-liner to work without auth)
3. Update these files with new repo URL:
   - `go.mod` - module path
   - `install.sh` - REPO variable at top
   - `README.md` - all GitHub URLs
   - All import paths in Go files (use grep for old org name)
4. Test the one-liner:
   ```bash
   curl -fsSL https://raw.githubusercontent.com/NEW_USER/annas-mcp-server/main/install.sh | bash
   ```

**Why public?** The one-liner uses GitHub's public API to fetch releases. Private repos require authentication tokens.

**Testing the installer** (after repo is public):
```bash
# On the Pi - full test with Tailscale
./tests/test-installer.sh

# Quick test - skip Tailscale setup
./tests/test-installer.sh --quick
```

The test script:
1. Cleans up existing installation
2. Runs installer with `ANNAS_SECRET_KEY` from .env (non-interactive)
3. Verifies binary, .env, service, and HTTP endpoint

**Non-interactive install** (for automation):
```bash
ANNAS_SECRET_KEY=your-key SKIP_TAILSCALE=1 bash install.sh
```

## Important Reminders
1. **Module path may change** - Repo will move to different GitHub org. When that happens, update `go.mod` and all import paths.

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
