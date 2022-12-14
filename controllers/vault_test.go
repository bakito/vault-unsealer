package controllers

import (
	"context"

	"github.com/bakito/vault-unsealer/pkg/types"
	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/vault"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Vault", func() {
	var (
		cluster *vault.TestCluster
		client  *api.Client
		ctx     context.Context
	)
	BeforeEach(func() {
		ctx = context.TODO()
	})
	AfterEach(func() {
		if cluster != nil {
			cluster.Cleanup()
		}
	})

	Context("worker", func() {
		It("read unseal keys from secret v1", func() {
			client, cluster = createTestVault("1", "secret/foo", map[string]interface{}{
				"unsealKey1": "foo",
				"unsealKey2": "bar",
			})
			vi := &types.VaultInfo{SecretPath: "secret/foo"}
			Ω(readSecret(ctx, client, vi)).ShouldNot(HaveOccurred())
			Ω(vi.UnsealKeys).Should(ContainElements("foo", "bar"))
		})
		It("read unseal keys from secret v2", func() {
			client, cluster = createTestVault("2", "secret/data/foo", map[string]interface{}{
				"unsealKey1": "foo",
				"unsealKey2": "bar",
			})
			vi := &types.VaultInfo{SecretPath: "secret/foo"}
			Ω(readSecret(ctx, client, vi)).ShouldNot(HaveOccurred())
			Ω(vi.UnsealKeys).Should(ContainElements("foo", "bar"))
		})
	})
})
