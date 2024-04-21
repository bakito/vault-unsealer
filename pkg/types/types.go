package types

import (
	"strings"

	"k8s.io/apimachinery/pkg/util/json"
)

type VaultInfo struct {
	Owner      string   `json:"owner"`
	Username   string   `json:"username,omitempty"`
	Password   string   `json:"password,omitempty"`
	UnsealKeys []string `json:"unsealKeys,omitempty"`
	SecretPath string   `json:"secretPath,omitempty"`
	Role       string   `json:"role,omitempty"`
}

func (i *VaultInfo) ShouldShare() bool {
	return len(i.UnsealKeys) > 0
}

func (i *VaultInfo) JSON() []byte {
	b, _ := json.Marshal(i)
	return b
}

func (i *VaultInfo) SecretMountAndPath() (string, string) {
	parts := strings.SplitN(i.SecretPath, "/", 2)

	return parts[0], parts[1]
}
