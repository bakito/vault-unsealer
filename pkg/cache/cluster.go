package cache

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/bakito/vault-unsealer/pkg/types"
	"github.com/hashicorp/serf/serf"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type clusteredCache struct {
	simpleCache
	serfCluster  *serf.Serf
	eventChannel chan serf.Event
}

// ----------------------------------------------------------------------------
// Serf
// ----------------------------------------------------------------------------

var serfLog = ctrl.Log.WithName("serf")

func NewClustered(myIP string, clusterMembers []string) (RunnableCache, error) {
	c := &clusteredCache{
		simpleCache:  simpleCache{vaults: make(map[string]*types.VaultInfo)},
		eventChannel: make(chan serf.Event, 256),
	}
	// Initialize or join Serf cluster.
	var err error
	c.serfCluster, err = setupSerfCluster(
		myIP,
		clusterMembers,
		c.eventChannel)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func setupSerfCluster(myIP string, clusterMembers []string, eventChannel chan<- serf.Event) (*serf.Serf, error) {
	// Configuration values.
	configuration := serf.DefaultConfig()
	configuration.Init()
	configuration.LogOutput = &logrWriter{}
	configuration.NodeName = myIP
	configuration.EnableNameConflictResolution = false
	configuration.UserEventSizeLimit = 1024

	configuration.MemberlistConfig.AdvertiseAddr = myIP
	configuration.MemberlistConfig.LogOutput = configuration.LogOutput
	configuration.MemberlistConfig.BindPort = 7946
	configuration.MemberlistConfig.AdvertisePort = configuration.MemberlistConfig.BindPort
	configuration.EventCh = eventChannel

	// Create the Serf cluster with the configuration.

	cluster, err := serf.Create(configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Couldn't create cluster")
	}

	// Try to join an existing Serf cluster.  If not, start a new cluster.
	if len(clusterMembers) > 0 {
		_, err = cluster.Join(clusterMembers, true)
		if err != nil {
			log.Info("starting new serf cluster")
		}
	}

	return cluster, nil
}

func (c *clusteredCache) Start() error {
	// Create a channel to receive Serf events.

	cancelChan := make(chan os.Signal, 1)
	// catch SIGETRM or SIGINTERRUPT
	signal.Notify(cancelChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		for event := range c.eventChannel {
			err := c.serfEventHandler(event)
			if err != nil {
				log.Error(err, "error handling event")
			}
		}
	}()
	sig := <-cancelChan
	log.WithValues("signal", sig).Info("Caught signal")

	return c.serfCluster.Leave()
}

// Handle any of the Serf event types.
func (c *clusteredCache) serfEventHandler(event serf.Event) error {
	switch event.EventType() {
	case serf.EventUser:
		ue := event.(serf.UserEvent)
		info := &types.VaultInfo{}
		if err := json.Unmarshal(ue.Payload, info); err != nil {
			return err
		}
		log.WithValues("owner", ue.Name, "keys", len(info.UnsealKeys)).Info("synced vault info from clustered cache")
		c.simpleCache.SetVaultInfoFor(ue.Name, info)
	case serf.EventMemberJoin:
		log.Info("syncing clustered cache with new member")
		for _, owner := range c.Owners() {
			info := c.VaultInfoFor(owner)
			if info.ShouldShare() {
				_ = c.serfCluster.UserEvent(owner, info.JSON(), true)
			}
		}
	}
	return nil
}

func (c *clusteredCache) SetVaultInfoFor(owner string, info *types.VaultInfo) {
	c.simpleCache.SetVaultInfoFor(owner, info)
	if info.ShouldShare() {
		_ = c.serfCluster.UserEvent(owner, info.JSON(), true)
	}
}

func FindMemberPodIPs(ctx context.Context, mgr manager.Manager, watchNamespace string, deploymentSelector map[string]string) (string, []string, error) {
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
