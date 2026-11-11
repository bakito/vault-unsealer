package controllers

import (
	"context"

	"github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"

	"github.com/bakito/vault-unsealer/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// mockVaultClient implements VaultClient for testing.
type mockVaultClient struct {
	mounts   *vault.Response[map[string]any]
	kvV1Data *vault.Response[map[string]any]
	kvV2Data *vault.Response[schema.KvV2ReadResponse]
}

func (m *mockVaultClient) MountsListSecretsEngines(
	_ context.Context,
	_ ...vault.RequestOption,
) (*vault.Response[map[string]any], error) {
	return m.mounts, nil
}

func (m *mockVaultClient) KvV1Read(
	_ context.Context,
	_ string,
	_ ...vault.RequestOption,
) (*vault.Response[map[string]any], error) {
	return m.kvV1Data, nil
}

func (m *mockVaultClient) KvV2Read(
	_ context.Context,
	_ string,
	_ ...vault.RequestOption,
) (*vault.Response[schema.KvV2ReadResponse], error) {
	return m.kvV2Data, nil
}

// newMockVaultClient creates a mock VaultClient for the given KV version and data.
func newMockVaultClient(version string, data map[string]any) *mockVaultClient {
	return &mockVaultClient{
		mounts: &vault.Response[map[string]any]{
			Data: map[string]any{
				"secret/": map[string]any{
					"options": map[string]any{
						"version": version,
					},
				},
			},
		},
		kvV1Data: &vault.Response[map[string]any]{
			Data: data,
		},
		kvV2Data: &vault.Response[schema.KvV2ReadResponse]{
			Data: schema.KvV2ReadResponse{
				Data: data,
			},
		},
	}
}

var _ = Describe("Vault", func() {
	var ctx context.Context

	BeforeEach(func() {
		ctx = context.TODO()
	})

	Context("readUnsealKeys", func() {
		It("reads unseal keys from secret v1", func() {
			cl := newMockVaultClient("1", map[string]any{
				"unsealKey1": "foo",
				"unsealKey2": "bar",
			})
			vi := &types.VaultInfo{SecretPath: "secret/foo"}
			Ω(readUnsealKeys(ctx, cl, vi)).ShouldNot(HaveOccurred())
			Ω(vi.UnsealKeys).Should(ContainElements("foo", "bar"))
		})

		It("reads unseal keys from secret v2", func() {
			cl := newMockVaultClient("2", map[string]any{
				"unsealKey1": "foo",
				"unsealKey2": "bar",
			})
			vi := &types.VaultInfo{SecretPath: "secret/foo"}
			Ω(readUnsealKeys(ctx, cl, vi)).ShouldNot(HaveOccurred())
			Ω(vi.UnsealKeys).Should(ContainElements("foo", "bar"))
		})
	})
})
