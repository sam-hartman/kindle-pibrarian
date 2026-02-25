# Raspberry Pi Troubleshooting

## Common Issues

### SSH Connection Issues

**Symptoms:**
- Cannot connect via SSH
- Connection refused or timeout

**Solutions:**

1. **Check if SSH is enabled:**
   ```bash
   sudo systemctl status ssh
   sudo systemctl enable ssh
   sudo systemctl start ssh
   ```

2. **Check firewall:**
   ```bash
   sudo ufw status
   sudo ufw allow 22/tcp
   ```

3. **Verify IP address:**
   ```bash
   # On Pi
   hostname -I

   # Or scan from another machine
   nmap -sn 192.168.1.0/24
   ```

4. **Check router settings:**
   - Disable AP isolation / client isolation
   - Ensure both devices are on the same network

### MCP Server Issues

**Server won't start:**
```bash
# Check service status
sudo systemctl status annas-mcp

# View logs
sudo journalctl -u annas-mcp -f

# Check if port is in use
sudo lsof -i :8081
```

**Server starts but doesn't respond:**
```bash
# Test locally
curl http://localhost:8081/health

# Check if .env file exists and has correct values
cat ~/annas-mcp-server/.env
```

### Tailscale Funnel Issues

**Funnel not working:**
```bash
# Check Tailscale status
tailscale status

# Check Funnel status
sudo tailscale funnel status

# Restart Funnel
sudo tailscale funnel --https=443 off
sudo tailscale funnel --bg 8081
```

**Funnel not enabled on tailnet:**
- Visit the URL shown when running `sudo tailscale funnel 8081`
- Enable Funnel in your Tailscale admin console

**Test Funnel externally:**
```bash
curl https://your-hostname.ts.net/health
```

### Build Issues

**Go not found:**
```bash
export PATH=$PATH:/usr/local/go/bin
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
```

**Build fails:**
```bash
# Check Go version
go version

# Clean and rebuild
cd ~/annas-mcp-server
go clean
go build -o annas-mcp ./cmd/annas-mcp
```

## Useful Commands

```bash
# Server management
sudo systemctl start annas-mcp
sudo systemctl stop annas-mcp
sudo systemctl restart annas-mcp
sudo systemctl status annas-mcp

# View logs
sudo journalctl -u annas-mcp -f
sudo journalctl -u annas-mcp --since "1 hour ago"

# Tailscale
tailscale status
sudo tailscale funnel status
sudo tailscale funnel --bg 8081

# Test endpoints
curl http://localhost:8081/health
curl -X POST http://localhost:8081/mcp -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

## Deployment Checklist

1. SSH into Pi
2. Run deployment script: `bash ~/annas-mcp-server/scripts/deploy-on-pi.sh`
3. Verify server is running: `sudo systemctl status annas-mcp`
4. Verify Funnel is running: `sudo tailscale funnel status`
5. Test externally: `curl https://your-hostname.ts.net/health`
