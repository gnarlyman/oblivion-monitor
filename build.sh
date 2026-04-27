#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

# Generate Win32 resource file from app.manifest. Required by walk for
# Common Controls 6 + High-DPI awareness.
rsrc -manifest app.manifest -o rsrc.syso

# Build GUI subsystem (no console window).
GOOS=windows GOARCH=amd64 go build -ldflags="-H=windowsgui -s -w" -o oblivion-monitor.exe .

ls -la oblivion-monitor.exe
echo "Build OK."
