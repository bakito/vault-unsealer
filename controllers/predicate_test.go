package controllers_test

import (
	"github.com/bakito/vault-unsealer/controllers"
	"github.com/bakito/vault-unsealer/pkg/cache"
	"github.com/bakito/vault-unsealer/pkg/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var _ = Describe("PodReconciler", func() {
	var (
		reconciler *controllers.PodReconciler
		pod        *corev1.Pod
	)

	BeforeEach(func() {
		reconciler = &controllers.PodReconciler{
			Cache: cache.NewSimple(),
		}

		reconciler.Cache.SetVaultInfoFor("test-statefulset", &types.VaultInfo{})

		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					{Kind: "StatefulSet", Name: "test-statefulset"},
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
			},
		}
	})

	Context("Create", func() {
		It("should return true for a matching Pod", func() {
			createEvent := event.CreateEvent{Object: pod}
			Expect(reconciler.Create(createEvent)).To(BeTrue())
		})

		It("should return false for a non-matching Pod", func() {
			pod.Status.Phase = corev1.PodPending
			createEvent := event.CreateEvent{Object: pod}
			Expect(reconciler.Create(createEvent)).To(BeFalse())
		})
	})

	Context("Update", func() {
		It("should return true for a matching Pod", func() {
			updateEvent := event.UpdateEvent{ObjectNew: pod}
			Expect(reconciler.Update(updateEvent)).To(BeTrue())
		})

		It("should return false for a non-matching Pod", func() {
			pod.Status.Phase = corev1.PodPending
			updateEvent := event.UpdateEvent{ObjectNew: pod}
			Expect(reconciler.Update(updateEvent)).To(BeFalse())
		})
	})

	Context("Delete", func() {
		It("should always return false", func() {
			deleteEvent := event.DeleteEvent{}
			Expect(reconciler.Delete(deleteEvent)).To(BeFalse())
		})
	})

	Context("Generic", func() {
		It("should return true for a matching Pod", func() {
			genericEvent := event.GenericEvent{Object: pod}
			Expect(reconciler.Generic(genericEvent)).To(BeTrue())
		})

		It("should return false for a non-matching Pod", func() {
			pod.Status.Phase = corev1.PodPending
			genericEvent := event.GenericEvent{Object: pod}
			Expect(reconciler.Generic(genericEvent)).To(BeFalse())
		})
	})
})
