COMMON_GOFLAGS := -mod=vendor
COMMON_LDFLAGS := -X $(PROJECT)/pkg/version.GITCOMMIT=$(GITCOMMIT)
BUILD_FLAGS := $(COMMON_GOFLAGS) -ldflags="$(COMMON_LDFLAGS)"

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

update-scripts:
	hack/update-apigen.sh
.PHONY: update-scripts

ensure-golangci-lint:
	hack/ensure-golangci-lint.sh
.PHONY: ensure-golangci-lint

lint: ensure-golangci-lint
	bin/golangci-lint -c .golangci.yaml run

verify: verify-scripts lint

test-e2e: build
	time ./test/e2e.sh
.PHONY: test-e2e

.PHONY: cross
cross: build test ## depends on https://github.com/openshift/build-machinery-go/blob/2b271bb3a0ad466045cd6da5c9423084e9cf68f0/make/lib/golang.mk
	./hack/compile.sh $(GO_LD_FLAGS)

.PHONY: prepare-release
prepare-release: cross ## create gzipped binaries in ./dist/release/ for uploading to GitHub release page
	./hack/prepare-release.sh
