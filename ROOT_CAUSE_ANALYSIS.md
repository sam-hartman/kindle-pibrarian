# Root Cause Analysis: Book Download Failures

## Date: January 9, 2026

## Problem Statement
Users were unable to download books from Anna's Archive MCP server. Downloads consistently failed with "broken pipe" errors.

## Root Cause

### Primary Issue: Gmail Attachment Size Limit
Gmail has a **25MB attachment limit** for emails. When base64-encoded (required for email attachments), files grow by ~33%. This means:
- Files larger than ~18-19MB fail to send via Gmail
- Gmail closes the SMTP connection mid-transfer when it detects oversized files
- Result: "broken pipe" error

### Secondary Issue: Missing Fallback Logic
The download tool only fell back to local download when email configuration was missing, but NOT when:
- File size exceeded Gmail's limit
- SMTP connection failed ("broken pipe")
- Gmail explicitly rejected the file (552 error)

## Evidence from Logs
```
Jan 09 22:23:28: ERROR: failed to send email: write tcp [...]:587: write: broken pipe
Jan 09 22:25:35: ERROR: 552 5.3.4 Your message exceeded Google's message size limits
Jan 09 22:27:50: ERROR: failed to send email: write tcp [...]:587: write: broken pipe
```

## Solution Implemented

### 1. Added File Size Validation (anna.go)
- Check file size BEFORE attempting email send
- Reject files > 18MB with clear error message
- Prevents wasted SMTP connection attempts

### 2. Enhanced Fallback Logic (mcpserver.go)
- Fall back to local download when:
  - Email configuration missing
  - File too large for email
  - SMTP "broken pipe" errors
  - Gmail size limit errors (552)
- Provides clear feedback about why fallback occurred

### 3. Improved Error Messages
- File size included in all error messages
- Clear distinction between different failure types
- Helpful suggestions for users

## Testing Results

### Before Fix
```
Error: write tcp [...]:587: write: broken pipe
Result: Download failed completely
```

### After Fix
```
Success: Book downloaded successfully to path: /home/samuelhartman/Downloads/Anna's Archive
Note: File too large for email (>18MB), so it was saved locally instead of emailing to Kindle
```

## Files Modified
1. `internal/anna/anna.go` - Added file size validation
2. `internal/modes/mcpserver.go` - Enhanced fallback logic (2 locations: MCP tool + HTTP endpoint)

## Deployment
- Built ARM binary for Raspberry Pi
- Deployed to Pi at: ~/annas-mcp-server/annas-mcp
- Restarted services: annas-mcp, cloudflared-tunnel
- Verified via Cloudflare tunnel: https://gzip-includes-corpus-mens.trycloudflare.com

## Recommendations
1. **For users**: Files > 18MB will download locally instead of emailing to Kindle
2. **Future enhancement**: Consider alternative delivery methods for large files (e.g., cloud storage links)
3. **Monitoring**: Track file sizes to understand typical download patterns

## Status
✅ **RESOLVED** - Downloads now work with automatic fallback to local storage for large files
