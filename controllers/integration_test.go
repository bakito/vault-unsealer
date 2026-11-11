//go:build integration

package controllers

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
	"github.com/testcontainers/testcontainers-go"
	tcvault "github.com/testcontainers/testcontainers-go/modules/vault"

	"github.com/bakito/vault-unsealer/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var _ = Describe("Integration", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.Background()
	})

	It("reads unseal keys from KV v1 store", func() {
		vaultContainer, err := tcvault.Run(
			ctx,
			"hashicorp/vault:1.19.1",
			tcvault.WithToken("root-token"),
			tcvault.WithInitCommand("secrets enable -version=1 kv"),
			tcvault.WithInitCommand("kv put foo unsealKey1=key1 unsealKey2=key2"),
		)
		Ω(err).ShouldNot(HaveOccurred())
		defer func() {
			_ = testcontainers.TerminateContainer(vaultContainer)
		}()

		address, err := vaultContainer.HttpHostAddress(ctx)
		Ω(err).ShouldNot(HaveOccurred())

		cl, err := vault.New(
			vault.WithAddress(address),
			vault.WithRequestTimeout(30*time.Second),
			vault.WithTLS(vault.TLSConfiguration{InsecureSkipVerify: true}),
		)
		Ω(err).ShouldNot(HaveOccurred())

		err = cl.SetToken("root-token")
		Ω(err).ShouldNot(HaveOccurred())

		vi := &types.VaultInfo{SecretPath: "kv/foo"}
		Ω(readUnsealKeys(ctx, newVaultClient(cl), vi)).ShouldNot(HaveOccurred())
		Ω(vi.UnsealKeys).Should(ContainElements("key1", "key2"))
	})

	It("reads unseal keys from KV v2 store", func() {
		vaultContainer, err := tcvault.Run(
			ctx,
			"hashicorp/vault:1.19.1",
			tcvault.WithToken("root-token"),
			tcvault.WithInitCommand("secrets enable -version=2 kv"),
			tcvault.WithInitCommand("kv put kv/foo unsealKey1=key1 unsealKey2=key2"),
		)
		Ω(err).ShouldNot(HaveOccurred())
		defer func() {
			_ = testcontainers.TerminateContainer(vaultContainer)
		}()

		address, err := vaultContainer.HttpHostAddress(ctx)
		Ω(err).ShouldNot(HaveOccurred())

		cl, err := vault.New(
			vault.WithAddress(address),
			vault.WithRequestTimeout(30*time.Second),
			vault.WithTLS(vault.TLSConfiguration{InsecureSkipVerify: true}),
		)
		Ω(err).ShouldNot(HaveOccurred())

		err = cl.SetToken("root-token")
		Ω(err).ShouldNot(HaveOccurred())

		vi := &types.VaultInfo{SecretPath: "kv/foo"}
		Ω(readUnsealKeys(ctx, newVaultClient(cl), vi)).ShouldNot(HaveOccurred())
		Ω(vi.UnsealKeys).Should(ContainElements("key1", "key2"))
	})

	It("verifies mount listing data structure matches real Vault", func() {
		vaultContainer, err := tcvault.Run(
			ctx,
			"hashicorp/vault:1.19.1",
			tcvault.WithToken("root-token"),
			tcvault.WithInitCommand("secrets enable -version=2 kv"),
		)
		Ω(err).ShouldNot(HaveOccurred())
		defer func() {
			_ = testcontainers.TerminateContainer(vaultContainer)
		}()

		address, err := vaultContainer.HttpHostAddress(ctx)
		Ω(err).ShouldNot(HaveOccurred())

		cl, err := vault.New(
			vault.WithAddress(address),
			vault.WithRequestTimeout(30*time.Second),
			vault.WithTLS(vault.TLSConfiguration{InsecureSkipVerify: true}),
		)
		Ω(err).ShouldNot(HaveOccurred())

		err = cl.SetToken("root-token")
		Ω(err).ShouldNot(HaveOccurred())

		// Verify the mount listing structure matches what the mock expects
		mounts, err := cl.System.MountsListSecretsEngines(ctx)
		Ω(err).ShouldNot(HaveOccurred())

		// The mock uses "kv/" as the key — verify this matches real Vault
		kvMount, ok := mounts.Data["kv/"].(map[string]any)
		Ω(ok).Should(BeTrue(), "Mount key should be 'kv/' with map value")

		options, ok := kvMount["options"].(map[string]any)
		Ω(ok).Should(BeTrue(), "options should be a map")

		version, ok := options["version"].(string)
		Ω(ok).Should(BeTrue(), "version should be a string")
		Ω(version).Should(Equal("2"))
	})

	It("handles KV v2 write and read with vault-client-go schema", func() {
		// This test verifies the exact data flow: write via vault-client-go,
		// read via readUnsealKeys, ensuring schema.KvV2ReadResponse.Data nesting
		vaultContainer, err := tcvault.Run(
			ctx,
			"hashicorp/vault:1.19.1",
			tcvault.WithToken("root-token"),
			tcvault.WithInitCommand("secrets enable -version=2 secret"),
		)
		Ω(err).ShouldNot(HaveOccurred())
		defer func() {
			_ = testcontainers.TerminateContainer(vaultContainer)
		}()

		address, err := vaultContainer.HttpHostAddress(ctx)
		Ω(err).ShouldNot(HaveOccurred())

		cl, err := vault.New(
			vault.WithAddress(address),
			vault.WithRequestTimeout(30*time.Second),
			vault.WithTLS(vault.TLSConfiguration{InsecureSkipVerify: true}),
		)
		Ω(err).ShouldNot(HaveOccurred())

		err = cl.SetToken("root-token")
		Ω(err).ShouldNot(HaveOccurred())

		// Write using vault-client-go API (same as production)
		_, err = cl.Secrets.KvV2Write(
			ctx,
			"test-secret",
			schema.KvV2WriteRequest{
				Data: map[string]any{
					"unsealKey1": "prod-key-1",
					"unsealKey2": "prod-key-2",
					"unsealKey3": "prod-key-3",
				},
			},
			vault.WithMountPath("secret"),
		)
		Ω(err).ShouldNot(HaveOccurred())

		// Read via readUnsealKeys (the function under test)
		vi := &types.VaultInfo{SecretPath: "secret/test-secret"}
		Ω(readUnsealKeys(ctx, newVaultClient(cl), vi)).ShouldNot(HaveOccurred())
		Ω(vi.UnsealKeys).Should(ContainElements("prod-key-1", "prod-key-2", "prod-key-3"))
	})
})
