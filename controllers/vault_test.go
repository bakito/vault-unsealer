package controllers

import (
	"net"

	"github.com/bakito/vault-unsealer/pkg/types"
	"github.com/hashicorp/vault/api"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Vault", func() {
	var (
		server net.Listener
		client *api.Client
	)
	BeforeEach(func() {
	})
	AfterEach(func() {
		if server != nil {
			_ = server.Close()
		}
	})

	Context("worker", func() {
		It("read unseal keys from secret v1", func() {
			server, client = createTestVault(1, "secret/foo", map[string]interface{}{
				"unsealKey1": "foo",
				"unsealKey2": "bar",
			})
			vi := &types.VaultInfo{SecretPath: "secret/foo"}
			Ω(readSecret(client, vi)).ShouldNot(HaveOccurred())
			Ω(vi.UnsealKeys).Should(ContainElements("foo", "bar"))
		})
		It("read unseal keys from secret v2", func() {
			server, client = createTestVault(2, "secret/data/foo", map[string]interface{}{
				"unsealKey1": "foo",
				"unsealKey2": "bar",
			})
			vi := &types.VaultInfo{SecretPath: "secret/data/foo"}
			Ω(readSecret(client, vi)).ShouldNot(HaveOccurred())
			Ω(vi.UnsealKeys).Should(ContainElements("foo", "bar"))
		})
	})
})
