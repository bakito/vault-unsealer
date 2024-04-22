package cache

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/bakito/vault-unsealer/pkg/hierarchy"
	"github.com/bakito/vault-unsealer/pkg/types"
	"github.com/gin-gonic/gin"
	"gopkg.in/resty.v1"
)

// webPostSync handles the POST request to synchronize cache information for a specific stateful set.
func (c *k8sCache) webPostSync(ctx *gin.Context) {
	// Authenticate the request.
	if !c.handleAuth(ctx) {
		return
	}

	// Extract stateful set name from URL parameter.
	statefulSet := ctx.Param("statefulSet")
	info := &types.VaultInfo{}

	// Bind JSON payload to VaultInfo struct.
	err := ctx.ShouldBindJSON(info)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		log.WithValues("from", ctx.ClientIP(), "stateful-set", statefulSet).Error(err, "could not parse owner info")
		return
	}

	// Update cache with received VaultInfo.
	c.simpleCache.SetVaultInfoFor(statefulSet, info)
	log.WithValues(
		"from", ctx.ClientIP(),
		"stateful-set", fmt.Sprintf("%s (keys: %d)", statefulSet, len(info.UnsealKeys)),
	).Info("received vault info")
	ctx.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// webGetInfo handles the GET request to retrieve cache information.
func (c *k8sCache) webGetInfo(ctx *gin.Context) {
	// Log info request.
	log.WithValues("from", ctx.ClientIP(), "method", ctx.Request.Method, "vaults", c.vaultString()).Info("info requested")

	// Authenticate the request.
	token, ok := c.getAuthToken(ctx)
	if !ok {
		return
	}

	// Verify the client and check if it's a peer.
	peer, err := hierarchy.GetPeers(ctx, c.reader)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		log.WithValues("ip", ctx.ClientIP()).Error(err, "could not verify client")
	}
	if _, ok := peer[ctx.ClientIP()]; !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": http.StatusUnauthorized})
		return
	}

	// Send cache information to the requesting peer.
	cl := resty.New().SetAuthToken(token)
	cl.SetTimeout(time.Second)
	resp, err := cl.R().SetBody(&info{Vaults: c.vaults, Token: c.token}).Put(fmt.Sprintf("http://%s:%d/info", ctx.ClientIP(), apiPort))

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		log.WithValues("ip", ctx.ClientIP()).Error(err, "could not send info")

	} else if resp.StatusCode() != http.StatusOK {
		err = errors.New("could not send info")
		log.WithValues("ip", ctx.ClientIP(), "status", resp.StatusCode()).Error(err, err.Error())
		ctx.JSON(resp.StatusCode(), gin.H{"error": err.Error()})
	}

	ctx.JSON(http.StatusOK, c.vaults)
}

// webPutInfo handles the PUT request to update cache information received from a peer.
func (c *k8sCache) webPutInfo(ctx *gin.Context) {
	// Authenticate the request.
	token, ok := c.getAuthToken(ctx)
	if !ok {
		return
	}

	// Verify peer token.
	if token != c.peerToken {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": http.StatusUnauthorized})
		return
	}

	// Parse JSON payload.
	i := &info{}
	err := ctx.ShouldBindJSON(i)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		log.WithValues("from", ctx.ClientIP()).Error(err, "could parse info")
		return
	}

	// Update cache with received information.
	c.vaults = i.Vaults
	c.token = i.Token
	if c.client != nil {
		c.client.Token = i.Token
	}
	log.WithValues("from", ctx.ClientIP(), "method", ctx.Request.Method, "vaults", c.vaultString()).
		Info("received info from peer")
	ctx.JSON(http.StatusOK, c.vaults)
}
