#!/usr/bin/env bash
# Clean build artifacts

echo "Cleaning build artifacts..."

# Remove build directories
rm -rf {{build_dirs}}

# Remove cache directories
rm -rf {{cache_dirs}}

echo "✓ Clean complete!"
