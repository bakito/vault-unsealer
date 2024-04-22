.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: test
test: lint test-ci ## Run tests.

.PHONY: test-ci
test-ci: manifests generate ginkgo ## Run tests.
	$(GINKGO) --cover --coverprofile cover.out ./...
	go tool cover -func=cover.out

# Run go lint against code
lint: golangci-lint
	$(GOLANGCI_LINT) run --fix

## Tool Binaries
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
SEMVER ?= $(LOCALBIN)/semver
HELM_DOCS ?= $(LOCALBIN)/helm-docs

## Tool Versions
CONTROLLER_TOOLS_VERSION ?= v0.9.2
SEMVER_VERSION ?= latest
HELM_DOCS_VERSION ?= v1.11.0

port-forward:
	kubectl port-forward pod/vault-0 8200:8200 &
	kubectl port-forward pod/vault-1 8201:8200 &
	kubectl port-forward pod/vault-2 8202:8200 &

stop-port-forward:
	@pkill -f "port-forward pod/vault" || true

docker-build:
	docker build -t ghcr.io/bakito/vault-unsealer .

docker-push: docker-build
	docker push ghcr.io/bakito/vault-unsealer

release: semver goreleaser
	@version=$$($(LOCALBIN)/semver); \
	git tag -s $$version -m"Release $$version"
	$(GORELEASER) --clean

test-release: goreleaser
	$(GORELEASER) --skip=publish --snapshot --clean

docs: helm-docs update-docs
	@$(LOCALBIN)/helm-docs

update-docs: semver
	@version=$$($(LOCALBIN)/semver -next); \
	versionNum=$$($(LOCALBIN)/semver -next -numeric); \
	sed -i "s/^version:.*$$/version: $${versionNum}/"    ./chart/Chart.yaml; \
	sed -i "s/^appVersion:.*$$/appVersion: $${version}/" ./chart/Chart.yaml

helm-lint:
	helm lint ./chart

helm-template:
	helm template ./chart

## toolbox - start
## Current working directory
LOCALDIR ?= $(shell which cygpath > /dev/null 2>&1 && cygpath -m $$(pwd) || pwd)
## Location to install dependencies to
LOCALBIN ?= $(LOCALDIR)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
DEEPCOPY_GEN ?= $(LOCALBIN)/deepcopy-gen
GINKGO ?= $(LOCALBIN)/ginkgo
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
GORELEASER ?= $(LOCALBIN)/goreleaser
HELM_DOCS ?= $(LOCALBIN)/helm-docs
SEMVER ?= $(LOCALBIN)/semver

## Tool Versions
CONTROLLER_GEN_VERSION ?= v0.15.0
DEEPCOPY_GEN_VERSION ?= v0.30.0
GINKGO_VERSION ?= v2.17.1
GOLANGCI_LINT_VERSION ?= v1.57.2
GORELEASER_VERSION ?= v1.25.1
HELM_DOCS_VERSION ?= v1.13.1
SEMVER_VERSION ?= v1.1.3

## Tool Installer
.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)
.PHONY: deepcopy-gen
deepcopy-gen: $(DEEPCOPY_GEN) ## Download deepcopy-gen locally if necessary.
$(DEEPCOPY_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/deepcopy-gen || GOBIN=$(LOCALBIN) go install k8s.io/code-generator/cmd/deepcopy-gen@$(DEEPCOPY_GEN_VERSION)
.PHONY: ginkgo
ginkgo: $(GINKGO) ## Download ginkgo locally if necessary.
$(GINKGO): $(LOCALBIN)
	test -s $(LOCALBIN)/ginkgo || GOBIN=$(LOCALBIN) go install github.com/onsi/ginkgo/v2/ginkgo@$(GINKGO_VERSION)
.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	test -s $(LOCALBIN)/golangci-lint || GOBIN=$(LOCALBIN) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
.PHONY: goreleaser
goreleaser: $(GORELEASER) ## Download goreleaser locally if necessary.
$(GORELEASER): $(LOCALBIN)
	test -s $(LOCALBIN)/goreleaser || GOBIN=$(LOCALBIN) go install github.com/goreleaser/goreleaser@$(GORELEASER_VERSION)
.PHONY: helm-docs
helm-docs: $(HELM_DOCS) ## Download helm-docs locally if necessary.
$(HELM_DOCS): $(LOCALBIN)
	test -s $(LOCALBIN)/helm-docs || GOBIN=$(LOCALBIN) go install github.com/norwoodj/helm-docs/cmd/helm-docs@$(HELM_DOCS_VERSION)
.PHONY: semver
semver: $(SEMVER) ## Download semver locally if necessary.
$(SEMVER): $(LOCALBIN)
	test -s $(LOCALBIN)/semver || GOBIN=$(LOCALBIN) go install github.com/bakito/semver@$(SEMVER_VERSION)

## Update Tools
.PHONY: update-toolbox-tools
update-toolbox-tools:
	@rm -f \
		$(LOCALBIN)/controller-gen \
		$(LOCALBIN)/deepcopy-gen \
		$(LOCALBIN)/ginkgo \
		$(LOCALBIN)/golangci-lint \
		$(LOCALBIN)/goreleaser \
		$(LOCALBIN)/helm-docs \
		$(LOCALBIN)/semver
	toolbox makefile -f $(LOCALDIR)/Makefile \
		sigs.k8s.io/controller-tools/cmd/controller-gen@github.com/kubernetes-sigs/controller-tools \
		k8s.io/code-generator/cmd/deepcopy-gen@github.com/kubernetes/code-generator \
		github.com/onsi/ginkgo/v2/ginkgo \
		github.com/golangci/golangci-lint/cmd/golangci-lint \
		github.com/goreleaser/goreleaser \
		github.com/norwoodj/helm-docs/cmd/helm-docs \
		github.com/bakito/semver
## toolbox - end
