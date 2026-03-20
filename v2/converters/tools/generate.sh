#!/usr/bin/env bash
#
# generate.sh — Reproducibly generates the compressed charset data blob
# used by go-ora's string converter.
#
# This script extracts the original Oracle charset conversion tables from
# git history and serializes them into a compact zstd-compressed binary
# that is embedded at compile time via //go:embed.
#
# Prerequisites: go, zstd CLI
# Usage: cd v2/converters/tools && ./generate.sh
#
# The original 78K-line string_conversion_new.go (11 MiB) contained hardcoded
# charset tables as Go source literals. This script serializes those same
# tables into a ~770 KB zstd-compressed binary blob — identical behavior,
# ~14x smaller.

set -euo pipefail

# --- Configuration ---
# Upstream commit containing the original string_conversion_new.go.
# This is the last upstream merge before our charset table replacement.
UPSTREAM_COMMIT="6955056"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CONVERTERS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_DIR="$(cd "$CONVERTERS_DIR/../.." && pwd)"

ORIGINAL_FILE="$CONVERTERS_DIR/string_conversion_new.go"
RAW_BLOB="$CONVERTERS_DIR/testdata/charsets.bin"
ZSTD_BLOB="$CONVERTERS_DIR/charsetdata/charsets.bin.zst"

# --- Preflight checks ---
command -v go >/dev/null 2>&1 || { echo "Error: 'go' is required but not found."; exit 1; }
command -v zstd >/dev/null 2>&1 || { echo "Error: 'zstd' is required but not found. Install with: brew install zstd"; exit 1; }

# Verify the upstream commit exists
git -C "$REPO_DIR" cat-file -e "$UPSTREAM_COMMIT" 2>/dev/null || {
    echo "Error: upstream commit $UPSTREAM_COMMIT not found in git history."
    echo "Make sure you're running this from the DataDog/go-ora fork."
    exit 1
}

# --- Step 1: Restore original charset tables from git history ---
echo "==> Restoring original string_conversion_new.go from commit $UPSTREAM_COMMIT"
cp "$ORIGINAL_FILE" "$ORIGINAL_FILE.bak"

cleanup() {
    echo "==> Restoring modified string_conversion_new.go"
    mv "$ORIGINAL_FILE.bak" "$ORIGINAL_FILE"
    rm -f "$RAW_BLOB"
}
trap cleanup EXIT

git -C "$REPO_DIR" show "$UPSTREAM_COMMIT:v2/converters/string_conversion_new.go" > "$ORIGINAL_FILE"

# --- Step 2: Generate raw binary charset data ---
echo "==> Generating raw binary charset data"
mkdir -p "$CONVERTERS_DIR/testdata"
mkdir -p "$CONVERTERS_DIR/charsetdata"
cd "$CONVERTERS_DIR"
go test -run TestGenerateRawCharsetData -v . 2>&1 | tail -3

# --- Step 3: Compress with zstd at maximum compression ---
echo "==> Compressing with zstd --ultra -22"
zstd --ultra -22 --force "$RAW_BLOB" -o "$ZSTD_BLOB" 2>&1

echo ""
echo "==> Done!"
echo "    Raw binary: $(wc -c < "$RAW_BLOB" | tr -d ' ') bytes"
echo "    Compressed: $(wc -c < "$ZSTD_BLOB" | tr -d ' ') bytes"
echo "    Output:     $ZSTD_BLOB"
