#!/usr/bin/env bash
# Release script

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

VERSION=$1

if [ -z "$VERSION" ]; then
    echo -e "${RED}Error: Version required${NC}"
    echo "Usage: ./scripts/release.sh v1.0.0"
    exit 1
fi

echo -e "${GREEN}Creating release ${VERSION}...${NC}"

# Check git status
if [ -n "$(git status --porcelain)" ]; then
    echo -e "${RED}Error: Working directory is not clean${NC}"
    exit 1
fi

# Run tests
echo "Running tests..."
{{test_command}}

# Update version
echo "Updating version..."
{{version_update_command}}

# Build
echo "Building..."
{{build_command}}

# Commit changes
git add .
git commit -m "chore: release ${VERSION}"

# Create tag
git tag -a ${VERSION} -m "Release ${VERSION}"

# Push
git push origin main --tags

echo -e "${GREEN}✓ Release ${VERSION} created!${NC}"
