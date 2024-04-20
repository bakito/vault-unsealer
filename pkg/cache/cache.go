package cache

import (
	"context"

	"github.com/bakito/vault-unsealer/pkg/types"
)

type Cache interface {
	Vaults() []string
	VaultInfoFor(vaultName string) *types.VaultInfo
	SetVaultInfoFor(vaultName string, info *types.VaultInfo)
	Sync()
	SetMember(map[string]string) bool
}

type RunnableCache interface {
	Cache
	StartCache(ctx context.Context) error
}

func NewSimple() Cache {
	return &simpleCache{vaults: make(map[string]*types.VaultInfo)}
}

type simpleCache struct {
	vaults map[string]*types.VaultInfo
}

func (s *simpleCache) SetMember(_ map[string]string) bool {
	return false
}

func (s *simpleCache) Sync() {
}

func (s *simpleCache) Vaults() []string {
	var o []string
	for k := range s.vaults {
		o = append(o, k)
	}
	return o
}

func (s *simpleCache) VaultInfoFor(vaultName string) *types.VaultInfo {
	return s.vaults[vaultName]
}

func (s *simpleCache) SetVaultInfoFor(vaultName string, info *types.VaultInfo) {
	s.vaults[vaultName] = info
}

func (s *simpleCache) StartCache(_ context.Context) error {
	return nil
}
