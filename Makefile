all: build
.PHONY: all

GO_PACKAGES=$(shell go list ./... | grep -v tools)

# Include the library makefile
include $(addprefix ./vendor/github.com/openshift/build-machinery-go/make/, \
	golang.mk \
	targets/openshift/bindata.mk \
	targets/openshift/deps.mk \
)

verify-scripts:
	bash -x hack/verify-apigen.sh

.PHONY: verify-scripts
verify: verify-scripts

update-scripts:
	hack/update-apigen.sh
.PHONY: update-scripts

ensure-golangci-lint:
	hack/ensure-golangci-lint.sh
.PHONY: ensure-golangci-lint

lint: ensure-golangci-lint
	bin/golangci-lint -c .golangci.yaml run
