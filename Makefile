COMMON_GOFLAGS := -mod=vendor
COMMON_LDFLAGS := -X $(PROJECT)/pkg/version.GITCOMMIT=$(GITCOMMIT)
BUILD_FLAGS := $(COMMON_GOFLAGS) -ldflags="$(COMMON_LDFLAGS)"
GO ?=go
GO_PACKAGE ?=$(shell $(GO) list $(GO_MOD_FLAGS) -m -f '{{ .Path }}' || echo 'no_package_detected')
SOURCE_GIT_TAG ?=$(shell git describe --long --tags --abbrev=7 --match 'v[0-9]*' || echo 'v0.0.0-unknown')
SOURCE_GIT_COMMIT ?=$(shell git rev-parse --short "HEAD^{commit}" 2>/dev/null)
SOURCE_GIT_TREE_STATE ?=$(shell ( ( [ ! -d ".git/" ] || git diff --quiet ) && echo 'clean' ) || echo 'dirty')
define version-ldflags
-X $(GO_PACKAGE)/pkg/version.versionFromGit="$(SOURCE_GIT_TAG)" \
-X $(GO_PACKAGE)/pkg/version.commitFromGit="$(SOURCE_GIT_COMMIT)" \
-X $(GO_PACKAGE)/pkg/version.gitTreeState="$(SOURCE_GIT_TREE_STATE)" \
-X $(GO_PACKAGE)/pkg/version.buildDate="$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')"
endef
CROSS_BUILD_FLAGS := $(COMMON_GOFLAGS) -ldflags="-s -w $(version-ldflags)"

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
