package cache

import (
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	la "github.com/bakito/go-log-logr-adapter/adapter"
	"github.com/hashicorp/serf/serf"
	"github.com/pkg/errors"
)

type clusteredCache struct {
	simpleCache
}

// ----------------------------------------------------------------------------
// Serf
// ----------------------------------------------------------------------------

// Setup the Serf Cluster
func setupSerfCluster(advertiseAddr string, clusterAddr string, eventChannel chan<- serf.Event) (*serf.Serf, error) {
	// Configuration values.
	configuration := serf.DefaultConfig()
	configuration.Init()
	configuration.Logger = la.ToStd(log)
	configuration.NodeName = advertiseAddr

	addrPort := strings.Split(advertiseAddr, ":")
	configuration.MemberlistConfig.AdvertiseAddr = addrPort[0]
	configuration.MemberlistConfig.Logger = configuration.Logger
	if len(addrPort) > 1 {
		p, err := strconv.Atoi(addrPort[1])
		if err != nil {
			return nil, err
		}
		configuration.MemberlistConfig.BindPort = p
		configuration.MemberlistConfig.AdvertisePort = p
	}
	configuration.EventCh = eventChannel

	// Create the Serf cluster with the configuration.

	cluster, err := serf.Create(configuration)
	if err != nil {
		return nil, errors.Wrap(err, "Couldn't create cluster")
	}

	// Try to join an existing Serf cluster.  If not, start a new cluster.
	if len(clusterAddr) > 0 {
		_, err = cluster.Join(strings.Split(clusterAddr, ","), true)
		if err != nil {
			log.Error(err, "Couldn't join cluster, starting own")
		}
	}

	return cluster, nil
}

// Get a list of members in the cluster.
//func getClusterMembers(cluster *serf.Serf) []serf.Member {
//	var result []serf.Member
//
//	// Get all members in all states.
//
//	members := cluster.Members()
//
//	// Filter list. Don't add this instance nor failed instances.
//
//	for _, member := range members {
//		if member.Name != cluster.LocalMember().Name && member.Status == serf.StatusAlive {
//			result = append(result, member)
//		}
//	}
//	return result
//}

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

func (c clusteredCache) Start() error {
	// Create a channel to receive Serf events.

	eventChannel := make(chan serf.Event, 256)

	// Initialize or join Serf cluster.

	serfCluster, err := setupSerfCluster(
		os.Getenv("ADVERTISE_ADDR"),
		os.Getenv("CLUSTER_ADDR"),
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
