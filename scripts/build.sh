#!/bin/bash
set -e

IMAGE_NAME="phathdt379/tcp-proxy"
DOCKERFILE_PATH="."
PLATFORM="linux/amd64,linux/arm64"

show_usage() {
    echo "Usage: $0 [VERSION] [push]"
    echo ""
    echo "Arguments:"
    echo "  VERSION    Image version tag (default: latest)"
    echo "  push       Push to registry after building (optional)"
    echo ""
    echo "Examples:"
    echo "  $0                    # Build with 'latest' tag locally"
    echo "  $0 v1.0.0             # Build with 'v1.0.0' tag locally"
    echo "  $0 v1.0.0 push        # Build with 'v1.0.0' tag and push to registry"
    echo "  $0 latest push        # Build with 'latest' tag and push to registry"
    exit 1
}

if [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    show_usage
fi

VERSION=${1:-latest}
PUSH=${2:-false}

echo "Building Docker image: $IMAGE_NAME:$VERSION"

if [ "$PUSH" = "true" ] || [ "$PUSH" = "push" ]; then
    if ! command -v docker &> /dev/null; then
        echo "❌ Error: Docker is not installed or not in PATH"
        exit 1
    fi

    if ! docker buildx version &> /dev/null; then
        echo "❌ Error: Docker buildx is not available"
        echo "💡 Install buildx or use \"docker build\" for single platform builds"
        exit 1
    fi
    echo "Building multi-platform image and pushing to registry..."
    docker buildx build \
        --platform $PLATFORM \
        --tag $IMAGE_NAME:$VERSION \
        --tag $IMAGE_NAME:latest \
        --push \
        $DOCKERFILE_PATH
else
    echo "Building local image..."
    docker build \
        --tag $IMAGE_NAME:$VERSION \
        --tag $IMAGE_NAME:latest \
        $DOCKERFILE_PATH
fi

echo "✅ Docker image built successfully!"
echo "Image tags:"
echo "  - $IMAGE_NAME:$VERSION"
echo "  - $IMAGE_NAME:latest"

if [ "$PUSH" = "true" ] || [ "$PUSH" = "push" ]; then
    echo "🚀 Image pushed to registry!"
else
    echo "💡 To push to registry, run: $0 $VERSION push"
fi
