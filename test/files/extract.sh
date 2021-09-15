#!/bin/bash

# example script how files in this dir can be extracted and manipulated

set -euo pipefail

current_dir=$(dirname "${BASH_SOURCE[0]}" ) 

for f in $( find "${current_dir}" -type f -name '*.tar.xz' | sort); do
    name=${current_dir}/$(basename $f .tar.xz)
    echo $name
    file ${f}
    tar -xJf ${f} -C ${current_dir}
    ls -lh ${name}
    rm -rf ${name}
done
