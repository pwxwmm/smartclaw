#!/bin/bash
# Build SmartClaw macOS .dmg installer
# Usage: ./scripts/build-dmg.sh [version]
# Requires: macOS, hdiutil (built-in)

set -e

VERSION="${1:-dev}"
BINARY="smartclaw"
DMG_NAME="SmartClaw-${VERSION}"
WORKDIR=$(mktemp -d)

echo "Building ${DMG_NAME}.dmg..."

# Build the binary
echo "  Compiling for darwin/arm64 + darwin/amd64 (universal)..."
GOOS=darwin GOARCH=arm64 go build -o "${WORKDIR}/${BINARY}_arm64" ./cmd/smartclaw
GOOS=darwin GOARCH=amd64 go build -o "${WORKDIR}/${BINARY}_amd64" ./cmd/smartclaw

# Create universal binary
lipo -create -output "${WORKDIR}/${BINARY}" "${WORKDIR}/${BINARY}_arm64" "${WORKDIR}/${BINARY}_amd64"
rm "${WORKDIR}/${BINARY}_arm64" "${WORKDIR}/${BINARY}_amd64"

# Create DMG structure
DMG_ROOT="${WORKDIR}/dmg"
mkdir -p "${DMG_ROOT}"

# Copy binary into a .app bundle
APP_DIR="${DMG_ROOT}/SmartClaw.app"
mkdir -p "${APP_DIR}/Contents/MacOS"
mkdir -p "${APP_DIR}/Contents/Resources"

cp "${WORKDIR}/${BINARY}" "${APP_DIR}/Contents/MacOS/smartclaw"

# Create Info.plist
cat > "${APP_DIR}/Contents/Info.plist" << 'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>smartclaw</string>
    <key>CFBundleIdentifier</key>
    <string>com.smartclaw.app</string>
    <key>CFBundleName</key>
    <string>SmartClaw</string>
    <key>CFBundleDisplayName</key>
    <string>SmartClaw</string>
    <key>CFBundleVersion</key>
PLIST
echo "    <string>${VERSION}</string>" >> "${APP_DIR}/Contents/Info.plist"
cat >> "${APP_DIR}/Contents/Info.plist" << 'PLIST'
    <key>CFBundleShortVersionString</key>
PLIST
echo "    <string>${VERSION}</string>" >> "${APP_DIR}/Contents/Info.plist"
cat >> "${APP_DIR}/Contents/Info.plist" << 'PLIST'
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleIconFile</key>
    <string>AppIcon</string>
    <key>LSMinimumSystemVersion</key>
    <string>11.0</string>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>NSSupportsAutomaticTermination</key>
    <true/>
    <key>NSSupportsSuddenTermination</key>
    <true/>
</dict>
</plist>
PLIST

# Create a launcher script that opens Terminal
cat > "${APP_DIR}/Contents/MacOS/SmartClaw" << 'LAUNCHER'
#!/bin/bash
DIR="$(cd "$(dirname "$0")" && pwd)"
SMARTCLAW="${DIR}/smartclaw"
PORT_FILE="/tmp/smartclaw-desktop-port-$$.txt"

$SMARTCLAW web --port 0 > "$PORT_FILE" 2>&1 &
SERVER_PID=$!

sleep 1

if [ -f "$PORT_FILE" ]; then
    PORT=$(grep -oE 'localhost:[0-9]+' "$PORT_FILE" | grep -oE '[0-9]+' | head -1)
    if [ -n "$PORT" ]; then
        open "http://localhost:${PORT}"
    fi
fi

wait $SERVER_PID 2>/dev/null
rm -f "$PORT_FILE"
LAUNCHER
chmod +x "${APP_DIR}/Contents/MacOS/SmartClaw"

# Create Applications symlink for drag-to-Applications
ln -s /Applications "${DMG_ROOT}/Applications"

# Create a simple README
cat > "${DMG_ROOT}/README.txt" << README
SmartClaw ${VERSION}
==================

Installation:
  1. Drag SmartClaw.app to the Applications folder
  2. Open Terminal and run: smartclaw

Or use the CLI directly:
  /Applications/SmartClaw.app/Contents/MacOS/smartclaw

First-time setup may require:
  System Preferences → Security → Allow SmartClaw
README

# Create the DMG
hdiutil create -volname "SmartClaw" \
  -srcfolder "${DMG_ROOT}" \
  -ov -format UDZO \
  "${DMG_NAME}.dmg"

# Cleanup
rm -rf "${WORKDIR}"

echo "Done: ${DMG_NAME}.dmg"
ls -lh "${DMG_NAME}.dmg"
