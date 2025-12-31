#!/bin/bash
cp -r ../../dist ./
docker buildx build --platform "linux/amd64,linux/arm64" --tag 9000000/torrlite:$* --push .
rm -rf ./dist