package constants

const (
	OperatorID           = "vault-unsealer.bakito.net"
	LabelStatefulSetName = OperatorID + "/stateful-set"

	DefaultSecretPath = "vault/data/unseal-keys" // #nosec G101 not a secret

	KeyPassword        = "password"
	KeyPrefixUnsealKey = "unsealKey"
	KeySecretPath      = "secretPath"
	KeyUsername        = "username"

	EnvDevelopmentMode       = "DEVELOPMENT_MODE"
	EnvDevelopmentModeSchema = "DEVELOPMENT_MODE_SCHEMA"
	EnvVaultAddr             = "VAULT_ADDR"
	EnvPodNamespace          = "POD_NAMESPACE"
	EnvPodName               = "POD_NAME"
	EnvHostname              = "HOSTNAME"

	ContainerNameVault = "vault"
)
