package hierarchy

import (
	"context"
	"fmt"
	"os"

	"github.com/bakito/vault-unsealer/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetOwningDeployment(ctx context.Context, r client.Reader, ns string) (string, error) {
	pod := &corev1.Pod{}
	if err := r.Get(ctx, client.ObjectKey{Name: os.Getenv(constants.EnvHostname), Namespace: ns}, pod); err != nil {
		return "", err
	}

	for _, owner := range pod.GetOwnerReferences() {
		// Check if the owner is a ReplicaSet
		if owner.Kind == "ReplicaSet" {
			// Retrieve details of the owning ReplicaSet
			rs := &appsv1.ReplicaSet{}
			err := r.Get(ctx, client.ObjectKey{Name: owner.Name, Namespace: ns}, rs)
			if err != nil {
				return "", err
			}
			// Check if the ReplicaSet has an owner (should be a Deployment)
			for _, rsOwner := range rs.GetOwnerReferences() {
				if rsOwner.Kind == "Deployment" {
					return rsOwner.Name, nil
				}
			}
		}
	}
	return "", fmt.Errorf("owning deplyoment of pod %q not found", pod.GetName())
}

func GetOwnedPod(ctx context.Context, r client.Reader, ns string, deploymentName string) (map[string]corev1.Pod, error) {
	ors, err := getOwnedReplicaSets(ctx, r, ns, deploymentName)
	if err != nil {
		return nil, err
	}

	ownedPods := make(map[string]corev1.Pod)
	for _, rs := range ors {
		if err := addOwnedPods(ctx, r, rs.GetNamespace(), rs.GetName(), ownedPods); err != nil {
			return nil, err
		}
	}

	return ownedPods, nil
}

func getOwnedReplicaSets(ctx context.Context, r client.Reader, ns string, depl string) ([]appsv1.ReplicaSet, error) {
	rsl := &appsv1.ReplicaSetList{}
	if err := r.List(ctx, rsl, client.InNamespace(ns)); err != nil {
		return nil, err
	}

	var ownedRs []appsv1.ReplicaSet
	for _, rs := range rsl.Items {
		for _, or := range rs.GetOwnerReferences() {
			if or.Kind == "Deployment" && or.Name == depl {
				ownedRs = append(ownedRs, rs)
			}
		}
	}
	return ownedRs, nil
}

func addOwnedPods(ctx context.Context, r client.Reader, ns string, rs string, ownedPods map[string]corev1.Pod) error {
	pods := &corev1.PodList{}
	if err := r.List(ctx, pods, client.InNamespace(ns)); err != nil {
		return err
	}

	for _, p := range pods.Items {
		for _, or := range p.GetOwnerReferences() {
			if or.Kind == "ReplicaSet" && or.Name == rs {
				ownedPods[p.GetName()] = p
			}
		}
	}
	return nil
}
