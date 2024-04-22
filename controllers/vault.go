package controllers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bakito/vault-unsealer/pkg/constants"
	"github.com/bakito/vault-unsealer/pkg/types"
	"github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
)

// defaultK8sTokenFile is the default path for the Kubernetes service account token file.
const defaultK8sTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token" // #nosec G101 not a secret

// newClient creates a new Vault client with the specified address.
func (r *PodReconciler) newClient(address string) (*vault.Client, error) {
	return vault.New(
		vault.WithAddress(address),
		vault.WithRequestTimeout(30*time.Second),
		vault.WithTLS(vault.TLSConfiguration{InsecureSkipVerify: true}),
	)
}

// userPassLogin performs authentication with Vault using username/password.
func userPassLogin(ctx context.Context, cl *vault.Client, username string, password string) (string, error) {
	secret, err := cl.Auth.UserpassLogin(ctx, username, schema.UserpassLoginRequest{Password: password})
	if err != nil {
		return "", err
	}
	token := secret.Auth.ClientToken
	return token, nil
}

// kubernetesLogin performs authentication with Vault using Kubernetes JWT.
func kubernetesLogin(ctx context.Context, cl *vault.Client, role string) (string, error) {
	// Get the path to the Kubernetes service account token file.
	tokenFile := defaultK8sTokenFile

	// Check if the token file path is overridden in development mode.
	if path, ok := constants.DevFlag(constants.EnvDevelopmentModeK8sTokenFile); ok {
		tokenFile = path
	}

	// Read the Kubernetes service account token from the file.
	saToken, err := os.ReadFile(tokenFile)
	if err != nil {
		return "", err
	}

	// Authenticate with Vault using Kubernetes JWT.
	secret, err := cl.Auth.KubernetesLogin(ctx, schema.KubernetesLoginRequest{Jwt: strings.TrimSpace(string(saToken)), Role: role})
	if err != nil {
		return "", err
	}
	token := secret.Auth.ClientToken
	return token, nil
}

// readUnsealKeys reads the unseal keys from Vault for the given VaultInfo.
func readUnsealKeys(ctx context.Context, cl *vault.Client, v *types.VaultInfo) error {
	mounts, err := cl.System.MountsListSecretsEngines(ctx)
	if err != nil {
		return err
	}
	mount, path := v.SecretMountAndPath()

	var data map[string]interface{}
	var warnings []string

	version := childOf[string](mounts.Data, mount+"/", "options", "version")
	switch version {
	case "1":
		sec, err := cl.Secrets.KvV1Read(ctx, path, vault.WithMountPath(mount))
		if err != nil {
			return err
		}
		data = sec.Data
		warnings = sec.Warnings
	case "2":
		sec, err := cl.Secrets.KvV2Read(ctx, path, vault.WithMountPath(mount))
		if err != nil {
			return err
		}
		data = sec.Data.Data
		warnings = sec.Warnings
	default:
		return fmt.Errorf("unsupported kv version %q", version)
	}

	if data == nil {
		return fmt.Errorf("did not receive a valid secret with path %s", v.SecretPath)
	}

	if len(warnings) > 0 {
		return errors.New(strings.Join(warnings, ","))
	}

	extractUnsealKeys(data, v)
	return nil
}

// childOf retrieves a nested value from a map[string]interface{}.
func childOf[T interface{}](m interface{}, key ...string) T {
	var empty T
	if mm, ok := m.(map[string]interface{}); ok {
		if len(key) == 1 {
			if t, ok := mm[key[0]].(T); ok {
				return t
			}
			return empty
		}
		return childOf[T](mm[key[0]], key[1:]...)
	}
	return empty
}

// extractUnsealKeys extracts unseal keys from the secret data.
func extractUnsealKeys(data map[string]interface{}, v *types.VaultInfo) {
	for k, val := range data {
		if strings.HasPrefix(k, constants.KeyPrefixUnsealKey) {
			v.UnsealKeys = append(v.UnsealKeys, fmt.Sprintf("%v", val))
		}
	}
}

// unseal unseals the Vault using the provided unseal keys.
func (r *PodReconciler) unseal(ctx context.Context, cl *vault.Client, vault *types.VaultInfo) error {
	for _, key := range vault.UnsealKeys {
		resp, err := cl.System.Unseal(ctx, schema.UnsealRequest{Key: key})
		if err != nil {
			return err
		}
		if !resp.Data.Sealed {
			return nil
		}
	}
	return errors.New("could not unseal vault")
}
