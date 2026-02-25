# Project Organization Standards

## File Organization

This project follows a consistent file organization structure:

### Test Scripts
**Location:** `tests/` directory

All test scripts, diagnostic scripts, and test-related utilities should be placed in the `tests/` directory.

**Current test scripts:**
- Test scripts: `test-repo-comprehensive.sh`, `test-end-to-end.sh`

### Shell Scripts
**Location:** `scripts/` directory

All shell scripts (.sh files) should be placed in the `scripts/` directory.

**Core scripts:**
- Deployment: `deploy-on-pi.sh`, `deploy-with-tunnel.sh`
- Setup: `raspberry-pi-setup.sh`, `setup-email-env.sh`
- Server: `start-server.sh` (consolidated - auto-detects email config)

**Note:** One-off utility scripts and scripts containing sensitive tokens have been removed to keep the repository clean.

### Documentation
**Location:** `docs/` directory

All project documentation and markdown files should be placed in the `docs/` directory.

**Exception:** `README.md` stays in the project root (standard practice).

**Current documentation:**
- Setup guides: `KINDLE_EMAIL_SETUP.md`, `LE_CHAT_SETUP.md`
- Troubleshooting: `PI_TROUBLESHOOTING.md`
- Organization: `PROJECT_ORGANIZATION.md` (this file)

## Standard Practice

**IMPORTANT:** When creating new files:
1. ✅ Place test scripts in `tests/` directory
2. ✅ Place all shell scripts (.sh) in `scripts/` directory
3. ✅ Place all documentation (.md, .txt) in `docs/` directory (except README.md)
4. ✅ Keep project root clean and organized - only essential files
5. ✅ Update this document if adding new categories

This organization makes the project easier to navigate and maintain.

