#!/bin/bash
# SmartClaw install script
set -e

REPO="instructkr/smartclaw"
BINARY="smartclaw"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
if [ "$ARCH" = "x86_64" ]; then ARCH="amd64"; fi
if [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then ARCH="arm64"; fi

# Get latest release
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$LATEST" ]; then
  echo "Error: Could not determine latest version"
  exit 1
fi

# Download
echo "Installing SmartClaw ${LATEST} for ${OS}/${ARCH}..."
URL="https://github.com/${REPO}/releases/download/${LATEST}/${BINARY}_${OS}_${ARCH}.tar.gz"
TMPDIR=$(mktemp -d)
curl -fsSL "${URL}" | tar xz -C "${TMPDIR}"

# Install
INSTALL_DIR="${HOME}/.local/bin"
mkdir -p "${INSTALL_DIR}"
mv "${TMPDIR}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
chmod +x "${INSTALL_DIR}/${BINARY}"
rm -rf "${TMPDIR}"

# Add to PATH if needed
if ! echo "$PATH" | grep -q "${INSTALL_DIR}"; then
  SHELL_RC="${HOME}/.bashrc"
  if [ -n "$ZSH_VERSION" ]; then SHELL_RC="${HOME}/.zshrc"; fi
  echo "" >> "${SHELL_RC}"
  echo 'export PATH="$HOME/.local/bin:$PATH"' >> "${SHELL_RC}"
  echo "Added ${INSTALL_DIR} to PATH in ${SHELL_RC}"
  echo "Run: source ${SHELL_RC}"
fi

echo "SmartClaw installed to ${INSTALL_DIR}/${BINARY}"
"${INSTALL_DIR}/${BINARY}" version
