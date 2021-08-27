#!/bin/bash
set -euo pipefail

# this script assumes that it runs on linux

BIN_DIR="./dist/bin"
RELEASE_DIR="./dist/release"

mkdir -p $RELEASE_DIR

archs=$(ls -1 $BIN_DIR/)

# gzipped binaries
for arch in $archs;do
    suffix=""
    if [[ $arch == windows-* ]]; then
        suffix=".exe"
    fi
    source_file=$BIN_DIR/$arch/must-gather-clean$suffix
    source_dir=$BIN_DIR/$arch
    source_filename=must-gather-clean$suffix
    target_file=$RELEASE_DIR/must-gather-clean-$arch$suffix

    # Create a tar.gz of the binary
    if [[ $suffix == .exe ]]; then
        echo "zipping binary $source_file as $target_file.zip"
        zip -9 -y -r -q "$target_file.zip" "$source_dir/$source_filename"
    else
        echo "gzipping binary $source_file as $target_file.tar.gz"
        tar -czvf "$target_file.tar.gz" --directory="$source_dir" "$source_filename"
    fi

    # Move binaries to the release directory as well
    echo "copying binary $source_file to release directory"
    cp "$source_file" "$target_file"
done

function release_sha() {
    echo "generating SHA256_SUM for release packages"
    release_dir_files=$(find $RELEASE_DIR -maxdepth 1 ! -name SHA256_SUM -type f -printf "%f\n")
    for filename in $release_dir_files; do
        sha_sum=$(sha256sum $RELEASE_DIR/"${filename}"|awk '{ print $1 }'); echo "$sha_sum"  "$filename";
    done > ${RELEASE_DIR}/SHA256_SUM
    echo "The SHA256 SUM for the release packages are:"
    cat ${RELEASE_DIR}/SHA256_SUM
}

release_sha

