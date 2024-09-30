# Include toolbox tasks
include ./.toolbox.mk

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

.PHONY: docs
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

