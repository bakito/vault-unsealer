package cache

import (
	"context"
	"os"
	"os/signal"
	"strings"
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
	serfCluster *serf.Serf
}

// ----------------------------------------------------------------------------
// Serf
// ----------------------------------------------------------------------------

var serfLog = ctrl.Log.WithName("serf")

type logrWriter struct{}

func (w *logrWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSuffix(string(p), "\n")
	if strings.Contains(msg, "[DEBUG] ") {
		serfLog.V(2).Info(strings.Split(msg, "[DEBUG] ")[1])
	} else if strings.Contains(msg, "[INFO] ") {
		serfLog.Info(strings.Split(msg, "[INFO] ")[1])
	} else if strings.Contains(msg, "[ERROR] ") {
		serfLog.Error(nil, strings.Split(msg, "[ERROR] ")[1])
	} else if strings.Contains(msg, "[WARN] ") {
		serfLog.Error(nil, strings.Split(msg, "[WARN] ")[1])
	}
	return 0, nil
}

func setupSerfCluster(myIP string, clusterMembers []string, eventChannel chan<- serf.Event) (*serf.Serf, error) {
	// Configuration values.
	configuration := serf.DefaultConfig()
	configuration.Init()
	configuration.LogOutput = &logrWriter{}
	configuration.NodeName = myIP

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
			log.Error(err, "Couldn't join cluster, starting own")
		}
	}

	return cluster, nil
}

func (c clusteredCache) Start(myIP string, clusterMembers []string) error {
	// Create a channel to receive Serf events.

	eventChannel := make(chan serf.Event, 256)

	// Initialize or join Serf cluster.
	var err error
	c.serfCluster, err = setupSerfCluster(
		myIP,
		clusterMembers,
		eventChannel)
	if err != nil {
		return err
	}

	cancelChan := make(chan os.Signal, 1)
	// catch SIGETRM or SIGINTERRUPT
	signal.Notify(cancelChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		for event := range eventChannel {
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
func (c clusteredCache) serfEventHandler(event serf.Event) error {
	if event.EventType() == serf.EventUser {
		ue := event.(serf.UserEvent)
		info := &types.VaultInfo{}
		if err := json.Unmarshal(ue.Payload, info); err != nil {
			return err
		}
		log.WithValues("owner", ue.Name).Info("received vault info from clustered cache")
		c.simpleCache.SetVaultInfoFor(ue.Name, info)
	}
	return nil
}

func (c clusteredCache) SetVaultInfoFor(owner string, info *types.VaultInfo) {
	c.simpleCache.SetVaultInfoFor(owner, info)
	b, _ := json.Marshal(info)
	_ = c.serfCluster.UserEvent("owner", b, false)
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
