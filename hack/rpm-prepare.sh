#!/usr/bin/env bash

set -euo pipefail

echo "Reading MGC_VERSION, MCG_RELEASE and GIT_COMMIT env, if they are set"
# Change version as needed. In most cases MGC_RELEASE would not be touched unless
# we want to do a re-lease of same version as we are not backporting.
export MGC_VERSION=${MGC_VERSION:=0.0.1}
export MGC_RELEASE=${MGC_RELEASE:=1}

export GIT_COMMIT=${GIT_COMMIT:=$(git rev-parse --short HEAD 2>/dev/null)}
export MGC_RPM_VERSION=${MGC_VERSION//-}

# Golang version variables, if you are bumping this, please contact redhat maintainers to ensure that internal
# build systems can handle these versions
export GOLANG_VERSION=${GOLANG_VERSION:-1.16}
export GOLANG_VERSION_NODOT=${GOLANG_VERSION_NODOT:-116}

# Print env for verification
echo "Printing envs for verification"
echo "MGC_VERSION=$MGC_VERSION"
echo "MGC_RELEASE=$MGC_RELEASE"
echo "GIT_COMMIT=$GIT_COMMIT"
echo "MGC_RPM_VERSION=$MGC_RPM_VERSION"
echo "GOLANG_VERSION=$GOLANG_VERSION"
echo "GOLANG_VERSION_NODOT=$GOLANG_VERSION_NODOT"

OUT_DIR=".rpmbuild"
DIST_DIR="$(pwd)/dist"

SPEC_DIR="$OUT_DIR/SPECS"
SOURCES_DIR="$OUT_DIR/SOURCES"
FINAL_OUT_DIR="$DIST_DIR/rpmbuild"

NAME="must-gather-clean-$MGC_RPM_VERSION-$MGC_RELEASE"

echo "Making release for $NAME, git commit $GIT_COMMIT"

echo "Cleaning up old content"
rm -rf "$DIST_DIR"
rm -rf "$FINAL_OUT_DIR"

echo "Configuring output directory $OUT_DIR"
rm -rf $OUT_DIR
mkdir -p $SPEC_DIR
mkdir -p $SOURCES_DIR/$NAME
mkdir -p "$FINAL_OUT_DIR"

echo "Generating spec file $SPEC_DIR/must-gather-clean.spec"
envsubst <rpms/must-gather-clean.spec > $SPEC_DIR/must-gather-clean.spec

echo "Generating tarball $SOURCES_DIR/$NAME.tar.gz"
# Copy code for manipulation
cp -arf ./* $SOURCES_DIR/$NAME
pushd $SOURCES_DIR
pushd $NAME
# Remove bin if it exists, we dont need it in tarball
rm -rf ./must-gather-clean
popd

# Create tarball
tar -czf $NAME.tar.gz $NAME
# Removed copied content
rm -rf $NAME
popd

echo "Finalizing..."
# Store version information in file for reference purposes
cat << EOF > "$OUT_DIR/version"
MGC_VERSION=$MGC_VERSION
MGC_RELEASE=$MGC_RELEASE
GIT_COMMIT=$GIT_COMMIT
MGC_RPM_VERSION=$MGC_RPM_VERSION
GOLANG_VERSION=$GOLANG_VERSION
GOLANG_VERSION_NODOT=$GOLANG_VERSION_NODOT
EOF


# After success copy stuff to actual location
mv $OUT_DIR/* "$FINAL_OUT_DIR"
# Remove out dir
rm -rf $OUT_DIR
echo "Generated content in $FINAL_OUT_DIR"
