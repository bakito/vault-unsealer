package types

type VaultInfo struct {
	Owner      string   `json:"owner"`
	Username   string   `json:"username,omitempty"`
	Password   string   `json:"password,omitempty"`
	UnsealKeys []string `json:"unsealKeys,omitempty"`
	SecretPath string   `json:"secretPath,omitempty"`
}
