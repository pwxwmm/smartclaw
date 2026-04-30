#!/bin/bash
# SmartClaw Desktop packaging script
# Usage: ./scripts/package-desktop.sh [macos|windows|linux|all]
# Make executable: chmod +x scripts/package-desktop.sh
set -euo pipefail

VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo "dev")"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")"
DATE="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
APP_NAME="SmartClaw"
BINARY="smartclaw"
DIST_DIR="dist/desktop"
LDFLAGS="-s -w -X github.com/instructkr/smartclaw/internal/cli.Version=${VERSION} -X github.com/instructkr/smartclaw/internal/cli.Commit=${COMMIT} -X github.com/instructkr/smartclaw/internal/cli.Date=${DATE}"
DESKTOP_LDFLAGS="${LDFLAGS} -X github.com/instructkr/smartclaw/desktop.BuildVersion=${VERSION} -X github.com/instructkr/smartclaw/desktop.BuildCommit=${COMMIT}"

PLATFORM="${1:-}"
if [ -z "${PLATFORM}" ]; then
    echo "Usage: $0 [macos|windows|linux|all]"
    exit 1
fi

rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

package_macos() {
    echo "==> Packaging for macOS..."
    local WORKDIR
    WORKDIR="$(mktemp -d)"
    trap 'rm -rf "${WORKDIR}"' RETURN

    # Build arm64 (Apple Silicon)
    echo "    Building darwin/arm64..."
    CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build \
        -tags desktop -ldflags "${DESKTOP_LDFLAGS}" \
        -o "${WORKDIR}/${BINARY}_arm64" ./cmd/smartclaw-desktop/

    # Build amd64 (Intel)
    echo "    Building darwin/amd64..."
    CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build \
        -tags desktop -ldflags "${DESKTOP_LDFLAGS}" \
        -o "${WORKDIR}/${BINARY}_amd64" ./cmd/smartclaw-desktop/

    # Create universal binary
    echo "    Creating universal binary..."
    lipo -create -output "${WORKDIR}/${BINARY}" \
        "${WORKDIR}/${BINARY}_arm64" "${WORKDIR}/${BINARY}_amd64" 2>/dev/null || {
        echo "    Warning: lipo failed, using arm64 binary"
        cp "${WORKDIR}/${BINARY}_arm64" "${WORKDIR}/${BINARY}"
    }

    # Create .app bundle
    local APP_DIR="${DIST_DIR}/${APP_NAME}.app"
    mkdir -p "${APP_DIR}/Contents/MacOS"
    mkdir -p "${APP_DIR}/Contents/Resources"
    cp "${WORKDIR}/${BINARY}" "${APP_DIR}/Contents/MacOS/${BINARY}"
    chmod +x "${APP_DIR}/Contents/MacOS/${BINARY}"

    # Copy frontend assets if available
    if [ -d "desktop/frontend/dist" ]; then
        cp -r desktop/frontend/dist "${APP_DIR}/Contents/Resources/frontend"
    fi

    # Copy icon if available
    if [ -f "assets/icon.icns" ]; then
        cp assets/icon.icns "${APP_DIR}/Contents/Resources/AppIcon.icns"
    fi

    # Generate Info.plist
    cat > "${APP_DIR}/Contents/Info.plist" << PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>${BINARY}</string>
    <key>CFBundleIdentifier</key>
    <string>com.smartclaw.desktop</string>
    <key>CFBundleName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleDisplayName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleVersion</key>
    <string>${VERSION}</string>
    <key>CFBundleShortVersionString</key>
    <string>${VERSION}</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleIconFile</key>
    <string>AppIcon</string>
    <key>LSMinimumSystemVersion</key>
    <string>11.0</string>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>LSUIElement</key>
    <false/>
    <key>NSHumanReadableCopyright</key>
    <string>Copyright © $(date +%Y) SmartClaw. MIT License.</string>
</dict>
</plist>
PLIST

    # Codesign if certificate available
    if [ -n "${CODESIGN_IDENTITY:-}" ]; then
        echo "    Codesigning with ${CODESIGN_IDENTITY}..."
        codesign --force --deep --sign "${CODESIGN_IDENTITY}" "${APP_DIR}"
    else
        echo "    Skipping codesign (set CODESIGN_IDENTITY to enable)"
    fi

    # Create DMG if create-dmg is available
    if command -v create-dmg &>/dev/null; then
        echo "    Creating DMG..."
        create-dmg \
            --volname "${APP_NAME}" \
            --app-drop-link 600 185 \
            "${DIST_DIR}/${APP_NAME}-${VERSION}.dmg" \
            "${APP_DIR}" 2>/dev/null || echo "    Warning: DMG creation failed"
    else
        echo "    Skipping DMG (install create-dmg: brew install create-dmg)"
    fi

    echo "    Done: ${APP_DIR}"
}

