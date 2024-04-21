package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	vc "github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
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

func createTestVault(version string, path string, data map[string]interface{}) (*vc.Client, *vault.TestCluster) {
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

	cl, err := vc.New(
		vc.WithAddress(cluster.Cores[0].Client.Address()),
		vc.WithRequestTimeout(30*time.Second),
		vc.WithTLS(vc.TLSConfiguration{InsecureSkipVerify: true}),
	)
	Ω(err).ShouldNot(HaveOccurred())

	err = cl.SetToken(cluster.RootToken)
	Ω(err).ShouldNot(HaveOccurred())

	ctx := context.TODO()
	if version == "2" {
		_, err = cl.Secrets.KvV2Write(
			ctx,
			path,
			schema.KvV2WriteRequest{
				Data: data,
			},
			vc.WithMountPath("secret"),
		)
	} else {
		_, err = cl.Secrets.KvV1Write(
			ctx,
			path,
			data,
			vc.WithMountPath("secret"),
		)
	}
	Ω(err).ShouldNot(HaveOccurred())

	return cl, cluster
}
