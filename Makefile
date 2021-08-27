COMMON_GOFLAGS := -mod=vendor
COMMON_LDFLAGS := -X $(PROJECT)/pkg/version.GITCOMMIT=$(GITCOMMIT)
BUILD_FLAGS := $(COMMON_GOFLAGS) -ldflags="$(COMMON_LDFLAGS)"
CROSS_BUILD_FLAGS := $(COMMON_GOFLAGS) -ldflags="-s -w $(COMMON_LDFLAGS)"

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


.PHONY: cross
cross: ## compile for multiple platforms
	./hack/compile.sh $(CROSS_BUILD_FLAGS)

.PHONY: prepare-release
prepare-release: cross ## create gzipped binaries in ./dist/release/ for uploading to GitHub release page
	./hack/prepare-release.sh
