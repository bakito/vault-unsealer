package cache

import (
	"context"

	"github.com/bakito/vault-unsealer/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

var log = ctrl.Log.WithName("cache")

type Cache interface {
	VaultInfoFor(owner string) *types.VaultInfo
	SetVaultInfoFor(owner string, info *types.VaultInfo)
}
type RunnableCache interface {
	Cache
	Start() error
}

func NewSimple() Cache {
	return &simpleCache{vaults: make(map[string]*types.VaultInfo)}
}

type simpleCache struct {
	vaults map[string]*types.VaultInfo
}

func (s simpleCache) VaultInfoFor(owner string) *types.VaultInfo {
	return s.vaults[owner]
}

func (s simpleCache) SetVaultInfoFor(owner string, info *types.VaultInfo) {
	s.vaults[owner] = info
}

func (s simpleCache) Start(_ context.Context) error {
	return nil
}

func NewClustered() RunnableCache {
	return &clusteredCache{simpleCache: simpleCache{vaults: make(map[string]*types.VaultInfo)}}
}
