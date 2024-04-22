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

// apiPort is the port for the cache API server.
const apiPort = 8866

var (
	log  = ctrl.Log.WithName("cache") // Logger for the cache package.
	once sync.Once                    // Ensures that certain operations are performed only once.

	_ manager.Runnable               = &k8sCache{} // Ensure that k8sCache implements the Runnable interface.
	_ manager.LeaderElectionRunnable = &k8sCache{} // Ensure that k8sCache implements the LeaderElectionRunnable interface.
)

// k8sCache implements the RunnableCache interface for managing Vault information cache in a Kubernetes cluster.
type k8sCache struct {
	simpleCache               // Embedding simpleCache to inherit its methods and fields.
	reader      client.Reader // Kubernetes client reader for interacting with the cluster.
	// clusterMembers is a map of cache members where key is IP address and value is name.
	clusterMembers map[string]string
	token          string        // Token for authentication.
	peerToken      string        // Token for peer communication.
	client         *resty.Client // HTTP client for communication with peers.
}

// NewK8s creates a new Kubernetes cache instance.
func NewK8s(reader client.Reader) (RunnableCache, error) {
	c := &k8sCache{
		simpleCache:    simpleCache{vaults: make(map[string]*types.VaultInfo)},
		reader:         reader,
		clusterMembers: map[string]string{},
	}

	return c, nil
}

// SetupWithManager sets up the Kubernetes cache with the provided manager.
func (c *k8sCache) SetupWithManager(mgr ctrl.Manager) error {
	go func() {
		// Block until our controller manager is elected leader.
		<-mgr.Elected()

		// Ask peers if we do not have vaults yet.
		if len(c.vaults) == 0 || len(c.token) == 0 {
			if err := c.AskPeers(context.Background()); err != nil {
				log.Error(err, "error asking peers")
			}
		}
	}()
	return mgr.Add(c)
}

// NeedLeaderElection indicates whether leader election is needed for the cache.
func (c *k8sCache) NeedLeaderElection() bool {
	return false
}

// Start starts the cache API server and handles incoming requests.
func (c *k8sCache) Start(ctx context.Context) error {
	log.Info("starting shared cache")
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.POST("/sync/:statefulSet", c.webPostSync)
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

// SetVaultInfoFor sets the Vault information for the specified stateful set.
// If the Vault instance should share its information with peers, it sends the information to all peers.
func (c *k8sCache) SetVaultInfoFor(statefulSet string, info *types.VaultInfo) {
	c.simpleCache.SetVaultInfoFor(statefulSet, info)
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
			resp, err := c.client.R().SetBody(info).Post(fmt.Sprintf("http://%s:%d/sync/%s", ip, apiPort, statefulSet))
			if err != nil {
				log.WithValues("pod", name, "stateful-set", statefulSet).Error(err, "could not send owner info")
			} else if resp.StatusCode() != http.StatusOK {
				log.WithValues("pod", name, "stateful-set", statefulSet, "status", resp.StatusCode()).
					Error(errors.New("could not send owner info"), "could not send owner info")
			}
		}
	}
}

// handleAuth handles the authentication for incoming requests.
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

// getAuthToken extracts the authentication token from the request headers.
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

// SetMember updates the cluster members and returns true if the members are updated.
func (c *k8sCache) SetMember(members map[string]string) bool {
	if maps.Equal(members, c.clusterMembers) {
		return false
	}

	c.clusterMembers = members
	return true
}

// Sync synchronizes the cache with the peers.
func (c *k8sCache) Sync() {
	for _, statefulSet := range c.Vaults() {
		c.SetVaultInfoFor(statefulSet, c.VaultInfoFor(statefulSet))
	}
}

// vaultString returns a sorted list of stateful sets with their respective number of keys.
func (c *k8sCache) vaultString() (keys []string) {
	for k, i := range c.vaults {
		keys = append(keys, fmt.Sprintf("%s (keys: %d)", k, len(i.UnsealKeys)))
	}
	sort.Strings(keys)
	return
}
