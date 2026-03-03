#!/bin/bash

# Build script for AniList scraper
# Usage: ./build.sh [platform]
# Platforms: linux, mac, windows, all

set -e

BINARY_NAME="scraper"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "🔨 Building AniList Scraper..."
echo "Working directory: $SCRIPT_DIR"
cd "$SCRIPT_DIR"

build_for_platform() {
    local os=$1
    local arch=$2
    local output=$3

    echo "Building for $os/$arch (static)..."
    CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -o "$output" scrape.go
    echo "✅ Built: $output"
}

case "${1:-current}" in
    linux)
        build_for_platform linux amd64 "${BINARY_NAME}_linux_amd64"
        ;;
    mac)
        build_for_platform darwin amd64 "${BINARY_NAME}_mac_amd64"
        build_for_platform darwin arm64 "${BINARY_NAME}_mac_arm64"
        ;;
    windows)
        build_for_platform windows amd64 "${BINARY_NAME}_windows_amd64.exe"
        ;;
    all)
        build_for_platform linux amd64 "${BINARY_NAME}_linux_amd64"
        build_for_platform darwin amd64 "${BINARY_NAME}_mac_amd64"
        build_for_platform darwin arm64 "${BINARY_NAME}_mac_arm64"
        build_for_platform windows amd64 "${BINARY_NAME}_windows_amd64.exe"
        ;;
    current|*)
        echo "Building for current platform (static)..."
        CGO_ENABLED=0 go build -o "$BINARY_NAME" scrape.go
        echo "✅ Built: $BINARY_NAME"
        ;;
esac

echo ""
echo "🎉 Build complete!"
echo ""
echo "Usage:"
echo "  Full scrape:        ./$BINARY_NAME"
echo "  Incremental scrape: ./$BINARY_NAME -incremental"
