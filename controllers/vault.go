package controllers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/bakito/vault-unsealer/pkg/constants"
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

func readSecret(cl *api.Client, v *vaultInfo) error {
	sec, err := cl.Logical().Read(v.secretPath)
	if err != nil {
		return err
	}

	if sec == nil {
		return fmt.Errorf("did not receive a valid secret with path %s", v.secretPath)
	}

	if len(sec.Warnings) > 0 {
		return errors.New(strings.Join(sec.Warnings, ","))
	}

	if data, ok := sec.Data["data"]; ok {
		extractUnsealKeys(data, v)
	} else {
		extractUnsealKeys(sec.Data, v)
	}
	return nil
}

func extractUnsealKeys(data interface{}, v *vaultInfo) {
	if m, o := data.(map[string]interface{}); o {
		for k, val := range m {
			if strings.HasPrefix(k, constants.KeyPrefixUnsealKey) {
				v.unsealKeys = append(v.unsealKeys, fmt.Sprintf("%v", val))
			}
		}
	}
}
