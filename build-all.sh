#!/bin/bash

PLATFORMS=(
  'linux/amd64'
  'linux/arm64'
  'linux/arm7'
  'linux/386'
)

type setopt >/dev/null 2>&1

set_goarm() {
  if [[ "$1" =~ arm([5,7]) ]]; then
    GOARCH="arm"
    GOARM="${BASH_REMATCH[1]}"
    GO_ARM="GOARM=${GOARM}"
  else
    GOARM=""
    GO_ARM=""
  fi
}
# use softfloat for mips builds
set_gomips() {
  if [[ "$1" =~ mips ]]; then
    if [[ "$1" =~ mips(64) ]]; then MIPS64="${BASH_REMATCH[1]}"; fi
    GO_MIPS="GOMIPS${MIPS64}=softfloat"
  else
    GO_MIPS=""
  fi
}

GOBIN="go"

$GOBIN version

LDFLAGS="'-s -w -checklinkname=0'"
FAILURES=""
ROOT=${PWD}
OUTPUT="${ROOT}/dist/TorrServer"

#### Build web
echo "Build web"
rm -fr web/build
rm -fr web/node_modules
cd web
export NODE_OPTIONS=--openssl-legacy-provider
npm install
npx update-browserslist-db@latest
npm run build
cd ../
$GOBIN run gen_web.go

#### Update api docs
echo "Build docs"
$GOBIN install github.com/swaggo/swag/cmd/swag@latest
cd "${ROOT}/server" || exit 1
swag init -g web/server.go

#### Build server
echo "Build server"
cd "${ROOT}/server" || exit 1
$GOBIN clean -i -r -cache --modcache
$GOBIN mod tidy

BUILD_FLAGS="-ldflags=${LDFLAGS} -tags=nosqlite -trimpath"

#####################################
### X86 build section
#####

for PLATFORM in "${PLATFORMS[@]}"; do
  GOOS=${PLATFORM%/*}
  GOARCH=${PLATFORM#*/}
  set_goarm "$GOARCH"
  set_gomips "$GOARCH"
  BIN_FILENAME="${OUTPUT}-${GOOS}-${GOARCH}${GOARM}"
  if [[ "${GOOS}" == "windows" ]]; then BIN_FILENAME="${BIN_FILENAME}.exe"; fi
  CMD="GOOS=${GOOS} GOARCH=${GOARCH} ${GO_ARM} ${GO_MIPS} CGO_ENABLED=0 ${GOBIN} build ${BUILD_FLAGS} -o ${BIN_FILENAME} ./cmd"
  echo "${CMD}"
  eval "$CMD" || FAILURES="${FAILURES} ${GOOS}/${GOARCH}${GOARM}"
  CMD="../upx -q ${BIN_FILENAME}"; # upx --brute produce much smaller binaries
  echo "compress with ${CMD}"
  eval "$CMD"
done



# eval errors
if [[ "${FAILURES}" != "" ]]; then
  echo ""
  echo "failed on: ${FAILURES}"
  exit 1
fi
