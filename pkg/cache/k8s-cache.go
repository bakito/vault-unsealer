package cache

import (
	"context"
	"net/http"

	"github.com/bakito/vault-unsealer/pkg/types"
	"github.com/gin-gonic/gin"
	"gopkg.in/resty.v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type k8sCache struct {
	simpleCache
	reader         client.Reader
	clusterMembers []string
}

func NewK8s(reader client.Reader) (RunnableCache, error) {
	c := &k8sCache{
		simpleCache: simpleCache{vaults: make(map[string]*types.VaultInfo)},
		reader:      reader,
	}
	return c, nil
}

func (c *k8sCache) SetVaultInfoFor(owner string, info *types.VaultInfo) {
	c.simpleCache.SetVaultInfoFor(owner, info)
	if info.ShouldShare() {
		client := resty.New()
		for _, member := range c.clusterMembers {
			client.R().SetBody(info).Post(member + ":8866/sync")
		}
	}
}

func (c *k8sCache) Start(_ context.Context) error {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.POST("/sync", func(c *gin.Context) {
		info := &types.VaultInfo{}
		err := c.ShouldBindJSON(info)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "ok",
		})
	})
	return r.Run(":8866")
}
