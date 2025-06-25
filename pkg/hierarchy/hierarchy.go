package hierarchy

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/bakito/vault-unsealer/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetPeers returns a map of peer IPs and their associated names.
func GetPeers(ctx context.Context, r client.Reader, past132 bool) (map[string]string, error) {
	deploymentSel, err := GetDeploymentSelector(ctx, r)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment selector: %w", err)
	}

	ns := os.Getenv(constants.EnvNamespace)

	var epl client.ObjectList
	if past132 {
		epl = &discoveryv1.EndpointSliceList{}
	} else {
		epl = &corev1.EndpointsList{}
	}

	err = r.List(ctx, epl, client.InNamespace(ns), client.MatchingLabelsSelector{Selector: deploymentSel})
	if err != nil {
		return nil, fmt.Errorf("failed to list endpoints: %w", err)
	}

	if past132 {
		if epsList, ok := epl.(*discoveryv1.EndpointSliceList); ok {
			if len(epsList.Items) == 0 {
				return nil, errors.New("could not find any service endpoints")
			}
			return GetPeersFrom(&epsList.Items[0]), nil
		}
	} else {
		if epList, ok := epl.(*corev1.EndpointsList); ok {
			if len(epList.Items) == 0 {
				return nil, errors.New("could not find any service endpoints")
			}
			return GetPeersFrom(&epList.Items[0]), nil
		}
	}

	return nil, errors.New("invalid endpoint list type")
}

// GetPeersFrom extracts peer IPs and their names from the given Endpoints object.
func GetPeersFrom(obj client.Object) map[string]string {
	myIP := os.Getenv(constants.EnvPodIP)
	peers := make(map[string]string)

	switch ep := obj.(type) {
	case *corev1.Endpoints: //nolint:staticcheck // deprecation is handled
		for _, subset := range ep.Subsets {
			for _, address := range subset.Addresses {
				if address.IP != myIP {
					name := "N/A"
					if address.TargetRef != nil {
						name = address.TargetRef.Name
					}
					peers[address.IP] = name
				}
			}
		}
	case *discoveryv1.EndpointSlice:
		for _, endpoint := range ep.Endpoints {
			if endpoint.Conditions.Ready != nil && *endpoint.Conditions.Ready {
				println("@@@@@@@@@@@@@@@@@@@@@@@@@@@2")
				for _, address := range endpoint.Addresses {
					if address != myIP {
						name := "N/A"
						if endpoint.TargetRef != nil {
							name = endpoint.TargetRef.Name
						}
						peers[address] = name
					}
				}
			}
		}
	}

	return peers
}

// GetDeploymentSelector retrieves the selector for the owning Deployment of the pod.
func GetDeploymentSelector(ctx context.Context, r client.Reader) (labels.Selector, error) {
	ns := os.Getenv(constants.EnvNamespace)

	if deploymentName, ok := constants.DevFlag(constants.EnvDeploymentName); ok {
		return getDeploymentSelectorInternal(ctx, r, ns, deploymentName)
	}

	pod := &corev1.Pod{}
	err := r.Get(ctx, client.ObjectKey{Name: os.Getenv(constants.EnvPodName), Namespace: ns}, pod)
	if err != nil {
		return nil, fmt.Errorf("failed to get pod: %w", err)
	}

	deploymentName, err := GetDeploymentNameFromPod(ctx, r, ns, pod)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment name: %w", err)
	}

	return getDeploymentSelectorInternal(ctx, r, ns, deploymentName)
}

// GetDeploymentNameFromPod retrieves the name of the Deployment owning the given pod.
func GetDeploymentNameFromPod(ctx context.Context, r client.Reader, ns string, pod *corev1.Pod) (string, error) {
	for _, owner := range pod.GetOwnerReferences() {
		if owner.Kind != "ReplicaSet" {
			continue
		}

		rs := &appsv1.ReplicaSet{}
		err := r.Get(ctx, client.ObjectKey{Name: owner.Name, Namespace: ns}, rs)
		if err != nil {
			return "", fmt.Errorf("failed to get ReplicaSet: %w", err)
		}

		for _, rsOwner := range rs.GetOwnerReferences() {
			if rsOwner.Kind == "Deployment" {
				return rsOwner.Name, nil
			}
		}
	}
	return "", errors.New("owning deployment of pod not found")
}

// getDeploymentSelectorInternal retrieves the selector for the given Deployment.
func getDeploymentSelectorInternal(ctx context.Context, r client.Reader, ns, name string) (labels.Selector, error) {
	depl := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKey{Name: name, Namespace: ns}, depl)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment: %w", err)
	}

	sel, err := metav1.LabelSelectorAsSelector(depl.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("failed to convert label selector: %w", err)
	}
	return sel, nil
}
