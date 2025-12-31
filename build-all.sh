#!/bin/bash

PLATFORMS=(
  'linux/amd64'
  'linux/arm64'
)

type setopt >/dev/null 2>&1

GOBIN="go"

$GOBIN version

LDFLAGS="'-s -w -checklinkname=0'"
FAILURES=""
ROOT=${PWD}
OUTPUT="${ROOT}/dist/TorrServer"

#### Build web
echo "Build web"
export NODE_OPTIONS=--openssl-legacy-provider
$GOBIN run gen_web.go

#### Update api docs
echo "Build docs"
$GOBIN install github.com/swaggo/swag/cmd/swag@latest
cd "${ROOT}/server" || exit 1
swag init -g web/server.go

#### Build server
echo "Build server"
cd "${ROOT}/server" || exit 1
$GOBIN clean -i -r -cache
$GOBIN mod tidy

BUILD_FLAGS="-ldflags=${LDFLAGS} -tags=nosqlite -trimpath"

for PLATFORM in "${PLATFORMS[@]}"; do
  GOOS=${PLATFORM%/*}
  GOARCH=${PLATFORM#*/}
  BIN_FILENAME="${OUTPUT}-${GOOS}-${GOARCH}"
  CMD="GOOS=${GOOS} GOARCH=${GOARCH} ${GOBIN} build ${BUILD_FLAGS} -o ${BIN_FILENAME} ./cmd"
  echo "${CMD}"
  eval "$CMD" || FAILURES="${FAILURES} ${GOOS}/${GOARCH}"
done

# eval errors
if [[ "${FAILURES}" != "" ]]; then
  echo ""
  echo "failed on: ${FAILURES}"
  exit 1
fi
