## toolbox - start
## Generated with https://github.com/bakito/toolbox

## Current working directory
TB_LOCALDIR ?= $(shell which cygpath > /dev/null 2>&1 && cygpath -m $$(pwd) || pwd)
## Location to install dependencies to
TB_LOCALBIN ?= $(TB_LOCALDIR)/bin
$(TB_LOCALBIN):
	if [ ! -e $(TB_LOCALBIN) ]; then mkdir -p $(TB_LOCALBIN); fi

# Helper functions
STRIP_V = $(patsubst v%,%,$(1))

## Tool Binaries
TB_CONTROLLER_GEN ?= $(TB_LOCALBIN)/controller-gen
TB_GINKGO ?= $(TB_LOCALBIN)/ginkgo
TB_GOFUMPT ?= $(TB_LOCALBIN)/gofumpt
TB_GOLANGCI_LINT ?= $(TB_LOCALBIN)/golangci-lint
TB_GOLINES ?= $(TB_LOCALBIN)/golines
TB_GORELEASER ?= $(TB_LOCALBIN)/goreleaser
TB_HELM_DOCS ?= $(TB_LOCALBIN)/helm-docs
TB_SEMVER ?= $(TB_LOCALBIN)/semver

## Tool Versions
TB_CONTROLLER_GEN_VERSION ?= v0.19.0
TB_GINKGO_VERSION ?= v2.25.2
TB_GOFUMPT_VERSION ?= v0.9.0
TB_GOLANGCI_LINT_VERSION ?= v2.4.0
TB_GOLANGCI_LINT_VERSION_NUM ?= $(call STRIP_V,$(TB_GOLANGCI_LINT_VERSION))
TB_GOLINES_VERSION ?= v0.13.0
TB_GORELEASER_VERSION ?= v2.12.0
TB_GORELEASER_VERSION_NUM ?= $(call STRIP_V,$(TB_GORELEASER_VERSION))
TB_HELM_DOCS_VERSION ?= v1.14.2
TB_SEMVER_VERSION ?= v1.1.6
TB_SEMVER_VERSION_NUM ?= $(call STRIP_V,$(TB_SEMVER_VERSION))

## Tool Installer
.PHONY: tb.controller-gen
tb.controller-gen: ## Download controller-gen locally if necessary.
	@test -s $(TB_CONTROLLER_GEN) || \
		GOBIN=$(TB_LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(TB_CONTROLLER_GEN_VERSION)
.PHONY: tb.ginkgo
tb.ginkgo: ## Download ginkgo locally if necessary.
	@test -s $(TB_GINKGO) || \
		GOBIN=$(TB_LOCALBIN) go install github.com/onsi/ginkgo/v2/ginkgo@$(TB_GINKGO_VERSION)
.PHONY: tb.gofumpt
tb.gofumpt: ## Download gofumpt locally if necessary.
	@test -s $(TB_GOFUMPT) || \
		GOBIN=$(TB_LOCALBIN) go install mvdan.cc/gofumpt@$(TB_GOFUMPT_VERSION)
.PHONY: tb.golangci-lint
tb.golangci-lint: ## Download golangci-lint locally if necessary.
	@test -s $(TB_GOLANGCI_LINT) && $(TB_GOLANGCI_LINT) --version | grep -q $(TB_GOLANGCI_LINT_VERSION_NUM) || \
		GOBIN=$(TB_LOCALBIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(TB_GOLANGCI_LINT_VERSION)
.PHONY: tb.golines
tb.golines: ## Download golines locally if necessary.
	@test -s $(TB_GOLINES) || \
		GOBIN=$(TB_LOCALBIN) go install github.com/segmentio/golines@$(TB_GOLINES_VERSION)
.PHONY: tb.goreleaser
tb.goreleaser: ## Download goreleaser locally if necessary.
	@test -s $(TB_GORELEASER) && $(TB_GORELEASER) --version | grep -q $(TB_GORELEASER_VERSION_NUM) || \
		GOBIN=$(TB_LOCALBIN) go install github.com/goreleaser/goreleaser/v2@$(TB_GORELEASER_VERSION)
.PHONY: tb.helm-docs
tb.helm-docs: ## Download helm-docs locally if necessary.
	@test -s $(TB_HELM_DOCS) || \
		GOBIN=$(TB_LOCALBIN) go install github.com/norwoodj/helm-docs/cmd/helm-docs@$(TB_HELM_DOCS_VERSION)
.PHONY: tb.semver
tb.semver: ## Download semver locally if necessary.
	@test -s $(TB_SEMVER) && $(TB_SEMVER) -version | grep -q $(TB_SEMVER_VERSION_NUM) || \
		GOBIN=$(TB_LOCALBIN) go install github.com/bakito/semver@$(TB_SEMVER_VERSION)

## Reset Tools
.PHONY: tb.reset
tb.reset:
	@rm -f \
		$(TB_CONTROLLER_GEN) \
		$(TB_GINKGO) \
		$(TB_GOFUMPT) \
		$(TB_GOLANGCI_LINT) \
		$(TB_GOLINES) \
		$(TB_GORELEASER) \
		$(TB_HELM_DOCS) \
		$(TB_SEMVER)

## Update Tools
.PHONY: tb.update
tb.update: tb.reset
	toolbox makefile -f $(TB_LOCALDIR)/Makefile \
		sigs.k8s.io/controller-tools/cmd/controller-gen@github.com/kubernetes-sigs/controller-tools \
		github.com/onsi/ginkgo/v2/ginkgo \
		mvdan.cc/gofumpt@github.com/mvdan/gofumpt \
		github.com/golangci/golangci-lint/v2/cmd/golangci-lint?--version \
		github.com/segmentio/golines \
		github.com/goreleaser/goreleaser/v2?--version \
		github.com/norwoodj/helm-docs/cmd/helm-docs \
		github.com/bakito/semver?-version
## toolbox - end
