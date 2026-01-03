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

### Deployment Scripts
**Location:** Project root (for main deployment scripts)

Main deployment scripts that are part of the core workflow stay in the root:
- `deploy-on-pi.sh`
- `deploy-with-tunnel.sh`
- `deploy-to-pi.sh`

## Standard Practice

**IMPORTANT:** When creating new test scripts or documentation:
1. ✅ Place test scripts in `tests/` directory
2. ✅ Place documentation in `docs/` directory
3. ✅ Keep project root clean and organized
4. ✅ Update this document if adding new categories

This organization makes the project easier to navigate and maintain.

