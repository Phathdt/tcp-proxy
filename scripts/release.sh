#!/bin/bash
set -e

VERSION=$1

if [ -z "$VERSION" ]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 v1.0.0"
    exit 1
fi

if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Error: Version must be in format vX.Y.Z (e.g., v1.0.0)"
    exit 1
fi

echo "Creating release $VERSION..."

git tag $VERSION
git push origin $VERSION

echo "Tag $VERSION created and pushed successfully!"
echo "GitHub Actions will now build and push the Docker image."
echo "Check: https://github.com/$(git config --get remote.origin.url | sed 's/.*://g' | sed 's/.git//g')/actions"
