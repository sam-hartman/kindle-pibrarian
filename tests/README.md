# Test Scripts

This directory contains test scripts and utilities for the Anna's Archive MCP server.

## Scripts

- `test-hash-response.sh` - Tests search response for hash inclusion and structuredContent field
- `test-connector.sh` - Tests MCP connector functionality
- `diagnose-pi.sh` - Diagnoses Raspberry Pi connectivity issues
- `deploy-update.sh` - Quick deployment script for updates
- `deploy-with-correct-user.sh` - Deployment script using correct username

## Usage

Run tests from the project root:
```bash
./tests/test-hash-response.sh
./tests/diagnose-pi.sh
```

## Note

All test scripts and related documentation should be organized:
- **Test scripts** → `tests/` directory
- **Documentation** → `docs/` directory

This keeps the project root clean and organized.

