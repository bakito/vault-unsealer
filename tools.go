//go:build tools
// +build tools

package tools

import (
	_ "github.com/bakito/semver"
	_ "github.com/golangci/golangci-lint/cmd/golangci-lint"
	_ "github.com/norwoodj/helm-docs/cmd/helm-docs"
	_ "k8s.io/code-generator/cmd/deepcopy-gen"
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
)
