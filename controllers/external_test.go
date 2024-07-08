package controllers

import (
	"github.com/bakito/vault-unsealer/pkg/cache"
	"github.com/bakito/vault-unsealer/pkg/constants"
	"github.com/bakito/vault-unsealer/pkg/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("PodReconciler", func() {
	var (
		sut    *ExternalHandler
		secret *corev1.Secret
	)

	BeforeEach(func() {
		sut = &ExternalHandler{
			Cache: cache.NewSimple(),
		}

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: "default",
				Labels: map[string]string{
					constants.LabelExternal: "3m0s",
				},
				Annotations: map[string]string{
					constants.AnnotationExternalSource:  "https://vault.bakito.org:8200",
					constants.AnnotationExternalTargets: "https://vault-1.bakito.org:8200;https://vault-2.bakito.org:8200",
				},
			},
		}

		sut.Cache.SetVaultInfoFor(secret.Name, &types.VaultInfo{})
	})

	It("should return correct interval", func() {
		d := sut.getInterval(*secret)
		Expect(d.String()).To(Equal(secret.Labels[constants.LabelExternal]))
	})

	It("should return source client", func() {
		c, err := sut.getSourceClient(*secret)
		Expect(err).To(BeNil())
		Expect(c).NotTo(BeNil())
	})

	It("should return target clients", func() {
		c, err := sut.getTargetClients(*secret)
		Expect(err).To(BeNil())
		Expect(len(c)).To(Equal(2))
	})
})
