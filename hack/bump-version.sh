#!/bin/bash

set -euo pipefail

# this script updates version number in odo source code
# run this script from root source with new version as an argument (./scripts/bump-version.sh v0.0.2 )

NEW_VERSION=$1

if [[ -z "${NEW_VERSION}" ]]; then
    echo "Version number is missing."
    echo "One argument required."
    echo "example: $0 0.0.2"
    exit 1
fi

check_version(){
    file=$1
    grep "${NEW_VERSION}" "$file"
    echo ""
}

VERSION_FILE=./cmd/must-gather-clean/version.go
echo "* Bumping version in "
sed -i "s/\(VERSION = \)\"v[0-9]*\.[0-9]*\.[0-9]*\(?:-\w+\)\?\"/\1\"v${NEW_VERSION}\"/g" $VERSION_FILE
check_version $VERSION_FILE

PREPARE_FILE=hack/rpm-prepare.sh
echo "* Bumping version in $PREPARE_FILE"
sed -i "s/\(MGC_VERSION:=\)[0-9]*\.[0-9]*\.[0-9]*/\1${NEW_VERSION}/g" $PREPARE_FILE
check_version $PREPARE_FILE
