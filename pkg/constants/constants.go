package constants

const (
	OperatorID           = "vault-unsealer.bakito.net"
	LabelStatefulSetName = OperatorID + "/stateful-set"

	DefaultSecretPath = "vault/data/unseal-keys" // #nosec G101 not a secret

	KeyUsername        = "username"
	KeyPassword        = "password"
	KeySecretPath      = "secretPath"
	KeyPrefixUnsealKey = "unsealKey"

	EnvVaultAddr             = "VAULT_ADDR"
	EnvDevelopmentMode       = "DEVELOPMENT_MODE"
	EnvDevelopmentModeSchema = "DEVELOPMENT_MODE_SCHEMA"
	EnvWatchNamespace        = "WATCH_NAMESPACE"

	ContainerNameVault = "vault"
)
