#!/bin/sh
# bootstrap.sh — Installs or updates squadai from GitHub.
#
# Save this script once to a directory on your PATH and make it executable:
#
#   mkdir -p ~/.local/bin
#   curl -fsSL https://raw.githubusercontent.com/PedroMosquera/squadai/main/scripts/bootstrap.sh \
#     -o ~/.local/bin/squadai-update
#   chmod +x ~/.local/bin/squadai-update
#
# Then run it any time to install or update squadai:
#
#   squadai-update
#
# Override the install directory with: SQUADAI_INSTALL_DIR=/some/path squadai-update

set -e

INSTALL_SCRIPT_URL="https://raw.githubusercontent.com/PedroMosquera/squadai/main/scripts/install.sh"

# --- Sanity checks ---
if ! command -v curl >/dev/null 2>&1; then
    echo "Error: curl is required but not installed." >&2
    exit 1
fi

echo "→ Fetching squadai installer from GitHub..."
TMP=$(mktemp)
trap 'rm -f "$TMP"' EXIT INT TERM

if ! curl -fsSL "$INSTALL_SCRIPT_URL" -o "$TMP"; then
    echo "" >&2
    echo "Error: failed to download install script." >&2
    echo "  Check your internet connection or visit:" >&2
    echo "  https://github.com/PedroMosquera/squadai" >&2
    exit 1
fi

chmod +x "$TMP"

if [ -n "$SQUADAI_INSTALL_DIR" ]; then
    export INSTALL_DIR="$SQUADAI_INSTALL_DIR"
fi

echo "→ Running installer..."
sh "$TMP"

echo ""
echo "✓ squadai-update finished. Re-run this command anytime to upgrade."
