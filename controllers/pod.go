package controllers

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/bakito/vault-unsealer/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func getOwner(pod *corev1.Pod) string {
	for _, or := range pod.OwnerReferences {
		if or.Kind == "StatefulSet" {
			return or.Name
		}
	}
	return ""
}

func getVaultAddress(ctx context.Context, pod *corev1.Pod) string {
	if strings.EqualFold(os.Getenv(constants.EnvDevelopmentMode), "true") {
		schema := "https"
		if s, ok := os.LookupEnv(constants.EnvDevelopmentModeSchema); ok {
			schema = s
		}
		if pod.Name == "vault-0" {
			return fmt.Sprintf("%s://localhost:8200", schema)
		}
		if pod.Name == "vault-1" {
			return fmt.Sprintf("%s://localhost:8201", schema)
		}
		if pod.Name == "vault-2" {
			return fmt.Sprintf("%s://localhost:8202", schema)
		}
	}

	for _, c := range pod.Spec.Containers {
		if c.Name == constants.ContainerNameVault {
			for _, e := range c.Env {
				if e.Name == constants.EnvVaultAddr {
					u, err := url.Parse(e.Value)
					if err == nil {
						return fmt.Sprintf("%s://%s:%s", u.Scheme, pod.Status.PodIP, u.Port())
					}
					log.FromContext(ctx).Error(err, "error parsing vault url of pod.")
				}
			}
		}
	}

	return ""
}
