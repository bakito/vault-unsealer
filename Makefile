# Include toolbox tasks
include ./.toolbox.mk

.PHONY: manifests
manifests: tb.controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(TB_CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: tb.controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(TB_CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: test
test: lint test-ci ## Run tests.

.PHONY: test-ci
test-ci: manifests generate tb.ginkgo ## Run tests.
	$(TB_GINKGO) --cover --coverprofile cover.out ./...
	go tool cover -func=cover.out

# Run go lint against code
lint: tb.golangci-lint
	$(TB_GOLANGCI_LINT) run --fix

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

release: tb.semver tb.goreleaser
	@version=$$($(TB_SEMVER)); \
	git tag -s $$version -m"Release $$version"
	$(TB_GORELEASER) --clean

test-release: tb.goreleaser
	$(TB_GORELEASER) --skip=publish --snapshot --clean

.PHONY: docs
docs: helm-docs update-docs
	@$(TB_HELM_DOCS)

update-docs: tb.semver
	@version=$$($(TB_SEMVER) -next); \
	versionNum=$$($(TB_SEMVER) -next -numeric); \
	sed -i "s/^version:.*$$/version: $${versionNum}/"    ./chart/Chart.yaml; \
	sed -i "s/^appVersion:.*$$/appVersion: $${version}/" ./chart/Chart.yaml

helm-lint:
	helm lint ./chart

helm-template:
	helm template ./chart
