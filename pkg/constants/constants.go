package constants

const (
	OperatorID           = "vault-unsealer.bakito.net"
	LabelStatefulSetName = OperatorID + "/stateful-set"

	DefaultSecretPath = "vault/data/unseal-keys" // #nosec G101 not a secret

	KeyPassword        = "password"
	KeyPrefixUnsealKey = "unsealKey"
	KeySecretPath      = "secretPath"
	KeyUsername        = "username"

	EnvDeploymentName        = "UNSEALER_DEPLOYMENT_NAME"
	EnvDevelopmentMode       = "UNSEALER_DEVELOPMENT_MODE"
	EnvDevelopmentModeSchema = "UNSEALER_DEVELOPMENT_MODE_SCHEMA"
	EnvVaultAddr             = "VAULT_ADDR"
	EnvNamespace             = "UNSEALER_NAMESPACE"
	EnvPodName               = "UNSEALER_POD_NAME"
	EnvPodIP                 = "UNSEALER_POD_IP"
	EnvHostname              = "HOSTNAME"

	ContainerNameVault = "vault"
)
