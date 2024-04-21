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

func (c *k8sCache) webPostSync(ctx *gin.Context) {
	if !c.handleAuth(ctx) {
		return
	}

	vaultName := ctx.Param("vaultName")
	info := &types.VaultInfo{}
	err := ctx.ShouldBindJSON(info)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		log.WithValues("from", ctx.ClientIP(), "vault", vaultName).Error(err, "could not parse owner info")
		return
	}
	c.simpleCache.SetVaultInfoFor(vaultName, info)
	log.WithValues(
		"from", ctx.ClientIP(),
		"vault", fmt.Sprintf("%s (keys: %d)", vaultName, len(info.UnsealKeys)),
	).Info("received vault info")
	ctx.JSON(http.StatusOK, gin.H{"message": "ok"})
}

func (c *k8sCache) webGetInfo(ctx *gin.Context) {
	log.WithValues("from", ctx.ClientIP(), "method", ctx.Request.Method, "vaults", c.vaultString()).Info("info requested")
	token, ok := c.getAuthToken(ctx)
	if !ok {
		return
	}

	peer, err := hierarchy.GetPeers(ctx, c.reader)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		log.WithValues("ip", ctx.ClientIP()).Error(err, "could not verify client")
	}
	if _, ok := peer[ctx.ClientIP()]; !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": http.StatusUnauthorized})
		return
	}

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

func (c *k8sCache) webPutInfo(ctx *gin.Context) {
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
	c.vaults = i.Vaults
	c.token = i.Token
	if c.client != nil {
		c.client.Token = i.Token
	}
	log.WithValues("from", ctx.ClientIP(), "method", ctx.Request.Method, "vaults", c.vaultString()).
		Info("received info from peer")
	ctx.JSON(http.StatusOK, c.vaults)
}
