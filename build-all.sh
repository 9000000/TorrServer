#!/bin/bash

#####################################
### Platform Selection
### Controlled via environment variables from CI or defaults to all
#####

# Check if selective build is enabled (set by GitHub Actions workflow)
# If SELECTIVE_BUILD is not set or "false", build ALL platforms
IS_SELECTIVE="${SELECTIVE_BUILD:-false}"
IS_BUILD_ALL="${BUILD_ALL:-false}"

should_build() {
  local flag="$1"
  # If not selective mode OR build_all is checked => always build
  if [[ "${IS_SELECTIVE}" == "false" ]] || [[ "${IS_BUILD_ALL}" == "true" ]]; then
    return 0  # true
  fi
  # Otherwise check individual flag
  if [[ "${flag}" == "true" ]]; then
    return 0  # true
  fi
  return 1  # false
}

# Build platform arrays dynamically based on selections
PLATFORMS=()

# Linux AMD64
if should_build "${BUILD_LINUX_AMD64}"; then
  PLATFORMS+=('linux/amd64')
fi

# Linux ARM64
if should_build "${BUILD_LINUX_ARM64}"; then
  PLATFORMS+=('linux/arm64')
fi

# Linux ARM7
if should_build "${BUILD_LINUX_ARM7}"; then
  PLATFORMS+=('linux/arm7')
fi

# Windows
if should_build "${BUILD_WINDOWS}"; then
  PLATFORMS+=('windows/amd64')
  PLATFORMS+=('windows/386')
fi

# macOS
if should_build "${BUILD_MACOS}"; then
  PLATFORMS+=('darwin/amd64')
  PLATFORMS+=('darwin/arm64')
fi

# Other platforms (ARM5, 386, MIPS, RISCV64, FreeBSD)
if should_build "${BUILD_OTHER}"; then
  PLATFORMS+=('linux/arm5')
  PLATFORMS+=('linux/386')
  PLATFORMS+=('linux/mips')
  PLATFORMS+=('linux/mipsle')
  PLATFORMS+=('linux/mips64')
  PLATFORMS+=('linux/mips64le')
  PLATFORMS+=('linux/riscv64')
  PLATFORMS+=('freebsd/amd64')
  PLATFORMS+=('freebsd/arm7')
fi

# Android compilers (built separately with CGO)
ANDROID_COMPILERS=()

if should_build "${BUILD_ANDROID_ARM64}"; then
  ANDROID_COMPILERS+=("arm64:aarch64-linux-android21-clang")
fi

if should_build "${BUILD_ANDROID_ARM7}"; then
  ANDROID_COMPILERS+=("arm7:armv7a-linux-androideabi21-clang")
fi

# Other Android platforms (386, amd64) - included in "other" group
if should_build "${BUILD_OTHER}"; then
  ANDROID_COMPILERS+=("386:i686-linux-android21-clang")
  ANDROID_COMPILERS+=("amd64:x86_64-linux-android21-clang")
fi

echo "========================================="
echo "  Build Configuration"
echo "========================================="
echo "Selective mode: ${IS_SELECTIVE}"
echo "Build all override: ${IS_BUILD_ALL}"
echo ""
echo "Standard platforms to build:"
for p in "${PLATFORMS[@]}"; do
  echo "  ✅ ${p}"
