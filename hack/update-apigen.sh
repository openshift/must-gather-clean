#!/bin/bash

set -e

# this hash contains support for additionalProperties that we make use of, 0.9.0 does not contain it yet
go install github.com/atombender/go-jsonschema/cmd/gojsonschema@v0.9.1-0.20210803111510-5167a44a3a0c
gojsonschema -p schema pkg/schema/schema.json >pkg/schema/schema.go
