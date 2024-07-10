package controllers

import (
	"strings"

	"github.com/bakito/vault-unsealer/pkg/constants"
	"github.com/bakito/vault-unsealer/pkg/types"
	corev1 "k8s.io/api/core/v1"
)

func extractVaultInfo(secret corev1.Secret) *types.VaultInfo {
	v := &types.VaultInfo{
		Username:   string(secret.Data[constants.KeyUsername]),
		Password:   string(secret.Data[constants.KeyPassword]),
		Role:       string(secret.Data[constants.KeyRole]),
		SecretPath: string(secret.Data[constants.KeySecretPath]),
	}

	for key, val := range secret.Data {
		if strings.HasPrefix(key, constants.KeyPrefixUnsealKey) {
			v.UnsealKeys = append(v.UnsealKeys, string(val))
		}
	}
	return v
}
