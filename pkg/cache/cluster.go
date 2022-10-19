package cache

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	la "github.com/bakito/go-log-logr-adapter/adapter"
	"github.com/hashicorp/serf/serf"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type clusteredCache struct {
	simpleCache
}

// ----------------------------------------------------------------------------
// Serf
// ----------------------------------------------------------------------------

func setupSerfCluster(myIP string, clusterMembers []string, eventChannel chan<- serf.Event) (*serf.Serf, error) {
	// Configuration values.
	configuration := serf.DefaultConfig()
	configuration.Init()
	configuration.Logger = la.ToStd(log)
	configuration.NodeName = myIP

	configuration.MemberlistConfig.AdvertiseAddr = myIP
	configuration.MemberlistConfig.Logger = configuration.Logger
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

// Get a list of members in the cluster.
//
//nolint:unused
func getClusterMembers(cluster *serf.Serf) []serf.Member {
	var result []serf.Member

	// Get all members in all states.

	members := cluster.Members()

	// Filter list. Don't add this instance nor failed instances.

	for _, member := range members {
		if member.Name != cluster.LocalMember().Name && member.Status == serf.StatusAlive {
			result = append(result, member)
		}
	}
	return result
}

// Example query responses.
func queryResponse(event serf.Event) {
	result := ""
	query := event.String()
	responder := event.(*serf.Query)
	switch query {
	case "query: bob":
		result = "Bob was here"
	case "query: mary":
		result = "Mary was here"
	case "query: time":
		result = time.Now().String()
	}
	_ = responder.Respond([]byte(result))
}

// Handle any of the Serf event types.
func serfEventHandler(event serf.Event) {
	l := log.WithValues("event", event.String())
	switch event.EventType() {
	case serf.EventMemberFailed:
		l.Info("EventMemberFailed")
	case serf.EventMemberJoin:
		l.Info("EventMemberJoin")
	case serf.EventMemberLeave:
		l.Info("EventMemberLeave")
	case serf.EventMemberReap:
		l.Info("EventMemberReap")
	case serf.EventMemberUpdate:
		l.Info("EventMemberUpdate")
	case serf.EventQuery:
		l.Info("EventQuery")
		queryResponse(event)
	case serf.EventUser:
		l.Info("EventUser")
	default:
		l.Info("[WARN] on: Unhandled Serf Event")
	}
}

func (c clusteredCache) Start(myIP string, clusterMembers []string) error {
	// Create a channel to receive Serf events.

	eventChannel := make(chan serf.Event, 256)

	// Initialize or join Serf cluster.

	serfCluster, err := setupSerfCluster(
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
			serfEventHandler(event)
		}
	}()
	sig := <-cancelChan
	log.WithValues("signal", sig).Info("Caught signal")

	return serfCluster.Leave()
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
