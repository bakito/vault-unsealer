package cache

import (
	"context"

	"github.com/bakito/vault-unsealer/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type Cache interface {
	Vaults() []string
	VaultInfoFor(statefulSet string) *types.VaultInfo
	SetVaultInfoFor(statefulSet string, info *types.VaultInfo)
	Sync()
	SetMember(map[string]string) bool
}

type RunnableCache interface {
	Cache
	manager.Runnable
	SetupWithManager(mgr ctrl.Manager) error
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

func (s *simpleCache) VaultInfoFor(statefulSet string) *types.VaultInfo {
	return s.vaults[statefulSet]
}

func (s *simpleCache) SetVaultInfoFor(statefulSet string, info *types.VaultInfo) {
	s.vaults[statefulSet] = info
}

func (s *simpleCache) StartCache(_ context.Context) error {
	return nil
}
