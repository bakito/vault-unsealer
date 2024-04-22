package cache

import (
	"context"

	"github.com/bakito/vault-unsealer/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// Cache defines the interface for managing Vault information cache.
type Cache interface {
	// Vaults returns the list of stateful sets for which Vault information is cached.
	Vaults() []string
	// VaultInfoFor retrieves the Vault information for the specified stateful set.
	VaultInfoFor(statefulSet string) *types.VaultInfo
	// SetVaultInfoFor sets the Vault information for the specified stateful set.
	SetVaultInfoFor(statefulSet string, info *types.VaultInfo)
	// Sync synchronizes the cache with the external source, if applicable.
	Sync()
	// SetMember sets the member status for the cache, if applicable.
	SetMember(map[string]string) bool
}

// RunnableCache extends the Cache interface with additional methods for running as a controller-runtime Runnable.
type RunnableCache interface {
	Cache
	manager.Runnable
	// SetupWithManager sets up the cache with the provided manager for running as a controller-runtime Runnable.
	SetupWithManager(mgr ctrl.Manager) error
}

// NewSimple creates a new simple cache instance.
func NewSimple() Cache {
	return &simpleCache{vaults: make(map[string]*types.VaultInfo)}
}

type simpleCache struct {
	vaults map[string]*types.VaultInfo
}

// SetMember is a no-op for simple cache.
func (s *simpleCache) SetMember(_ map[string]string) bool {
	// No-op for simple cache
	return false
}

// Sync is a no-op for simple cache.
func (s *simpleCache) Sync() {
	// No-op for simple cache
}

// Vaults returns the list of stateful sets for which Vault information is cached.
func (s *simpleCache) Vaults() []string {
	var out []string
	for k := range s.vaults {
		out = append(out, k)
	}
	return out
}

// VaultInfoFor retrieves the Vault information for the specified stateful set.
func (s *simpleCache) VaultInfoFor(statefulSet string) *types.VaultInfo {
	return s.vaults[statefulSet]
}

// SetVaultInfoFor sets the Vault information for the specified stateful set.
func (s *simpleCache) SetVaultInfoFor(statefulSet string, info *types.VaultInfo) {
	s.vaults[statefulSet] = info
}

// StartCache starts the cache, but it's a no-op for simple cache.
func (s *simpleCache) StartCache(_ context.Context) error {
	// No-op for simple cache
	return nil
}
