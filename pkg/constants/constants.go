package constants

import (
	"os"
	"strings"
)

const (
	OperatorID           = "vault-unsealer.bakito.net"
	LabelStatefulSetName = OperatorID + "/stateful-set"

	KeyPassword        = "password"
	KeyPrefixUnsealKey = "unsealKey"
	KeySecretPath      = "secretPath"
	KeyUsername        = "username"
	KeyRole            = "role"

	envDevelopmentMode             = "UNSEALER_DEVELOPMENT_MODE"
	EnvDevelopmentModeSchema       = "UNSEALER_DEVELOPMENT_MODE_SCHEMA"
	EnvDevelopmentModeK8sTokenFile = "UNSEALER_DEVELOPMENT_MODE_K8S_TOKEN_FILE"

	EnvDeploymentName = "UNSEALER_DEPLOYMENT_NAME"
	EnvVaultAddr      = "VAULT_ADDR"
	EnvNamespace      = "UNSEALER_NAMESPACE"
	EnvPodName        = "UNSEALER_POD_NAME"
	EnvPodIP          = "UNSEALER_POD_IP"

	ContainerNameVault = "vault"
)

func DevFlag(name string) (string, bool) {
	if !IsDevMode() {
		return "", false
	}
	return os.LookupEnv(name)
}

func IsDevMode() bool {
	return strings.EqualFold(os.Getenv(envDevelopmentMode), "true")
}
