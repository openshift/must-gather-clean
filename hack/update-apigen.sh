#!/bin/bash

set -e

go run cmd/jsonschema/main.go -p schema pkg/schema/schema.json >pkg/schema/schema.go
