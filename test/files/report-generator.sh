#!/bin/bash

# generate reports from must-gather archives in current directory

set -xeuo pipefail

current_dir="$(dirname "${BASH_SOURCE[0]}" )"
test_dir="${current_dir}/testrun-${RANDOM}"
mkdir ${test_dir}
config="examples/openshift_default.yaml"

for archive in $( find "${current_dir}" -type f -name '*.tar.xz' | sort); do
    archive_name=$(basename ${archive} .tar.xz)
    testcase_dir="${test_dir}/${archive_name}"
    mkdir -p ${testcase_dir}
    tar -xJf ${archive} -C ${testcase_dir} --strip-components=1
    ./must-gather-clean -i ${testcase_dir} -o "${testcase_dir}.cleaned" -c ${config} -r "${testcase_dir}.cleaned/report"
    mv "${testcase_dir}.cleaned/report/report.yaml" "${current_dir}/${archive_name}-report.yaml"
    path=$(echo ${testcase_dir} | sed "s|^\./||")
    sed -i "s|${path}||" "${current_dir}/${archive_name}-report.yaml"
done

rm -rf ${test_dir}
