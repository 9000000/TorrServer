#!/bin/bash
# Build from project root to include source code

# Ensure a buildx builder with multi-platform support is used
BUILDER_NAME="torr-builder"
if ! docker buildx inspect "$BUILDER_NAME" > /dev/null 2>&1; then
  echo "Creating new buildx builder '$BUILDER_NAME' for multi-platform support..."
  docker buildx create --name "$BUILDER_NAME" --driver docker-container --use
else
  docker buildx use "$BUILDER_NAME"
fi

docker buildx build -f Dockerfile --platform "linux/amd64,linux/arm64,linux/arm/v7" --tag 9000000/torrserver:lite --push ../../