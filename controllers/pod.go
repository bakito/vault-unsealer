package controllers

import (
	"context"
	"fmt"
	"net/url"

	"github.com/bakito/vault-unsealer/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// getStatefulSetFor returns the name of the StatefulSet that owns the given Pod.
func getStatefulSetFor(pod *corev1.Pod) string {
	for _, or := range pod.OwnerReferences {
		if or.Kind == "StatefulSet" {
			return or.Name
		}
	}
	return ""
}

// getVaultAddress returns the address of the Vault service running in the given Pod.
func getVaultAddress(ctx context.Context, pod *corev1.Pod) string {
	// Check if development mode is enabled.
	if schema, ok := constants.DevFlag(constants.EnvDevelopmentModeSchema); ok {
		// For development mode, return the local Vault addresses based on Pod names.
		switch pod.Name {
		case "vault-0":
			return fmt.Sprintf("%s://localhost:8200", schema)
		case "vault-1":
			return fmt.Sprintf("%s://localhost:8201", schema)
		case "vault-2":
			return fmt.Sprintf("%s://localhost:8202", schema)
		}
	}

	// Iterate through containers in the Pod to find the Vault container.
	for _, c := range pod.Spec.Containers {
		if c.Name == constants.ContainerNameVault {
			// Iterate through environment variables in the container to find the Vault address.
			for _, e := range c.Env {
				if e.Name == constants.EnvVaultAddr {
					// Parse the Vault URL from the environment variable value.
					u, err := url.Parse(e.Value)
					if err == nil {
						// Construct the Vault address using the Pod's IP and port from the URL.
						return fmt.Sprintf("%s://%s:%s", u.Scheme, pod.Status.PodIP, u.Port())
					}
					// Log error if parsing the Vault URL fails.
					log.FromContext(ctx).Error(err, "error parsing vault url of pod.")
				}
			}
		}
	}

	// Return an empty string if Vault address cannot be determined.
	return ""
}
