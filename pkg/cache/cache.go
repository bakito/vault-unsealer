package cache

import (
	"context"

	"github.com/bakito/vault-unsealer/pkg/types"
)

type Cache interface {
	Owners() []string
	VaultInfoFor(owner string) *types.VaultInfo
	SetVaultInfoFor(owner string, info *types.VaultInfo)
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

func (s *simpleCache) Owners() []string {
	var o []string
	for k := range s.vaults {
		o = append(o, k)
	}
	return o
}

func (s *simpleCache) VaultInfoFor(owner string) *types.VaultInfo {
	return s.vaults[owner]
}

func (s *simpleCache) SetVaultInfoFor(owner string, info *types.VaultInfo) {
	s.vaults[owner] = info
}

func (s *simpleCache) StartCache(_ context.Context) error {
	return nil
}
