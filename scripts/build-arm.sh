#!/usr/bin/env bash

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist"
TARGET_OS="${GOOS:-$(go env GOOS)}"

mkdir -p "${DIST_DIR}"

GOOS="${TARGET_OS}" \
GOARCH="arm64" \
CGO_ENABLED=0 \
go build -trimpath -ldflags="-s -w" -o "${DIST_DIR}/kguard-${TARGET_OS}-arm64" "${ROOT_DIR}"

echo "Binary generated: ${DIST_DIR}/kguard-${TARGET_OS}-arm64"
