package constants

const (
	OperatorID           = "vault-unsealer.bakito.net"
	LabelStatefulSetName = OperatorID + "/stateful-set"

	DefaultSecretPath = "vault/data/unseal-keys" // #nosec G101 not a secret

	KeyPassword        = "password"
	KeyPrefixUnsealKey = "unsealKey"
	KeySecretPath      = "secretPath"
	KeyUsername        = "username"

	EnvDeploymentName        = "DEPLOYMENT_NAME"
	EnvDevelopmentMode       = "DEVELOPMENT_MODE"
	EnvDevelopmentModeSchema = "DEVELOPMENT_MODE_SCHEMA"
	EnvVaultAddr             = "VAULT_ADDR"
	EnvWatchNamespace        = "WATCH_NAMESPACE"

	ContainerNameVault = "vault"
)
