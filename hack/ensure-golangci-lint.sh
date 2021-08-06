#!/bin/bash
set -euo pipefail

VERSION=1.41.1
GOOS=$(go env GOOS)
TARNAME=golangci-lint-$VERSION-$GOOS-amd64.tar.gz
URL=https://github.com/golangci/golangci-lint/releases/download/v$VERSION/$TARNAME
GOLANGCI_LINT=bin/golangci-lint

case $GOOS in
    linux)
        CHECKSUM=2ad1b2313f89d8ecb35f2e4d2463cf5bc1e057518e4c523565f09416046c21f7
        ;;
    darwin)
        CHECKSUM=458f15c43f72b0bd905ff165c42cf474e5510f91f4a10ae025e42e22fe8b578f
        ;;
    *)
        echo "Unknown GOOS $GOOS"
        exit 1
        ;;
esac

# If golangci-lint exists locally verify checksum
if [ -f $GOLANGCI_LINT ]; then
    if echo "$CHECKSUM $GOLANGCI_LINT" | sha256sum --check --quiet ; then
        exit 0
    else
        rm -f $GOLANGCI_LINT
    fi
fi

DESTINATION=$(mktemp -d)
curl -L -o "$DESTINATION/golangci-lint.tar.gz" "$URL"
tar xzf "$DESTINATION/golangci-lint.tar.gz" --directory="$DESTINATION"

mkdir -p bin
mv "$DESTINATION/golangci-lint-$VERSION-$GOOS-amd64/golangci-lint" $GOLANGCI_LINT

if echo "$CHECKSUM $GOLANGCI_LINT" | sha256sum --check --quiet ; then
    echo "golangci-lint downloaded and verified."
    exit 0
else
    echo "Checksum of downloaded golangci-lint cannot be verified."
    exit 1
fi
