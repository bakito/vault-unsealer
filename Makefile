# Include toolbox tasks
include ./.toolbox.mk

.PHONY: manifests
manifests: tb.controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(TB_CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: tb.controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(TB_CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

fmt: tb.golines tb.gofumpt
	$(TB_GOLINES) --base-formatter="$(TB_GOFUMPT)" --max-len=120 --write-output .

.PHONY: test
test: lint test-ci ## Run tests.

.PHONY: test-ci
test-ci: manifests generate tb.ginkgo ## Run tests.
	$(TB_GINKGO) --cover --coverprofile cover.out ./...
	go tool cover -func=cover.out

# Run go lint against code
lint: tb.golangci-lint
	$(TB_GOLANGCI_LINT) run --fix


# Configuration variables
PORTS = 8200 8201 8202
BASE_PORT = 8200
REPLICAS = 0 1 2

# Port forward target for any vault-like service
port-forward-%:
	$(foreach idx,$(REPLICAS),\
		kubectl port-forward -n $* pod/$*-$(idx) $(word $(shell expr $(idx) + 1),$(PORTS)):$(BASE_PORT) & \
	)

# Alias targets for specific services
.PHONY: port-forward-openbao port-forward-vault

stop-port-forward:
	@pkill -f "port-forward -n" || true

docker-build:
	docker build -t ghcr.io/bakito/vault-unsealer .

docker-push: docker-build
	docker push ghcr.io/bakito/vault-unsealer

release: tb.semver tb.goreleaser
	@version=$$($(TB_SEMVER)); \
	git tag -s $$version -m"Release $$version"
	$(TB_GORELEASER) --clean

test-release: tb.goreleaser
	$(TB_GORELEASER) --skip=publish --snapshot --clean

.PHONY: docs
docs: tb.helm-docs update-docs
	@$(TB_HELM_DOCS)

# Detect OS
OS := $(shell uname)
# Define the sed command based on OS
SED := $(if $(filter Darwin, $(OS)), sed -i "", sed -i)
update-docs: tb.semver
	@version=$$($(TB_SEMVER) -next); \
	versionNum=$$($(TB_SEMVER) -next -numeric); \
	$(SED) "s/^version:.*$$/version: $${versionNum}/"    ./chart/Chart.yaml; \
	$(SED) "s/^appVersion:.*$$/appVersion: $${version}/" ./chart/Chart.yaml

helm-lint:
	helm lint ./chart

helm-template:
	helm template ./chart
