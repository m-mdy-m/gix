#!/usr/bin/env bash
# Install script for gix

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Installing gix...${NC}"

# Check requirements
if ! command -v git &> /dev/null; then
    echo -e "${RED}Error: git is not installed${NC}"
    exit 1
fi

# Clone repository
echo "Cloning repository..."
git clone https://github.com/m-mdy-m/gix
cd gix

# Install dependencies
echo "Installing dependencies..."
{{install_command}}

echo -e "${GREEN}✓ Installation complete!${NC}"
echo ""
echo "To get started:"
echo "  cd gix"
echo "  {{start_command}}"
