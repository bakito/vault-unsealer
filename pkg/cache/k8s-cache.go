package cache

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bakito/vault-unsealer/pkg/constants"
	"github.com/bakito/vault-unsealer/pkg/hierarchy"
	"github.com/bakito/vault-unsealer/pkg/types"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gopkg.in/resty.v1"
	appsv1 "k8s.io/api/apps/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	log  = ctrl.Log.WithName("cache")
	once sync.Once
)

type info struct {
	vaults map[string]*types.VaultInfo
	token  string
}

type k8sCache struct {
	simpleCache
	mu             sync.Mutex
	reader         client.Reader
	clusterMembers map[string]string
	token          string
	peerToken      string
	client         *resty.Client
	depl           *appsv1.Deployment
}

func NewK8s(reader client.Reader, depl *appsv1.Deployment) (RunnableCache, manager.Runnable, error) {
	c := &k8sCache{
		simpleCache:    simpleCache{vaults: make(map[string]*types.VaultInfo)},
		reader:         reader,
		clusterMembers: map[string]string{},
		depl:           depl,
	}

	return c, c, nil
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
					Error(errors.New("could not send owner info"), "could not send owner info")
			}
		}
	}
}

func (c *k8sCache) StartCache(_ context.Context) error {
	log.Info("starting shared cache")
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
			log.WithValues("from", ctx.ClientIP(), "owner", owner).Error(err, "could not parse owner info")
			return
		}
		c.simpleCache.SetVaultInfoFor(owner, info)
		log.WithValues("from", ctx.ClientIP(), "owner", owner).Info("received vault info")
		ctx.JSON(http.StatusOK, gin.H{
			"message": "ok",
		})
	})
	r.GET("/info", func(ctx *gin.Context) {
		log.WithValues("from", ctx.ClientIP(), "method", ctx.Request.Method).Info("INFO")
		token, ok := c.getAuthToken(ctx)
		if !ok {
			return
		}

		pods, err := hierarchy.GetOwnedPod(ctx, c.reader, c.depl)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			log.WithValues("ip", ctx.ClientIP()).Error(err, "could not verify client")
		}
		if _, ok := pods[ctx.ClientIP()]; !ok {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": http.StatusUnauthorized})
			return
		}

		cl := resty.New().SetAuthToken(token)
		cl.SetTimeout(time.Second)
		log.WithValues("token", c.token, "vaults", c.vaults).Info("########################")
		resp, err := cl.R().SetBody(&info{vaults: c.vaults, token: c.token}).Put(fmt.Sprintf("http://%s:8866/info", ctx.ClientIP()))

		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			log.WithValues("ip", ctx.ClientIP()).Error(err, "could send info")

		} else if resp.StatusCode() != http.StatusOK {
			err = errors.New("could send info")
			log.WithValues("ip", ctx.ClientIP(), "status", resp.StatusCode()).Error(err, err.Error())
			ctx.JSON(resp.StatusCode(), gin.H{"error": err.Error()})
		}

		ctx.JSON(http.StatusOK, c.vaults)
	})
	r.PUT("/info", func(ctx *gin.Context) {
		token, ok := c.getAuthToken(ctx)
		if !ok {
			return
		}

		if token != c.peerToken {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": http.StatusUnauthorized})
			return
		}

		i := &info{}
		err := ctx.ShouldBindJSON(i)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			log.WithValues("from", ctx.ClientIP()).Error(err, "could parse info")
			return
		}
		c.vaults = i.vaults
		c.token = i.token
		if c.client != nil {
			c.client.Token = i.token
		}
		log.WithValues("from", ctx.ClientIP(), "method", ctx.Request.Method, "vaults", len(c.vaults)).
			Info("received info from peer")
		ctx.JSON(http.StatusOK, c.vaults)
	})
	return r.Run(":8866")
}

func (c *k8sCache) handleAuth(ctx *gin.Context) bool {
	token, ok := c.getAuthToken(ctx)
	if !ok {
		return false
	}
	if c.token != "" {
		if c.token != token {
			ctx.JSON(http.StatusUnauthorized, gin.H{"error": http.StatusUnauthorized})
			return false
		}
	} else {
		c.token = token
	}
	return true
}

func (c *k8sCache) getAuthToken(ctx *gin.Context) (string, bool) {
	auth := ctx.GetHeader("Authorization")
	if auth == "" {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": http.StatusUnauthorized})
		return "", false
	}

	t := strings.Split(auth, "Bearer ")
	if len(t) != 2 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": http.StatusUnauthorized})
		return "", false
	}
	return t[1], true
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

// LeaderElectionRunnable implementation

func (c *k8sCache) Start(ctx context.Context) error {
	return c.AskPeers(ctx)
}

func (c *k8sCache) NeedLeaderElection() bool {
	return true
}

func (c *k8sCache) AskPeers(ctx context.Context) error {
	pods, err := hierarchy.GetOwnedPod(ctx, c.reader, c.depl)
	if err != nil {
		return err
	}
	if len(pods) == 0 {
		return nil
	}
	c.peerToken = uuid.NewString()
	defer func() { c.peerToken = "" }()

	cl := resty.New().SetAuthToken(c.peerToken)
	cl.SetTimeout(time.Second)

	for ip, pod := range pods {
		if pod.Name != os.Getenv(constants.EnvPodName) {
			log.WithValues("pod", pod.Name, "ip", ip).Info("requesting cache info from peer")
			resp, err := cl.R().Get(fmt.Sprintf("http://%s:8866/info", ip))

			if err != nil {
				log.WithValues("pod", pod.Name, "ip", ip).Error(err, "could request info")
			} else if resp.StatusCode() != http.StatusOK {
				log.WithValues("pod", pod.Name, "ip", ip, "status", resp.StatusCode()).Error(err, "could request info")
			}
		}

		return nil
	}
	return nil
}
