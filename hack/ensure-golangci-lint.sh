#!/bin/bash
set -euo pipefail

VERSION=1.64.8
GOOS=$(go env GOOS)
TARNAME=golangci-lint-$VERSION-$GOOS-amd64.tar.gz
URL=https://github.com/golangci/golangci-lint/releases/download/v$VERSION/$TARNAME
GOLANGCI_LINT=bin/golangci-lint

case $GOOS in
    linux)
        CHECKSUM=592fc1d66c8cd64600a8fa3820f80373389c9ca18a491a2464f74f4a314c8e02
        ;;
    darwin)
        CHECKSUM=71574595b748b247aa12126f79fab03e47add27def7011dcea27a7c7f7c84580
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
