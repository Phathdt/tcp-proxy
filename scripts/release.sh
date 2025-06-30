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

# Extract repository info from git remote
REMOTE_URL=$(git config --get remote.origin.url)
if [[ $REMOTE_URL =~ github\.com[:/]([^/]+)/([^/]+)(\.git)?$ ]]; then
    REPO_OWNER="${BASH_REMATCH[1]}"
    REPO_NAME="${BASH_REMATCH[2]}"
else
    echo "Error: Could not parse GitHub repository from remote URL: $REMOTE_URL"
    exit 1
fi

echo "Repository: $REPO_OWNER/$REPO_NAME"
echo "Creating release $VERSION..."

# Create and push git tag
git tag $VERSION
git push origin $VERSION

echo "✅ Tag $VERSION created and pushed successfully!"
echo "🚀 GitHub Actions will automatically:"
echo "   • Create the GitHub release"
echo "   • Build and push the Docker image"
echo "🔗 Monitor progress: https://github.com/$REPO_OWNER/$REPO_NAME/actions"
echo "🔗 Releases: https://github.com/$REPO_OWNER/$REPO_NAME/releases"
