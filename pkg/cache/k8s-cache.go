package cache

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bakito/vault-unsealer/pkg/constants"
	"github.com/bakito/vault-unsealer/pkg/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gopkg.in/resty.v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	log  = ctrl.Log.WithName("cache")
	once sync.Once
)

type k8sCache struct {
	simpleCache
	reader         client.Reader
	clusterMembers map[string]bool
	token          string
	client         *resty.Client
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

		once.Do(func() {
			if c.token == "" {
				c.token = uuid.NewString()
			}
			c.client = resty.New().SetAuthToken(c.token)
		})

		c.client.SetTimeout(time.Second)
		for member := range c.clusterMembers {
			if strings.EqualFold(os.Getenv(constants.EnvDevelopmentMode), "true") {
				member = "localhost"
			}
			_, err := c.client.R().SetBody(info).Post(fmt.Sprintf("http://%s:8866/sync/%s", member, owner))
			if err != nil {
				log.WithValues("member", member, "owner", owner).Error(err, "could not send owner info")
			}
		}
	}
}

func (c *k8sCache) Start(_ context.Context) error {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.POST("/sync/:owner", func(ctx *gin.Context) {
		if !c.handleAuth(ctx) {
			return
		}

		owner := ctx.Param("owner")
		info := &types.VaultInfo{}
		err := ctx.ShouldBindJSON(info)
		if err != nil {
			ctx.JSON(http.StatusOK, gin.H{
				"error": err.Error(),
			})
			log.WithValues("owner", owner).Error(err, "could parse owner info")
			return
		}
		c.simpleCache.SetVaultInfoFor(owner, info)
		ctx.JSON(http.StatusOK, gin.H{
			"message": "ok",
		})
	})
	return r.Run(":8866")
}

func (c *k8sCache) handleAuth(ctx *gin.Context) bool {
	auth := ctx.GetHeader("Authorization")
	if auth == "" {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": http.StatusUnauthorized})
		return false
	}

	t := strings.Split(auth, "Bearer ")
	if len(t) != 2 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": http.StatusUnauthorized})
		return false
	}
	if c.token != "" {
		if c.token != t[1] {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": http.StatusUnauthorized})
			return false
		}
	} else {
		c.token = t[1]
	}
	return true
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
