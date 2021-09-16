#!/bin/bash

# run e2e tests on must-gather-clean

set -xeuo pipefail

current_dir="$(dirname "${BASH_SOURCE[0]}" )"
target_dir="${current_dir}/files"
test_dir="${target_dir}/testrun-${RANDOM}"
mkdir ${test_dir}
config="examples/openshift_default.yaml"

for archive in $( find "${target_dir}" -type f -name '*.tar.xz' | sort); do
    archive_name=$(basename ${archive} .tar.xz)
    testcase_dir="${test_dir}/${archive_name}"
    mkdir -p ${testcase_dir}
    tar -xJf ${archive} -C ${testcase_dir} --strip-components=1
    go test -mod=vendor -v -tags e2e ./pkg/cli -- -input="${test_dir}/${archive_name}" -report="${target_dir}/${archive_name}-report.yaml"
done

rm -rf ${test_dir}