package_windows() {
    echo "==> Packaging for Windows..."
    local WORKDIR
    WORKDIR="$(mktemp -d)"
    trap 'rm -rf "${WORKDIR}"' RETURN

    echo "    Building windows/amd64..."
    CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build \
        -tags desktop -ldflags "${DESKTOP_LDFLAGS}" \
        -o "${WORKDIR}/${BINARY}.exe" ./cmd/smartclaw-desktop/

    # Create installer structure
    local INST_DIR="${DIST_DIR}/windows"
    mkdir -p "${INST_DIR}"

    cp "${WORKDIR}/${BINARY}.exe" "${INST_DIR}/"

    # Copy frontend assets
    if [ -d "desktop/frontend/dist" ]; then
        cp -r desktop/frontend/dist "${INST_DIR}/frontend"
    fi

    # Copy icon
    if [ -f "assets/icon.ico" ]; then
        cp assets/icon.ico "${INST_DIR}/"
    fi

    # Copy NSIS installer script
    if [ -f "scripts/windows/installer.nsi" ]; then
        cp scripts/windows/installer.nsi "${INST_DIR}/"
        sed -i.bak "s/PRODUCT_VERSION \"1.0.0\"/PRODUCT_VERSION \"${VERSION}\"/" "${INST_DIR}/installer.nsi"
        rm -f "${INST_DIR}/installer.nsi.bak"
    fi

    # Build NSIS installer if makensis available
    if command -v makensis &>/dev/null && [ -f "${INST_DIR}/installer.nsi" ]; then
        echo "    Building NSIS installer..."
        (cd "${INST_DIR}" && makensis installer.nsi)
        mv "${INST_DIR}/SmartClaw-${VERSION}-setup.exe" "${DIST_DIR}/" 2>/dev/null || true
    else
        echo "    Skipping NSIS installer (install makensis: brew install nsis)"
    fi

    # Create ZIP as fallback
    if command -v zip &>/dev/null; then
        (cd "${DIST_DIR}" && zip -r "${APP_NAME}-${VERSION}-windows-amd64.zip" windows/)
        echo "    Created: ${APP_NAME}-${VERSION}-windows-amd64.zip"
    fi

    echo "    Done: ${INST_DIR}"
}

package_linux() {
    echo "==> Packaging for Linux..."
    local WORKDIR
    WORKDIR="$(mktemp -d)"
    trap 'rm -rf "${WORKDIR}"' RETURN

    echo "    Building linux/amd64..."
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
        -tags desktop -ldflags "${DESKTOP_LDFLAGS}" \
        -o "${WORKDIR}/${BINARY}" ./cmd/smartclaw-desktop/

    # Create AppDir structure for AppImage
    local APPDIR="${DIST_DIR}/${APP_NAME}.AppDir"
    mkdir -p "${APPDIR}/usr/bin"
    mkdir -p "${APPDIR}/usr/share/applications"
    mkdir -p "${APPDIR}/usr/share/icons/hicolor/256x256/apps"
    mkdir -p "${APPDIR}/usr/share/${BINARY}"

    cp "${WORKDIR}/${BINARY}" "${APPDIR}/usr/bin/"
    chmod +x "${APPDIR}/usr/bin/${BINARY}"

    # Copy frontend assets
    if [ -d "desktop/frontend/dist" ]; then
        cp -r desktop/frontend/dist "${APPDIR}/usr/share/${BINARY}/frontend"
    fi

    # Copy icon
    if [ -f "assets/icon.png" ]; then
        cp assets/icon.png "${APPDIR}/usr/share/icons/hicolor/256x256/apps/${BINARY}.png"
        cp assets/icon.png "${APPDIR}/${BINARY}.png"
    fi

    # Generate .desktop file
    cat > "${APPDIR}/usr/share/applications/${BINARY}.desktop" << DESKTOP
[Desktop Entry]
Name=${APP_NAME}
Exec=/usr/bin/${BINARY}
Icon=${BINARY}
Type=Application
Categories=Development;Utility;
Comment=Self-improving AI agent
Terminal=false
StartupWMClass=${APP_NAME}
DESKTOP

    # AppDir root files
    cp "${APPDIR}/usr/share/applications/${BINARY}.desktop" "${APPDIR}/"
    cat > "${APPDIR}/AppRun" << 'APPRUN'
#!/bin/bash
SELF="$(readlink -f "$0" 2>/dev/null || realpath "$0")"
HERE="$(dirname "${SELF}")"
exec "${HERE}/usr/bin/smartclaw" "$@"
APPRUN
    chmod +x "${APPDIR}/AppRun"

    # Build AppImage if appimagetool available
    if command -v appimagetool &>/dev/null; then
        echo "    Building AppImage..."
        ARCH=x86_64 appimagetool "${APPDIR}" "${DIST_DIR}/${APP_NAME}-${VERSION}-x86_64.AppImage" 2>/dev/null || \
            echo "    Warning: AppImage creation failed"
    else
        echo "    Skipping AppImage (install appimagetool from https://appimage.github.io/)"
    fi

    # Create tar.gz as fallback
    (cd "${DIST_DIR}" && tar czf "${APP_NAME}-${VERSION}-linux-amd64.tar.gz" "${APP_NAME}.AppDir")
    echo "    Done: ${APPDIR}"
}

echo "SmartClaw Desktop Packaging"
echo "Version: ${VERSION} (${COMMIT})"
echo ""

case "${PLATFORM}" in
    macos)  package_macos ;;
    windows) package_windows ;;
    linux)  package_linux ;;
    all)
        package_macos
        package_windows
        package_linux
        ;;
    *)
        echo "Unknown platform: ${PLATFORM}"
        echo "Usage: $0 [macos|windows|linux|all]"
        exit 1
        ;;
esac

echo ""
echo "All packages written to ${DIST_DIR}/"
ls -lh "${DIST_DIR}/"
