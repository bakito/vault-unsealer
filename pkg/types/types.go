package types

import (
	"strings"

	"k8s.io/apimachinery/pkg/util/json"
)

// VaultInfo represents the configuration data for a Vault instance.
type VaultInfo struct {
	StatefulSet string   `json:"statefulSet"`
	Username    string   `json:"username,omitempty"`
	Password    string   `json:"password,omitempty"`
	UnsealKeys  []string `json:"unsealKeys,omitempty"`
	SecretPath  string   `json:"secretPath,omitempty"`
	Role        string   `json:"role,omitempty"`
}

// ShouldShare returns true if the Vault instance should share its unseal keys.
func (i *VaultInfo) ShouldShare() bool {
	return len(i.UnsealKeys) > 0
}

// JSON returns the JSON representation of the VaultInfo struct.
func (i *VaultInfo) JSON() ([]byte, error) {
	return json.Marshal(i)
}

// SecretMountAndPath returns the mount and path components of the secret path.
func (i *VaultInfo) SecretMountAndPath() (string, string) {
	// Split the secret path into mount and path components
	parts := strings.SplitN(i.SecretPath, "/", 2)

	// Ensure that the secret path contains both mount and path components
	if len(parts) != 2 {
		return "", ""
	}

	return parts[0], parts[1]
}
