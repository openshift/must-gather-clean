#!/bin/bash

set -e

go run vendor/github.com/atombender/go-jsonschema/cmd/gojsonschema/main.go -p schema pkg/schema/schema.json >pkg/schema/schema.go
