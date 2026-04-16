#!/bin/bash
# Build SmartClaw Windows .exe installer
# Usage: ./scripts/build-exe.sh [version]
# Requires: makensis (NSIS), Go cross-compilation

set -e

VERSION="${1:-dev}"
BINARY="smartclaw"
WORKDIR=$(mktemp -d)

echo "Building SmartClaw-${VERSION}-setup.exe..."

GOOS=windows GOARCH=amd64 go build -o "${WORKDIR}/smartclaw.exe" ./cmd/smartclaw

mkdir -p "${WORKDIR}/nsis"
cp "${WORKDIR}/smartclaw.exe" "${WORKDIR}/nsis/"
cp "$(dirname "$0")/windows/installer.nsi" "${WORKDIR}/nsis/"

sed -i.bak "s/PRODUCT_VERSION \"1.0.0\"/PRODUCT_VERSION \"${VERSION}\"/" "${WORKDIR}/nsis/installer.nsi"
rm -f "${WORKDIR}/nsis/installer.nsi.bak"

if command -v makensis &>/dev/null; then
    makensis "${WORKDIR}/nsis/installer.nsi"
    mv "${WORKDIR}/nsis/SmartClaw-${VERSION}-setup.exe" .
    rm -rf "${WORKDIR}"
    echo "Done: SmartClaw-${VERSION}-setup.exe"
    ls -lh "SmartClaw-${VERSION}-setup.exe"
else
    echo "Warning: makensis not found. Install NSIS to build .exe installer."
    echo "  macOS: brew install nsis"
    echo "  Ubuntu: sudo apt install nsis"
    echo ""
    echo "Binary built at: ${WORKDIR}/nsis/smartclaw.exe"
    echo "Run makensis manually: makensis ${WORKDIR}/nsis/installer.nsi"
fi