done
if [[ ${#PLATFORMS[@]} -eq 0 ]]; then
  echo "  (none selected)"
fi
echo ""
echo "Android platforms to build:"
for v in "${ANDROID_COMPILERS[@]}"; do
  echo "  ✅ android/${v%:*}"
done
if [[ ${#ANDROID_COMPILERS[@]} -eq 0 ]]; then
  echo "  (none selected)"
fi
echo "========================================="

# Exit early if nothing to build
if [[ ${#PLATFORMS[@]} -eq 0 ]] && [[ ${#ANDROID_COMPILERS[@]} -eq 0 ]]; then
  echo "ERROR: No platforms selected for build!"
  exit 1
fi

#####################################
### Helper functions
#####

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

#####################################
### Build preparation
#####

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
$GOBIN clean -i -r -cache # --modcache
$GOBIN mod tidy

BUILD_FLAGS="-ldflags=${LDFLAGS} -tags=nosqlite -trimpath"

#####################################
### Standard platforms build section
#####

if [[ ${#PLATFORMS[@]} -gt 0 ]]; then
  echo ""
  echo ">>> Building standard platforms..."
  for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS=${PLATFORM%/*}
    GOARCH=${PLATFORM#*/}
    set_goarm "$GOARCH"
    set_gomips "$GOARCH"
    BIN_FILENAME="${OUTPUT}-${GOOS}-${GOARCH}${GOARM}"
    if [[ "${GOOS}" == "windows" ]]; then BIN_FILENAME="${BIN_FILENAME}.exe"; fi
    CMD="GOOS=${GOOS} GOARCH=${GOARCH} ${GO_ARM} ${GO_MIPS} ${GOBIN} build ${BUILD_FLAGS} -o ${BIN_FILENAME} ./cmd"
    echo "${CMD}"
    eval "$CMD" || FAILURES="${FAILURES} ${GOOS}/${GOARCH}${GOARM}"
  #  CMD="../upx -q ${BIN_FILENAME}"; # upx --brute produce much smaller binaries
  #  echo "compress with ${CMD}"
  #  eval "$CMD"
  done
fi

#####################################
### Android build section
#####

if [[ ${#ANDROID_COMPILERS[@]} -gt 0 ]]; then
  echo ""
  echo ">>> Building Android platforms..."

  export NDK_VERSION="25.2.9519653"
  # Auto-detect NDK toolchain path
  if [[ -n "${ANDROID_NDK_HOME}" ]]; then
    export NDK_TOOLCHAIN="${ANDROID_NDK_HOME}/toolchains/llvm/prebuilt/linux-x86_64"
  elif [[ -n "${ANDROID_HOME}" ]]; then
    export NDK_TOOLCHAIN="${ANDROID_HOME}/ndk/${NDK_VERSION}/toolchains/llvm/prebuilt/linux-x86_64"
  elif [[ -d "/usr/local/lib/android/sdk/ndk/${NDK_VERSION}" ]]; then
    # GitHub Actions default path
    export NDK_TOOLCHAIN="/usr/local/lib/android/sdk/ndk/${NDK_VERSION}/toolchains/llvm/prebuilt/linux-x86_64"
  else
    echo "WARNING: Android NDK not found, skipping Android builds"
    NDK_TOOLCHAIN=""
  fi

  if [[ -n "${NDK_TOOLCHAIN}" && -d "${NDK_TOOLCHAIN}" ]]; then
    GOOS=android

    for V in "${ANDROID_COMPILERS[@]}"; do
      GOARCH=${V%:*}
      COMPILER=${V#*:}
      export CC="$NDK_TOOLCHAIN/bin/$COMPILER"
      export CXX="$NDK_TOOLCHAIN/bin/$COMPILER++"
      set_goarm "$GOARCH"
      BIN_FILENAME="${OUTPUT}-${GOOS}-${GOARCH}${GOARM}"
      CMD="GOOS=${GOOS} GOARCH=${GOARCH} ${GO_ARM} CGO_ENABLED=1 ${GOBIN} build ${BUILD_FLAGS} -o ${BIN_FILENAME} ./cmd"
      echo "${CMD}"
      eval "${CMD}" || FAILURES="${FAILURES} ${GOOS}/${GOARCH}${GOARM}"
    #  CMD="../upx -q ${BIN_FILENAME}"; # upx --brute produce much smaller binaries
    #  echo "compress with ${CMD}"
    #  eval "$CMD"
    done
  else
    echo "Skipping Android builds - NDK toolchain not available at: ${NDK_TOOLCHAIN}"
    for V in "${ANDROID_COMPILERS[@]}"; do
      GOARCH=${V%:*}
      set_goarm "$GOARCH"
      FAILURES="${FAILURES} android/${GOARCH}${GOARM}"
    done
  fi
fi

# eval errors
if [[ "${FAILURES}" != "" ]]; then
  echo ""
  echo "========================================="
  echo "  ❌ Build failures: ${FAILURES}"
  echo "========================================="
  exit 1
fi

echo ""
echo "========================================="
echo "  ✅ All builds completed successfully!"
echo "========================================="

# Build docker if available
if [[ -f "${ROOT}/docker/lite/makedocker.sh" ]]; then
  cd "${ROOT}/docker/lite" || exit 1
  ./makedocker.sh
fi