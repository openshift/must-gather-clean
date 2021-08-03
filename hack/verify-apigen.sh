#!/bin/bash

set -e

FINAL_GEN=pkg/schema/schema.go
TMP_GEN=pkg/schema/schema_tmp.go

# this hash contains support for additionalProperties that we make use of, 0.9.0 does not contain it yet
go install github.com/atombender/go-jsonschema/cmd/gojsonschema@v0.9.1-0.20210803111510-5167a44a3a0c
gojsonschema -p schema pkg/schema/schema.json >"${TMP_GEN}"

# make sure that we have no diff, that's important for CI to not let schema and generated structs diverge
if ! diff --unified --text "${TMP_GEN}" "${FINAL_GEN}"; then
  printf "\njsonschema is out of date. Please run hack/update-apigen.sh\n"
  exit 1
fi

rm ${TMP_GEN}
