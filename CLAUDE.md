# Claude Instructions for kindle-pibrarian

## Project Overview
MCP server for searching and downloading books from Anna's Archive, with Kindle email support. Deployed on Raspberry Pi with Tailscale Funnel for remote access.

**Repository:** https://github.com/sam-hartman/kindle-pibrarian

## Key Files
- `internal/anna/anna.go` - Core search/download/email logic
- `internal/modes/mcpserver.go` - MCP server implementation
- `install.sh` - One-liner installer for Raspberry Pi

## Development
```bash
# Build
go build -o annas-mcp ./cmd/annas-mcp

# Test locally
./annas-mcp http --port 8080
curl http://localhost:8080/health
```

## Release
```bash
git tag v1.x.x
git push --tags
# GitHub Actions builds binaries via goreleaser
```

## Install Script
`install.sh` is a one-liner installer. Update it when:
- `.env` format changes
- Setup steps change
- Tailscale setup process changes

## Guidelines
1. **Keep it simple** - This is a personal tool, avoid over-engineering
2. **No Cloudflare** - We use Tailscale Funnel, not Cloudflare tunnel
3. **Module path** - `github.com/sam-hartman/kindle-pibrarian`

## Code Patterns
- Email logic consolidated in `SendFileToKindle()` helper
- MIME types via `getMimeType()` helper
- Tool definitions use constants at top of mcpserver.go
