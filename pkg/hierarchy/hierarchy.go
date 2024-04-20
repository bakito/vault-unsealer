package hierarchy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/bakito/vault-unsealer/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetPeers(ctx context.Context, r client.Reader) (map[string]string, error) {
	sel, err := GetDeploymentSelector(ctx, r)
	if err != nil {
		return nil, err
	}

	ns := os.Getenv(constants.EnvNamespace)

	spList := &corev1.EndpointsList{}
	err = r.List(ctx, spList, client.InNamespace(ns), client.MatchingLabelsSelector{Selector: sel})
	if err != nil {
		return nil, err
	}

	if len(spList.Items) == 0 {
		return nil, errors.New("could not find a service endpoint")
	}

	peers := GetPeersFrom(&spList.Items[0])

	return peers, nil
}

func GetPeersFrom(ep *corev1.Endpoints) map[string]string {
	myIP := os.Getenv(constants.EnvPodIP)
	peers := make(map[string]string)
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
	return peers
}

func GetDeploymentSelector(ctx context.Context, r client.Reader) (labels.Selector, error) {
	ns := os.Getenv(constants.EnvNamespace)
	if strings.EqualFold(os.Getenv(constants.EnvDevelopmentMode), "true") {
		if n := os.Getenv(constants.EnvDeploymentName); n != "" {
			return getDeploymentSelector(ctx, r, ns, n)
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
					return getDeploymentSelector(ctx, r, ns, rsOwner.Name)
				}
			}
		}
	}
	return nil, fmt.Errorf("owning deployment of pod %q not found", pod.GetName())
}

func getDeploymentSelector(ctx context.Context, r client.Reader, ns string, name string) (labels.Selector, error) {
	depl := &appsv1.Deployment{}
	err := r.Get(ctx, client.ObjectKey{Name: name, Namespace: ns}, depl)
	if err != nil {
		return nil, err
	}

	sel, err := metav1.LabelSelectorAsSelector(depl.Spec.Selector)
	if err != nil {
		return nil, err
	}
	return sel, err
}
