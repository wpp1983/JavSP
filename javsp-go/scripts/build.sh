#!/bin/bash

set -e

# Build script for JavSP Go
# Usage: ./scripts/build.sh [version]

VERSION=${1:-$(git describe --tags --always --dirty)}
COMMIT=$(git rev-parse --short HEAD)
DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)

echo "Building JavSP Go version: $VERSION"
echo "Commit: $COMMIT"
echo "Date: $DATE"

# Create build directory
mkdir -p build

# Build flags
LDFLAGS="-X main.version=$VERSION -X main.commit=$COMMIT -X main.date=$DATE -w -s"

# Build for multiple platforms
platforms=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

for platform in "${platforms[@]}"; do
    IFS='/' read -ra PLATFORM_SPLIT <<< "$platform"
    GOOS=${PLATFORM_SPLIT[0]}
    GOARCH=${PLATFORM_SPLIT[1]}
    
    output_name="javsp-$GOOS-$GOARCH"
    if [ "$GOOS" = "windows" ]; then
        output_name="$output_name.exe"
    fi
    
    echo "Building for $GOOS/$GOARCH..."
    CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags "$LDFLAGS" \
        -o "build/$output_name" \
        ./cmd/javsp
    
    echo "✓ Built build/$output_name"
done

echo "All builds completed successfully!"
echo "Build artifacts are in the 'build' directory."