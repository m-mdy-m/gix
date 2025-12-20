#!/usr/bin/env bash
# Docker build script

set -e

IMAGE_NAME=m-mdy-m/gix
TAG=${1:-latest}

echo "Building Docker image: ${IMAGE_NAME}:${TAG}"

docker build -t ${IMAGE_NAME}:${TAG} .

echo "✓ Docker image built: ${IMAGE_NAME}:${TAG}"
echo ""
echo "To run: docker run -p 8080:8080 ${IMAGE_NAME}:${TAG}"
