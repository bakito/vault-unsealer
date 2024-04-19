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
	mu             sync.Mutex
	reader         client.Reader
	clusterMembers map[string]string
	token          string
	client         *resty.Client
}

func NewK8s(reader client.Reader) (RunnableCache, error) {
	c := &k8sCache{
		simpleCache:    simpleCache{vaults: make(map[string]*types.VaultInfo)},
		reader:         reader,
		clusterMembers: map[string]string{},
	}
	return c, nil
}

func (c *k8sCache) SetVaultInfoFor(owner string, info *types.VaultInfo) {
	c.simpleCache.SetVaultInfoFor(owner, info)
	if info.ShouldShare() {
		for ip, name := range c.clusterMembers {
			once.Do(func() {
				if c.token == "" {
					c.token = uuid.NewString()
				}
				c.client = resty.New().SetAuthToken(c.token)
				c.client.SetTimeout(time.Second)
			})
			if strings.EqualFold(os.Getenv(constants.EnvDevelopmentMode), "true") {
				ip = "localhost"
			}
			resp, err := c.client.R().SetBody(info).Post(fmt.Sprintf("http://%s:8866/sync/%s", ip, owner))
			if err != nil {
				log.WithValues("pod", name, "owner", owner).Error(err, "could not send owner info")
			} else if resp.StatusCode() != http.StatusOK {
				log.WithValues("pod", name, "owner", owner, "status", resp.StatusCode()).
					Error(err, "could not send owner info")
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
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			log.WithValues("owner", owner).Error(err, "could parse owner info")
			return
		}
		c.simpleCache.SetVaultInfoFor(owner, info)
		log.WithValues("owner", owner).Info("received vault info")
		ctx.JSON(http.StatusOK, gin.H{
			"message": "ok",
		})
	})
	r.GET("/info", func(ctx *gin.Context) {
		// TODO check if pod belongs to same deployment
		ctx.JSON(http.StatusOK, c.vaults)
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

func (c *k8sCache) AddMember(ip string, name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.clusterMembers[ip]; !ok {
		log.WithValues("name", name, "ip", ip).Info("adding pod to cache")
		c.clusterMembers[ip] = name
	}
}

func (c *k8sCache) RemoveMember(ip string, name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.clusterMembers[ip]; ok {
		log.WithValues("name", name, "ip", ip).Info("removing pod from cache")
		delete(c.clusterMembers, ip)
	}
}

func (c *k8sCache) Sync() {
	for _, owner := range c.Owners() {
		c.SetVaultInfoFor(owner, c.VaultInfoFor(owner))
	}
}
