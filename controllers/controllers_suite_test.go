package controllers

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/helper/benchhelpers"
	"github.com/hashicorp/vault/helper/builtinplugins"
	"github.com/hashicorp/vault/http"
	"github.com/hashicorp/vault/vault"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var testingT *testing.T

func TestControllers(t *testing.T) {
	testingT = t
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers Suite")
}

func createTestVault(version string, secretPath string, data map[string]interface{}) (*api.Client, *vault.TestCluster) {
	testingT.Helper()

	coreConfig := &vault.CoreConfig{
		DisableMlock:    true,
		DisableCache:    true,
		Logger:          hclog.NewNullLogger(),
		BuiltinRegistry: builtinplugins.Registry,
	}
	opts := &vault.TestClusterOptions{
		HandlerFunc: http.Handler,
		NumCores:    1,
		KVVersion:   version,
	}

	cluster := vault.NewTestCluster(benchhelpers.TBtoT(testingT), coreConfig, opts)
	cluster.Start()

	// Make it easy to get access to the active
	core := cluster.Cores[0].Core
	vault.TestWaitActive(benchhelpers.TBtoT(testingT), core)

	// Get the client already setup for us!
	client := cluster.Cores[0].Client
	client.SetToken(cluster.RootToken)

	var err error
	if version == "2" {
		_, err = client.Logical().Write(secretPath, map[string]interface{}{
			"data": data,
		})
	} else {
		_, err = client.Logical().Write(secretPath, data)
	}
	Ω(err).ShouldNot(HaveOccurred())

	return client, cluster
}
