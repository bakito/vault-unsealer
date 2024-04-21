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

type info struct {
	Vaults map[string]*types.VaultInfo `json:"vaults"`
	Token  string                      `json:"token"`
}

// LeaderElectionRunnable implementation

func (c *k8sCache) AskPeers(ctx context.Context) error {
	peers, err := hierarchy.GetPeers(ctx, c.reader)
	if err != nil {
		return err
	}
	if len(peers) == 0 {
		return nil
	}
	c.peerToken = uuid.NewString()
	defer func() { c.peerToken = "" }()

	cl := resty.New().SetAuthToken(c.peerToken)
	cl.SetTimeout(time.Second)

	for ip, name := range peers {
		l := log.WithValues("name", name, "ip", ip)
		l.Info("requesting cache info from peer")
		resp, err := cl.R().Get(fmt.Sprintf("http://%s:%d/info", ip, apiPort))

		if err != nil {
			l.Error(err, "could request info")
		} else if resp.StatusCode() != http.StatusOK {
			l.WithValues("status", resp.StatusCode()).Error(err, "could request info")
		}
		return nil
	}
	return nil
}
