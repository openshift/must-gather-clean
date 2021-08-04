#!/bin/bash

set -e

FINAL_GEN=pkg/schema/schema.go
TMP_GEN=$(mktemp)

go run vendor/github.com/atombender/go-jsonschema/cmd/gojsonschema/main.go -p schema pkg/schema/schema.json >"${TMP_GEN}"

# make sure that we have no diff, that's important for CI to not let schema and generated structs diverge
if ! diff --unified --text "${TMP_GEN}" "${FINAL_GEN}"; then
  printf "\njsonschema is out of date. Please run hack/update-apigen.sh\n"
  exit 1
fi

rm "${TMP_GEN}"
