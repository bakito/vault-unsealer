package cache_test

import (
	"github.com/bakito/vault-unsealer/pkg/cache"
	"github.com/bakito/vault-unsealer/pkg/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SimpleCache", func() {
	var simpleCache cache.Cache

	BeforeEach(func() {
		simpleCache = cache.NewSimple()
	})

	Describe("Vaults", func() {
		Context("when there are no vaults", func() {
			It("should return an empty list", func() {
				Expect(simpleCache.Vaults()).To(BeEmpty())
			})
		})

		Context("when there are vaults", func() {
			BeforeEach(func() {
				simpleCache.SetVaultInfoFor("statefulSet1", &types.VaultInfo{})
				simpleCache.SetVaultInfoFor("statefulSet2", &types.VaultInfo{})
			})

			It("should return a list of stateful sets", func() {
				vaults := simpleCache.Vaults()
				Expect(vaults).To(ContainElement("statefulSet1"))
				Expect(vaults).To(ContainElement("statefulSet2"))
				Expect(len(vaults)).To(Equal(2))
			})
		})
	})

	Describe("VaultInfoFor", func() {
		Context("when the stateful set has vault information", func() {
			BeforeEach(func() {
				vaultInfo := &types.VaultInfo{}
				simpleCache.SetVaultInfoFor("statefulSet1", vaultInfo)
			})

			It("should return the vault information", func() {
				info := simpleCache.VaultInfoFor("statefulSet1")
				Expect(info).NotTo(BeNil())
			})
		})

		Context("when the stateful set does not have vault information", func() {
			It("should return nil", func() {
				info := simpleCache.VaultInfoFor("statefulSet1")
				Expect(info).To(BeNil())
			})
		})
	})

	Describe("SetVaultInfoFor", func() {
		It("should set the vault information for the specified stateful set", func() {
			vaultInfo := &types.VaultInfo{}
			simpleCache.SetVaultInfoFor("statefulSet1", vaultInfo)
			info := simpleCache.VaultInfoFor("statefulSet1")
			Expect(info).To(Equal(vaultInfo))
		})
	})

	Describe("SetMember", func() {
		It("should be a no-op and return false", func() {
			Expect(simpleCache.SetMember(nil)).To(BeFalse())
		})
	})

	Describe("Sync", func() {
		It("should be a no-op", func() {
			simpleCache.Sync()
			// No assertions as it's a no-op
		})
	})
})
