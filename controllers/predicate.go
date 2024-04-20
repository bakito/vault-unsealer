package controllers

import (
	"github.com/bakito/vault-unsealer/pkg/hierarchy"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func (r *PodReconciler) Create(e event.CreateEvent) bool {
	return r.matches(e.Object)
}

func (r *PodReconciler) Update(e event.UpdateEvent) bool {
	return r.matches(e.ObjectNew)
}

func (r *PodReconciler) Delete(_ event.DeleteEvent) bool {
	return false
}

func (r *PodReconciler) Generic(e event.GenericEvent) bool {
	return r.matches(e.Object)
}

func (r *PodReconciler) matches(m metav1.Object) bool {
	p, ok := m.(*corev1.Pod)
	if !ok {
		return false
	}

	if r.isUnsealer(p) {
		// we have an unsealer pod
		return true
	}

	// we have a vault pod
	return p.DeletionTimestamp == nil && hierarchy.IsReady(p) && r.hasCorrectOwner(p)
}

func (r *PodReconciler) hasCorrectOwner(pod *corev1.Pod) bool {
	owner := getOwner(pod)
	return r.Cache.VaultInfoFor(owner) != nil
}

func (r *PodReconciler) isUnsealer(pod *corev1.Pod) bool {
	return r.UnsealerSelector.Matches(labels.Set(pod.GetLabels()))
}
