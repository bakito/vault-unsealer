package cache

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"sort"
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
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const apiPort = 8866

var (
	log  = ctrl.Log.WithName("cache")
	once sync.Once

	_ manager.Runnable               = &k8sCache{}
	_ manager.LeaderElectionRunnable = &k8sCache{}
)

type k8sCache struct {
	simpleCache
	reader client.Reader
	// clusterMembers map of cache members / key: ip value: name
	clusterMembers map[string]string
	token          string
	peerToken      string
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

func (c *k8sCache) SetupWithManager(mgr ctrl.Manager) error {
	go func() {
		// Block until our controller manager is elected leader. We presume our
		// entire process will terminate if we lose leadership, so we don't need
		// to handle that.
		<-mgr.Elected()

		// ask peers if we do not have vaults yet
		if len(c.vaults) == 0 || len(c.token) == 0 {
			if err := c.AskPeers(context.Background()); err != nil {
				log.Error(err, "error asking peers")
			}
		}
	}()
	return mgr.Add(c)
}

func (c *k8sCache) NeedLeaderElection() bool {
	return false
}

func (c *k8sCache) Start(ctx context.Context) error {
	log.Info("starting shared cache")
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.POST("/sync/:vaultName", c.webPostSync)
	r.GET("/info", c.webGetInfo)
	r.PUT("/info", c.webPutInfo)

	// Start the server in a separate goroutine
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", apiPort),
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	serverShutdown := make(chan struct{})
	go func() {
		<-ctx.Done()
		log.Info("shutting down cache server")
		if err := server.Shutdown(context.Background()); err != nil {
			log.Error(err, "error shutting down cache server")
		}
		close(serverShutdown)
	}()

	log.Info("starting cache server")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	<-serverShutdown
	return nil
}

func (c *k8sCache) SetVaultInfoFor(vaultName string, info *types.VaultInfo) {
	c.simpleCache.SetVaultInfoFor(vaultName, info)
	if info.ShouldShare() {
		for ip, name := range c.clusterMembers {
			once.Do(func() {
				if c.token == "" {
					c.token = uuid.NewString()
				}
				c.client = resty.New().SetAuthToken(c.token)
				c.client.SetTimeout(time.Second)
			})
			if constants.IsDevMode() {
				ip = "localhost"
			}
			resp, err := c.client.R().SetBody(info).Post(fmt.Sprintf("http://%s:%d/sync/%s", ip, apiPort, vaultName))
			if err != nil {
				log.WithValues("pod", name, "vault", vaultName).Error(err, "could not send owner info")
			} else if resp.StatusCode() != http.StatusOK {
				log.WithValues("pod", name, "vault", vaultName, "status", resp.StatusCode()).
					Error(errors.New("could not send owner info"), "could not send owner info")
			}
		}
	}
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

func (c *k8sCache) SetMember(members map[string]string) bool {
	if maps.Equal(members, c.clusterMembers) {
		return false
	}

	c.clusterMembers = members
	return true
}

func (c *k8sCache) Sync() {
	for _, vaultName := range c.Vaults() {
		c.SetVaultInfoFor(vaultName, c.VaultInfoFor(vaultName))
	}
}

func (c *k8sCache) vaultString() (keys []string) {
	for k, i := range c.vaults {
		keys = append(keys, fmt.Sprintf("%s (keys: %d)", k, len(i.UnsealKeys)))
	}
	sort.Strings(keys)
	return
}
