#!/bin/bash
cp -r ../../dist ./
docker buildx build --platform "linux/386,linux/amd64,linux/arm64,linux/arm/v7,linux/arm/v6" --tag 9000000/torrlite:$* --push .
rm -rf ./dist