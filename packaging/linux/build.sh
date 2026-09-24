#!/usr/bin/env bash
# ==============================================================================
# Build script for Linux Agent Packages (.deb and GUI bundle)
# ==============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

VERSION="1.0.0"
ARCH="${1:-amd64}"

echo "=== Building Endpoint Management Agent Linux Package ==="
echo "Version: $VERSION, Arch: $ARCH"

# 1. Compile agent binary
cd "$REPO_ROOT"
OUT_BIN="$SCRIPT_DIR/deb/usr/bin/endpoint-agent"
mkdir -p "$(dirname "$OUT_BIN")"

echo "[1/3] Compiling agent binary for linux/$ARCH..."
CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" \
  go build -ldflags="-s -w" -o "$OUT_BIN" ./agent/cmd/agent

# Also copy to root for general convenience
cp "$OUT_BIN" "$REPO_ROOT/agent-linux-$ARCH"

# 2. Update architecture in control file
sed -i -e "s/^Architecture: .*/Architecture: $ARCH/" "$SCRIPT_DIR/deb/DEBIAN/control"
chmod 0755 "$SCRIPT_DIR/deb/DEBIAN/postinst" "$SCRIPT_DIR/deb/DEBIAN/prerm" "$SCRIPT_DIR/deb/DEBIAN/postrm"

# 3. Build .deb package if dpkg-deb is available
if command -v dpkg-deb >/dev/null 2>&1; then
    echo "[2/3] Building .deb package..."
    DEB_NAME="$SCRIPT_DIR/endpoint-agent_${VERSION}_${ARCH}.deb"
    dpkg-deb --build --root-owner-group "$SCRIPT_DIR/deb" "$DEB_NAME"
    echo "[+] Debian package created: $DEB_NAME"
else
    echo "[!] dpkg-deb not found on this host. Skipping .deb packaging."
    echo "    On Ubuntu/Debian, install dpkg-dev to build .deb packages."
fi

# 4. Make GUI installer executable
chmod +x "$SCRIPT_DIR/gui-installer/install-gui.sh" "$SCRIPT_DIR/gui-installer/uninstall-gui.sh"
echo "[3/3] GUI installer scripts ready in packaging/linux/gui-installer/"
