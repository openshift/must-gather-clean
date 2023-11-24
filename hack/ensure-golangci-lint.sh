#!/bin/bash
set -euo pipefail

VERSION=1.55.2
GOOS=$(go env GOOS)
TARNAME=golangci-lint-$VERSION-$GOOS-amd64.tar.gz
URL=https://github.com/golangci/golangci-lint/releases/download/v$VERSION/$TARNAME
GOLANGCI_LINT=bin/golangci-lint

case $GOOS in
    linux)
        CHECKSUM=bbc027140417125a1833a83291fa7f9516c9c6fd6244d2dded000730608fa525
        ;;
    darwin)
        CHECKSUM=632e96e6d5294fbbe7b2c410a49c8fa01c60712a0af85a567de85bcc1623ea21
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
