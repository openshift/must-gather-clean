#!/bin/env bash

set -euo pipefail

# This will cross-compile must-gather-clean for all platforms:
# Windows, Linux and macOS

if [[ -z "${*}" ]]; then
    echo "Build flags are missing"
    exit 1
fi

for platform in linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64 ; do
  echo "Cross compiling $platform and placing binary at dist/bin/$platform/"
  if [ $platform == "windows-amd64" ]; then
    GOARCH=amd64 GOOS=windows go build -o dist/bin/$platform/must-gather-clean.exe "${@}" ./cmd/must-gather-clean/
  else
    GOARCH=${platform#*-} GOOS=${platform%-*} go build -o dist/bin/$platform/must-gather-clean "${@}" ./cmd/must-gather-clean/
  fi
done

