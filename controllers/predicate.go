package controllers

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// Create is invoked when a new Pod is created.
func (r *PodReconciler) Create(e event.CreateEvent) bool {
	return r.matches(e.Object)
}

// Update is invoked when an existing Pod is updated.
func (r *PodReconciler) Update(e event.UpdateEvent) bool {
	return r.matches(e.ObjectNew)
}

// Delete is invoked when a Pod is deleted.
func (r *PodReconciler) Delete(_ event.DeleteEvent) bool {
	return false
}

// Generic is invoked when a generic event occurs on a Pod.
func (r *PodReconciler) Generic(e event.GenericEvent) bool {
	return r.matches(e.Object)
}

// matches checks if the given object meets the criteria for reconciliation.
func (r *PodReconciler) matches(m metav1.Object) bool {
	p, ok := m.(*corev1.Pod)
	if !ok {
		return false
	}

	// Check if the Pod is running and has the correct owner.
	return p.DeletionTimestamp == nil && p.Status.Phase == corev1.PodRunning && r.hasCorrectOwner(p)
}

// hasCorrectOwner checks if the given Pod has the correct owner (StatefulSet).
func (r *PodReconciler) hasCorrectOwner(pod *corev1.Pod) bool {
	owner := getStatefulSetFor(pod)
	return r.Cache.VaultInfoFor(owner) != nil
}
