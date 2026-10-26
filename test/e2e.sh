#!/bin/bash

# run e2e tests on must-gather-clean

set -Eeuo pipefail

current_dir="$(dirname "${BASH_SOURCE[0]}" )"
target_dir="${current_dir}/files"
test_dir="${target_dir}/testrun-${RANDOM}"
mkdir "${test_dir}"
trap 'rm -rf -- "${test_dir}"' EXIT
config="examples/openshift_default.yaml"

for archive in $( find "${target_dir}" -type f -name '*.tar.xz' | sort); do
    archive_name=$(basename ${archive} .tar.xz)
    testcase_dir="${test_dir}/${archive_name}"
    mkdir -p ${testcase_dir}
    tar -xJf ${archive} -C ${testcase_dir} --strip-components=1
    go test -mod=vendor -v -tags e2e -run '^TestEndToEnd$' ./pkg/cli -- -input="${testcase_dir}" -report="${target_dir}/${archive_name}-report.yaml" || exit 1
    cleaned_dir="${testcase_dir}.cleaned"
    # grep for IPv4 addresses
    is_clean=$(grep -aErho '(^|[^0-9.])((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)([^0-9.]|$)' "${cleaned_dir}" | grep -Ev '(^|[^0-9.])(0\.0\.0\.0|127\.0\.0\.1)($|[^0-9.])' || true)
    if [[ "${is_clean}" != "" ]]; then
        echo "${cleaned_dir} is dirty: [${is_clean}]"
        exit 1
    fi
    # grep for MAC addresses
    is_clean=$(grep -aErho '([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})' "${cleaned_dir}" || true)
    if [[ "${is_clean}" != "" ]]; then
        echo "${cleaned_dir} is dirty: [${is_clean}]"
        exit 1
    fi
done

if [[ ! -f "${target_dir}/azure-ci-job.tar.xz" || ! -f "${target_dir}/azure-ci-job-report.yaml" ]]; then
    echo "azure-ci-job fixture and report are required for the parallel-worker E2E test" >&2
    exit 1
fi

go test -mod=vendor -v -tags e2e -run '^TestEndToEndParallelWorkers$' ./pkg/cli -- -input="${test_dir}/azure-ci-job" -report="${target_dir}/azure-ci-job-report.yaml" || exit 1
