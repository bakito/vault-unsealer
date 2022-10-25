package controllers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bakito/vault-unsealer/pkg/constants"
	"github.com/bakito/vault-unsealer/pkg/types"
	"github.com/hashicorp/vault/api"
)

func (r *PodReconciler) newClient(address string) (*api.Client, error) {
	cfg := api.DefaultConfig()
	cfg.Address = address
	cfg.HttpClient.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify = true
	return api.NewClient(cfg)
}

func userpassLogin(cl *api.Client, username string, password string) (string, error) {
	// to pass the password
	options := map[string]interface{}{
		"password": password,
	}
	path := fmt.Sprintf("auth/userpass/login/%s", username)

	// PUT call to get a token
	secret, err := cl.Logical().Write(path, options)
	if err != nil {
		return "", err
	}

	token := secret.Auth.ClientToken
	return token, nil
}

func readSecret(ctx context.Context, cl *api.Client, v *types.VaultInfo) error {
	mounts, err := cl.Sys().ListMountsWithContext(ctx)
	if err != nil {
		return err
	}
	mount, path := v.SecretMountAndPath()
	var sec *api.KVSecret
	if m, ok := mounts[mount+"/"]; ok {
		switch vers := m.Options["version"]; vers {
		case "1":
			if sec, err = cl.KVv1(mount).Get(ctx, path); err != nil {
				return err
			}
		case "2":
			if sec, err = cl.KVv2(mount).Get(ctx, path); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported kv version %q", vers)
		}
	}

	if sec == nil {
		return fmt.Errorf("did not receive a valid secret with path %s", v.SecretPath)
	}

	if len(sec.Raw.Warnings) > 0 {
		return errors.New(strings.Join(sec.Raw.Warnings, ","))
	}

	extractUnsealKeys(sec.Data, v)
	return nil
}

func extractUnsealKeys(data interface{}, v *types.VaultInfo) {
	if m, o := data.(map[string]interface{}); o {
		for k, val := range m {
			if strings.HasPrefix(k, constants.KeyPrefixUnsealKey) {
				v.UnsealKeys = append(v.UnsealKeys, fmt.Sprintf("%v", val))
			}
		}
	}
}
