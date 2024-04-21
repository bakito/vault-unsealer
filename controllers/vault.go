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

const defaultK8sTokenFile = "/var/run/secrets/kubernetes.io/serviceaccount/token" // #nosec G101 not a secret

func (r *PodReconciler) newClient(address string) (*vault.Client, error) {
	return vault.New(
		vault.WithAddress(address),
		vault.WithRequestTimeout(30*time.Second),
		vault.WithTLS(vault.TLSConfiguration{InsecureSkipVerify: true}),
	)
}

func userPassLogin(ctx context.Context, cl *vault.Client, username string, password string) (string, error) {
	secret, err := cl.Auth.UserpassLogin(ctx, username, schema.UserpassLoginRequest{Password: password})
	if err != nil {
		return "", err
	}
	token := secret.Auth.ClientToken
	return token, nil
}

func kubernetesLogin(ctx context.Context, cl *vault.Client, role string) (string, error) {
	tokenFile := defaultK8sTokenFile

	if path, ok := constants.DevFlag(constants.EnvDevelopmentModeK8sTokenFile); ok {
		tokenFile = path
	}

	saToken, err := os.ReadFile(tokenFile)
	if err != nil {
		return "", err
	}

	secret, err := cl.Auth.KubernetesLogin(ctx, schema.KubernetesLoginRequest{Jwt: strings.TrimSpace(string(saToken)), Role: role})
	if err != nil {
		return "", err
	}
	token := secret.Auth.ClientToken
	return token, nil
}

func readSecret(ctx context.Context, cl *vault.Client, v *types.VaultInfo) error {
	mounts, err := cl.System.MountsListSecretsEngines(ctx)
	if err != nil {
		return err
	}
	mount, path := v.SecretMountAndPath()

	var data map[string]interface{}
	var warnings []string

	vers := childOf[string](mounts.Data, mount+"/", "options", "version")
	switch vers {
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
		return fmt.Errorf("unsupported kv version %q", vers)
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

func extractUnsealKeys(data map[string]interface{}, v *types.VaultInfo) {
	m := childOf[map[string]interface{}](data, "data")
	for k, val := range m {
		if strings.HasPrefix(k, constants.KeyPrefixUnsealKey) {
			v.UnsealKeys = append(v.UnsealKeys, fmt.Sprintf("%v", val))
		}
	}
}
