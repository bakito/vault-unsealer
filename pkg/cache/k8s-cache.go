package cache

import (
	"context"
	"github.com/bakito/vault-unsealer/pkg/types"
	"github.com/gin-gonic/gin"
	"gopkg.in/resty.v1"
	corev1 "k8s.io/api/core/v1"
	"net/http"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type k8sCache struct {
	simpleCache
	reader         client.Reader
	clusterMembers []string
}

func NewK8s(reader client.Reader, clusterMembers []string) (RunnableCache, error) {
	return &k8sCache{
		simpleCache:    simpleCache{vaults: make(map[string]*types.VaultInfo)},
		reader:         reader,
		clusterMembers: clusterMembers,
	}, nil
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

func FindMemberPodIPs(ctx context.Context, mgr manager.Manager, watchNamespace string,
	deploymentSelector map[string]string) (string, []string, error) {
	pods := &corev1.PodList{}
	if err := mgr.GetAPIReader().List(
		ctx,
		pods,
		client.MatchingLabels(deploymentSelector),
		client.InNamespace(watchNamespace),
	); err != nil {
		return "", nil, err
	}

	hostName := os.Getenv("HOSTNAME")

	var myIP string
	var members []string
	for _, pod := range pods.Items {
		if pod.Name == hostName {
			myIP = pod.Status.PodIP
		} else if pod.Status.Phase == corev1.PodRunning {
			members = append(members, pod.Status.PodIP)
		}
	}

	return myIP, members, nil
}
