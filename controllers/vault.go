package controllers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bakito/vault-unsealer/pkg/constants"
	"github.com/bakito/vault-unsealer/pkg/types"
	"github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
)

func (r *PodReconciler) newClient(address string) (*vault.Client, error) {
	return vault.New(
		vault.WithAddress(address),
		vault.WithRequestTimeout(30*time.Second),
		vault.WithTLS(vault.TLSConfiguration{InsecureSkipVerify: true}),
	)
}

func userpassLogin(ctx context.Context, cl *vault.Client, username string, password string) (string, error) {
	// PUT call to get a token
	secret, err := cl.Auth.UserpassLogin(ctx, username, schema.UserpassLoginRequest{Password: password})
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
