package constants

import (
	"os"
	"strings"
)

// OperatorID is the unique identifier for the vault-unsealer operator.
const OperatorID = "vault-unsealer.bakito.net"

// LabelStatefulSetName is the label used to identify stateful sets managed by the operator.
const LabelStatefulSetName = OperatorID + "/stateful-set"

// ContainerNameVault is the default vault container name
const ContainerNameVault = "vault"

// Environment variable names
const (
	envDevelopmentMode             = "UNSEALER_DEVELOPMENT_MODE"
	EnvDevelopmentModeSchema       = "UNSEALER_DEVELOPMENT_MODE_SCHEMA"
	EnvDevelopmentModeK8sTokenFile = "UNSEALER_DEVELOPMENT_MODE_K8S_TOKEN_FILE"
	EnvDeploymentName              = "UNSEALER_DEPLOYMENT_NAME"
	EnvVaultAddr                   = "VAULT_ADDR"
	EnvNamespace                   = "UNSEALER_NAMESPACE"
	EnvPodName                     = "UNSEALER_POD_NAME"
	EnvPodIP                       = "UNSEALER_POD_IP"
)

// Secret key names
const (
	KeyPassword        = "password"
	KeyPrefixUnsealKey = "unsealKey"
	KeySecretPath      = "secretPath"
	KeyUsername        = "username"
	KeyRole            = "role"
)

// DevFlag returns the value of the given environment variable if development mode is enabled.
func DevFlag(name string) (string, bool) {
	if !IsDevMode() {
		return "", false
	}
	return os.LookupEnv(name)
}

// IsDevMode checks if development mode is enabled.
func IsDevMode() bool {
	return strings.EqualFold(os.Getenv(envDevelopmentMode), "true")
}
