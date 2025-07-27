#!/bin/bash

# Build script for nmcrun
# This script builds binaries for multiple platforms with version information

set -e

# Get version information
VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT=${GIT_COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}

# Remove 'v' prefix from version if present
VERSION=$(echo $VERSION | sed 's/^v//')

echo "Building nmcrun version: $VERSION"
echo "Build date: $BUILD_DATE"
echo "Git commit: $GIT_COMMIT"
echo ""

# Build flags with version information
LDFLAGS="-X nmcrun/internal/version.Version=$VERSION -X nmcrun/internal/version.BuildDate=$BUILD_DATE -X nmcrun/internal/version.GitCommit=$GIT_COMMIT"

# Create dist directory
mkdir -p dist

# Platforms to build for
PLATFORMS=(
    "darwin/amd64"
    "darwin/arm64"
    "linux/amd64"
    "linux/arm64"
    "windows/amd64"
)

echo "Building for platforms:"
for platform in "${PLATFORMS[@]}"; do
    echo "  - $platform"
done
echo ""

# Build for each platform
for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -r -a platform_split <<< "$platform"
    GOOS="${platform_split[0]}"
    GOARCH="${platform_split[1]}"
    
    output_name="nmcrun"
    if [ "$GOOS" = "windows" ]; then
        output_name="nmcrun.exe"
    fi
    
    archive_name="nmcrun_${VERSION}_${GOOS}_${GOARCH}"
    
    echo "Building $GOOS/$GOARCH..."
    
    # Build binary
    env GOOS=$GOOS GOARCH=$GOARCH go build -ldflags "$LDFLAGS" -trimpath -o "dist/$output_name" .
    
    # Create archive
    cd dist
    if [ "$GOOS" = "windows" ]; then
        zip "${archive_name}.zip" "$output_name"
        echo "  ✓ Created ${archive_name}.zip"
    else
        tar -czf "${archive_name}.tar.gz" "$output_name"
        echo "  ✓ Created ${archive_name}.tar.gz"
    fi
    
    # Clean up binary
    rm "$output_name"
    cd ..
done

echo ""
echo "🎉 Build completed! Archives created in dist/ directory:"
ls -la dist/

echo ""
echo "To test a build locally:"
echo "  # Extract an archive and run:"
echo "  ./nmcrun version"
echo "  ./nmcrun --help" 