#!/bin/bash
# Build from project root to include source code
docker buildx build -f Dockerfile --platform "linux/386,linux/amd64,linux/arm64,linux/arm/v7,linux/arm/v6" --tag 9000000/torrserver:lite --push ../../