package cache

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bakito/vault-unsealer/pkg/constants"
	"github.com/bakito/vault-unsealer/pkg/types"
	"github.com/gin-gonic/gin"
	"gopkg.in/resty.v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type k8sCache struct {
	simpleCache
	reader         client.Reader
	clusterMembers map[string]bool
}

func NewK8s(reader client.Reader) (RunnableCache, error) {
	c := &k8sCache{
		simpleCache:    simpleCache{vaults: make(map[string]*types.VaultInfo)},
		reader:         reader,
		clusterMembers: map[string]bool{},
	}
	return c, nil
}

func (c *k8sCache) SetVaultInfoFor(owner string, info *types.VaultInfo) {
	c.simpleCache.SetVaultInfoFor(owner, info)
	if info.ShouldShare() {
		client := resty.New()
		client.SetTimeout(time.Second)
		for member := range c.clusterMembers {
			if strings.EqualFold(os.Getenv(constants.EnvDevelopmentMode), "true") {
				member = "localhost"
			}
			_, err := client.R().SetBody(info).Post(fmt.Sprintf("http://%s:8866/sync/%s", member, owner))
			if err != nil {
				println(err.Error())
			}
		}
	}
}

func (c *k8sCache) Start(_ context.Context) error {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.POST("/sync/:owner", func(ctx *gin.Context) {
		owner := ctx.Param("owner")
		info := &types.VaultInfo{}
		err := ctx.ShouldBindJSON(info)
		if err != nil {
			ctx.JSON(http.StatusOK, gin.H{
				"error": err.Error(),
			})
			return
		}
		c.simpleCache.SetVaultInfoFor(owner, info)
		ctx.JSON(http.StatusOK, gin.H{
			"message": "ok",
		})
	})
	return r.Run(":8866")
}

func (c *k8sCache) AddMember(ip string) {
	c.clusterMembers[ip] = true
}

func (c *k8sCache) RemoveMember(ip string) {
	delete(c.clusterMembers, ip)
}

func (c *k8sCache) Sync() {
	for _, owner := range c.Owners() {
		c.SetVaultInfoFor(owner, c.VaultInfoFor(owner))
	}
}
