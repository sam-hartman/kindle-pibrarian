# Pibrarian — Operations & Hardening

Practical runbook for keeping the Pibrarian reliable for non-technical users.
Architecture: **webapp (Vercel) → Fly MCP `kindle-pibrarian-api` → Pi relay
(Tailscale Funnel) → Anna's Archive + Gmail SMTP → Amazon Send-to-Kindle**.
The Pi does the real download + SMTP send, so most server logic runs there.

## Deploying changes

Both Fly and the Pi build from `github.com/sam-hartman/kindle-pibrarian`.

**Pi (the box that downloads + emails) — deterministic deploy:**
```bash
ssh samuelhartman@192.168.1.201 \
  'cd ~/annas-mcp-server && git fetch origin && git reset --hard origin/main && \
   /usr/local/go/bin/go build -o annas-mcp ./cmd/annas-mcp && \
   sudo systemctl restart annas-mcp && systemctl is-active annas-mcp'
```
Use `git reset --hard origin/main` (not `git pull`) so a deploy can't silently
no-op on a local-edit conflict.

**Fly (proxy + fly.toml like min_machines_running / health check):**
```bash
cd ~/kindle-pibrarian && flyctl deploy
```

**Webapp:** auto-deploys on push to `main` via Vercel.

## Configuration / secrets

| Var | Where | Notes |
|-----|-------|-------|
| `ANNAS_BASE_URLS` | Pi (+ Fly) | Optional comma-separated mirror list, highest priority first. Defaults to `annas-archive.gl, .se, .org`. **Change here when Anna's rotates domains** — no code change/redeploy needed. |
| `ANNAS_SECRET_KEY` | Pi | Anna's membership key. Expires on renewal lapse → downloads fail. |
| `SMTP_*`, `FROM_EMAIL` | Pi | Gmail app password. Google revokes these periodically. |
| `COOKIE_SECRET` | Vercel | **Must** be set in production (app now fails closed without it). |
| `FLY_PASSCODE` (Vercel) == `WEB_PASSCODE` (Fly) | both | Must match or every call 401s. |
| `UPSTREAM_RELAY_URL` / `UPSTREAM_RELAY_SECRET` | Fly | Points Fly at the Pi Funnel; the secret must match the Pi. |

## Monitoring (do this — the girlfriend should never be the monitor)

1. **External uptime monitor** (UptimeRobot/Better Uptime free tier) on:
   - the Pi's Tailscale Funnel `/health` (the true bottleneck), and
   - the Vercel app URL.
   Alert **Sam** by SMS/Slack on failure.
2. **Disable Tailscale key expiry** for the Pi node in the Tailscale admin console
   (otherwise the Funnel dies ~every 6 months and takes everything down).
3. Confirm the Funnel survives a Pi reboot (`tailscale funnel status` after a test
   reboot); `annas-mcp.service` already has `Restart=always`.
4. **Synthetic send check:** monthly cron on the Pi running `annas-mcp` test-email
   that pings Sam if SMTP/Anna's creds have broken.

## Known remaining gaps (not yet built)

- **Async bounce visibility.** Amazon accepts the SMTP email, then can reject it
  hours later (E999, etc.) with a bounce to the Gmail inbox the app never reads.
  The pre-send EPUB validation now catches the common causes (DRM, broken
  structure, the `data-Amzn` E999), but a full fix is a Gmail IMAP bounce-watcher
  + a persisted send-log that flips a send to "failed" and notifies. See the
  "bounce-watcher" task. Until then, the webapp says "on its way... usually within
  a few minutes" rather than claiming guaranteed delivery.
- **Mobile tap targets** on the small action links could be enlarged.
