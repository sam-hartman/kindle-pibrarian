# Raspberry Pi SSH Connectivity Troubleshooting

## Current Status
✅ **Cloudflare tunnel is working** - Pi is online and services are running  
❌ **SSH is not accessible** - Cannot connect from local network

## Diagnosis
The Pi can make **outbound** connections (Cloudflare tunnel works) but cannot receive **inbound** connections (SSH blocked).

## Most Likely Causes

### 1. Network Isolation / Different Networks
**Symptoms:**
- Pi and Mac are on different WiFi networks or VLANs
- Pi can reach internet but not local network

**Solution:**
- Ensure both devices are on the **same WiFi network**
- Check router settings for VLAN isolation
- Try connecting Mac to same network as Pi

### 2. Router Firewall Blocking Local Access
**Symptoms:**
- Router has firewall rules blocking local device-to-device communication
- Outbound internet works, but local connections fail

**Solution:**
- Check router admin panel for firewall settings
- Look for "AP Isolation" or "Client Isolation" settings (disable them)
- Check for any firewall rules blocking port 22

### 3. SSH Service Disabled on Pi
**Symptoms:**
- SSH daemon not running on Pi

**Solution (if you have physical/keyboard access to Pi):**
```bash
# Check SSH status
sudo systemctl status ssh

# Enable and start SSH
sudo systemctl enable ssh
sudo systemctl start ssh

# Verify it's running
sudo systemctl status ssh
```

### 4. Pi Firewall Blocking SSH
**Symptoms:**
- UFW or iptables blocking port 22

**Solution (if you have physical/keyboard access to Pi):**
```bash
# Check UFW status
sudo ufw status

# Allow SSH if UFW is active
sudo ufw allow 22/tcp

# Or check iptables
sudo iptables -L -n | grep 22
```

### 5. IP Address Changed
**Symptoms:**
- Pi got a new IP from DHCP

**Solution:**
- Check router DHCP lease table
- Or scan network: `nmap -sn 192.168.1.0/24`
- Look for device with hostname "raspberrypi" or MAC address of Pi

## Quick Fixes

### Option A: Fix Network Connection
1. Connect Mac to same WiFi as Pi
2. Check router settings for AP isolation
3. Verify Pi's IP address hasn't changed

### Option B: Enable SSH via Physical Access
If you have HDMI/keyboard access to Pi:
```bash
sudo systemctl enable ssh
sudo systemctl start ssh
sudo ufw allow 22/tcp
```

### Option C: Deploy Manually
Since tunnel works, deploy binary manually:

1. **Copy binary to USB:**
   ```bash
   cp annas-mcp-linux-arm /Volumes/USBDRIVE/
   ```

2. **On Pi (via physical access or once SSH works):**
   ```bash
   sudo cp /media/usb/annas-mcp-linux-arm /home/pi/annas-mcp-server/annas-mcp
   sudo chmod +x /home/pi/annas-mcp-server/annas-mcp
   sudo systemctl restart annas-mcp
   ```

### Option D: Use deploy-on-pi.sh Script
If you can access Pi another way (VNC, physical access, etc.):
```bash
# Copy deploy-on-pi.sh to Pi
# Then run it directly on Pi:
cd /home/pi/annas-mcp-server
bash deploy-on-pi.sh
```

## Verify Fix
After fixing network/SSH:
```bash
ssh pi@192.168.1.201
# Password: test
```

## Current Deployment Status
- ✅ Code updated with `structuredContent` support
- ✅ Binary built: `annas-mcp-linux-arm` (22MB)
- ✅ Ready to deploy once SSH is accessible
- ✅ Tunnel working, so Pi is functional

## Next Steps
1. Fix network connectivity (most likely issue)
2. Or deploy manually via USB
3. Then test with: `./test-hash-response.sh`

