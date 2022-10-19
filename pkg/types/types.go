package types

type VaultInfo struct {
	Owner      string   `json:"owner"`
	Username   string   `json:"username"`
	Password   string   `json:"password"`
	UnsealKeys []string `json:"unsealKeys"`
	SecretPath string   `json:"secretPath"`
}
