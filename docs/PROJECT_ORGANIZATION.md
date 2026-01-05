# Project Organization Standards

## File Organization

This project follows a consistent file organization structure:

### Test Scripts
**Location:** `tests/` directory

All test scripts, diagnostic scripts, and test-related utilities should be placed in the `tests/` directory.

Examples:
- Test scripts: `test-hash-response.sh`, `test-connector.sh`
- Diagnostic scripts: `diagnose-pi.sh`
- Deployment test scripts: `deploy-update.sh`

### Documentation
**Location:** `docs/` directory

All project documentation, test results, troubleshooting guides, and related markdown files should be placed in the `docs/` directory.

Examples:
- Test results: `TEST_RESULTS.md`
- Troubleshooting guides: `PI_TROUBLESHOOTING.md`
- Project documentation: `PROJECT_ORGANIZATION.md`

### Shell Scripts
**Location:** `scripts/` directory

All shell scripts (.sh files) should be placed in the `scripts/` directory.

Examples:
- Deployment scripts: `deploy-on-pi.sh`, `deploy-with-tunnel.sh`, `deploy-to-pi.sh`
- Setup scripts: `setup-email-env.sh`, `setup-new-repo.sh`, `raspberry-pi-setup.sh`
- Utility scripts: `load-env.sh`, `start-http-server.sh`, `manage-tag.sh`

### Documentation
**Location:** `docs/` directory

All project documentation, markdown files, and text files should be placed in the `docs/` directory.

**Exception:** `README.md` stays in the project root (standard practice).

Examples:
- Setup guides: `KINDLE_EMAIL_SETUP.md`, `LE_CHAT_SETUP.md`
- Documentation: `AGENT_SYSTEM_PROMPT.md`, `SECURITY_AUDIT.md`
- Troubleshooting: `PI_TROUBLESHOOTING.md`
- Test results: `TEST_RESULTS.md`

## Standard Practice

**IMPORTANT:** When creating new files:
1. ✅ Place test scripts in `tests/` directory
2. ✅ Place all shell scripts (.sh) in `scripts/` directory
3. ✅ Place all documentation (.md, .txt) in `docs/` directory (except README.md)
4. ✅ Keep project root clean and organized - only essential files
5. ✅ Update this document if adding new categories

This organization makes the project easier to navigate and maintain.

