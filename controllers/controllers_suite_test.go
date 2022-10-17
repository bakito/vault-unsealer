package controllers

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/hashicorp/vault/api"
	"github.com/hashicorp/vault/helper/namespace"
	"github.com/hashicorp/vault/http"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/hashicorp/vault/vault"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var testingT *testing.T

func TestControllers(t *testing.T) {
	testingT = t
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers Suite")
}

func createTestVault(version int, secretPath string, data map[string]interface{}) (net.Listener, *api.Client) {
	// Create an in-memory, unsealed core (the "backend", if you will).
	core, keyShares, rootToken := vault.TestCoreUnsealed(testingT)
	_ = keyShares

	// Start an HTTP server for the core.
	ln, addr := http.TestServer(testingT, core)

	// Create a client that talks to the server, initially authenticating with
	// the root token.
	conf := api.DefaultConfig()
	conf.Address = addr

	client, err := api.NewClient(conf)
	Ω(err).ShouldNot(HaveOccurred())

	client.SetToken(rootToken)

	_, err = client.Logical().Delete("sys/mounts/secret")
	Ω(err).ShouldNot(HaveOccurred())

	kvReq := &logical.Request{
		Operation:   logical.UpdateOperation,
		ClientToken: rootToken,
		Path:        "sys/mounts/secret",
		Data: map[string]interface{}{
			"type":        "kv",
			"path":        "secret/",
			"description": fmt.Sprintf("key/value secret storage v%d", version),
			"options": map[string]string{
				"version": fmt.Sprintf("%d", version),
			},
		},
	}
	resp, err := core.HandleRequest(namespace.RootContext(context.TODO()), kvReq)
	Ω(err).ShouldNot(HaveOccurred())
	Ω(resp.IsError()).Should(BeFalse())

	if version == 2 {
		_, err = client.Logical().Write(secretPath, map[string]interface{}{
			"data": data,
		})
	} else {
		_, err = client.Logical().Write(secretPath, data)
	}
	Ω(err).ShouldNot(HaveOccurred())

	return ln, client
}
