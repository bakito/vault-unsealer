package controllers

import (
	"context"

	"github.com/bakito/vault-unsealer/pkg/types"
	vc "github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault/vault"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Vault", func() {
	var (
		cluster *vault.TestCluster
		client  *vc.Client
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
			client, cluster = createTestVault("1", "foo", map[string]interface{}{
				"unsealKey1": "foo",
				"unsealKey2": "bar",
			})
			vi := &types.VaultInfo{SecretPath: "secret/foo"}
			Ω(readUnsealKeys(ctx, client, vi)).ShouldNot(HaveOccurred())
			Ω(vi.UnsealKeys).Should(ContainElements("foo", "bar"))
		})
		It("read unseal keys from secret v2", func() {
			client, cluster = createTestVault("2", "foo", map[string]interface{}{
				"unsealKey1": "foo",
				"unsealKey2": "bar",
			})
			vi := &types.VaultInfo{SecretPath: "secret/foo"}
			Ω(readUnsealKeys(ctx, client, vi)).ShouldNot(HaveOccurred())
			Ω(vi.UnsealKeys).Should(ContainElements("foo", "bar"))
		})
	})
})
