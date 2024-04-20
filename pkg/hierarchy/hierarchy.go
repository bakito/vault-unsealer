package hierarchy

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/bakito/vault-unsealer/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetOwningDeployment(ctx context.Context, r client.Reader) (*appsv1.Deployment, error) {
	ns := os.Getenv(constants.EnvPodNamespace)
	if strings.EqualFold(os.Getenv(constants.EnvDevelopmentMode), "true") {
		if n := os.Getenv(constants.EnvDeploymentName); n != "" {
			return getDeployment(ctx, r, ns, n)
		}
	}
	pod := &corev1.Pod{}
	if err := r.Get(ctx, client.ObjectKey{Name: os.Getenv(constants.EnvHostname), Namespace: ns}, pod); err != nil {
		return nil, err
	}

	for _, owner := range pod.GetOwnerReferences() {
		// Check if the owner is a ReplicaSet
		if owner.Kind == "ReplicaSet" {
			// Retrieve details of the owning ReplicaSet
			rs := &appsv1.ReplicaSet{}
			err := r.Get(ctx, client.ObjectKey{Name: owner.Name, Namespace: ns}, rs)
			if err != nil {
				return nil, err
			}
			// Check if the ReplicaSet has an owner (should be a Deployment)
			for _, rsOwner := range rs.GetOwnerReferences() {
				if rsOwner.Kind == "Deployment" {
					return getDeployment(ctx, r, ns, rsOwner.Name)
				}
			}
		}
	}
	return nil, fmt.Errorf("owning deployment of pod %q not found", pod.GetName())
}

func getDeployment(ctx context.Context, r client.Reader, ns string, name string) (*appsv1.Deployment, error) {
	depl := &appsv1.Deployment{}
	err := r.Get(ctx,
		client.ObjectKey{Name: name, Namespace: ns},
		depl)
	return depl, err
}

// GetOwnedPod find all pods of the given deployment, that are ready
func GetOwnedPod(ctx context.Context, r client.Reader, depl *appsv1.Deployment) (map[string]corev1.Pod, error) {
	pods := &corev1.PodList{}
	err := r.List(ctx, pods, client.InNamespace(os.Getenv(constants.EnvPodNamespace)), client.MatchingLabels(depl.Spec.Selector.MatchLabels))
	if err != nil {
		return nil, err
	}

	ownedPods := make(map[string]corev1.Pod)
	for _, pod := range pods.Items {
		if IsReady(&pod) {
			ownedPods[pod.Status.PodIP] = pod
		}
	}

	return ownedPods, nil
}

func IsReady(pod *corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	if len(pod.Status.Conditions) > 0 {
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady &&
				condition.Status == corev1.ConditionTrue {
				return true
			}
		}
	}
	return false
}
