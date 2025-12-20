#!/usr/bin/env bash
# Build script

set -e

echo "Building gix..."

# Clean previous build
if [ -d "{{build_dir}}" ]; then
    rm -rf {{build_dir}}
fi

# Build
{{build_command}}

echo "✓ Build complete: {{build_dir}}/"
