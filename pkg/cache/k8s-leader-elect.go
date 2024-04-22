package cache

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/bakito/vault-unsealer/pkg/hierarchy"
	"github.com/bakito/vault-unsealer/pkg/types"
	"github.com/google/uuid"
	"gopkg.in/resty.v1"
)

// info is a struct representing the cache information to be exchanged between peers.
type info struct {
	Vaults map[string]*types.VaultInfo `json:"vaults"` // Vaults contains the Vault information.
	Token  string                      `json:"token"`  // Token is the authentication token for peers.
}

// AskPeers requests cache information from peer nodes and updates the cache with the received information.
func (c *k8sCache) AskPeers(ctx context.Context) error {
	// Get the list of peers from the hierarchy package.
	peers, err := hierarchy.GetPeers(ctx, c.reader)
	if err != nil {
		return err
	}
	if len(peers) == 0 {
		return nil
	}

	// Generate a unique token for peer communication.
	c.peerToken = uuid.NewString()
	defer func() { c.peerToken = "" }()

	// Create a REST client for communicating with peers.
	cl := resty.New().SetAuthToken(c.peerToken)
	cl.SetTimeout(time.Second)

	// Iterate over each peer and request cache information.
	for ip, name := range peers {
		l := log.WithValues("name", name, "ip", ip)
		l.Info("requesting cache info from peer")
		resp, err := cl.R().Get(fmt.Sprintf("http://%s:%d/info", ip, apiPort))

		if err != nil {
			l.Error(err, "could request info")
		} else if resp.StatusCode() != http.StatusOK {
			l.WithValues("status", resp.StatusCode()).Error(err, "could request info")
		}
		return nil // Exit after requesting info from the first peer.
	}
	return nil
}
