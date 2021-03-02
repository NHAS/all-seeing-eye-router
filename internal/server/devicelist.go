package server

import (
	"fmt"
	"log"
	"net/rpc"
	"time"

	"github.com/NHAS/all-seeing-eye-router/internal/global"

	"github.com/jsimonetti/rtnetlink"
	"golang.org/x/sys/unix"
)

//Devices is an RPC structure
type Devices struct {
}

func getDevices(conn *rtnetlink.Conn) (neigh []global.NeighInfo, err error) {
	// Request all neighbors
	msg, err := conn.Neigh.List()
	if err != nil {
		return nil, err
	}

	// Filter neighbors by family and type
	for _, v := range msg {
		if v.Family == unix.AF_UNSPEC && v.Family != unix.AF_INET {
			continue
		}

		if v.Attributes.Address.IsLinkLocalMulticast() || v.Attributes.Address.IsUnspecified() || v.Attributes.Address.IsLoopback() {
			continue
		}

		neigh = append(neigh, global.NeighInfo{
			Ip:     v.Attributes.Address.String(),
			LLaddr: v.Attributes.LLAddress.String(),
			State:  v.State,
		})
	}

	return neigh, nil
}

//Result is used for RPC return
type Result struct {
	Neighbours []global.NeighInfo
}

//Get is an RPC method called by client to return the current list of device neighbors. Not currently in use as client hasnt implemented it
func (d *Devices) Get(ignored bool, r *Result) error {
	// Dial a connection to the rtnetlink socket
	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	r.Neighbours, _ = getDevices(conn)

	return nil
}

//Info is a wrapper struct to add a time variable to neighinfo to detect when old STALE states become live
type Info struct {
	NeighborState global.NeighInfo
	Occured       time.Time
}

//DeviceWatcher serves a list of rpc clients to inform them of network device liveness changes
func DeviceWatcher(rpcClients <-chan *rpc.Client) {

	// Dial a connection to the rtnetlink socket
	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	states := map[string]Info{}

	setupHosts, err := getDevices(conn)
	if err != nil {
		log.Fatal(err)
	}

	//Add some entries initially, so we dont send 60000 messages on startup
	for _, v := range setupHosts {
		states[v.Ip] = Info{
			NeighborState: v,
			Occured:       time.Now(),
		}
	}

	var observers []*rpc.Client

	for {

		select {
		case rpcClient := <-rpcClients:
			observers = append(observers, rpcClient)

		case <-time.After(1 * time.Second):
			neigh, err := getDevices(conn)
			if err != nil {
				log.Fatal(err)
			}

			var newObservers []*rpc.Client
			var changes []Info
			for i := range neigh {
				v := neigh[i]
				previousState, ok := states[v.Ip]

				if previousState.NeighborState != v || !ok {

					nv := Info{
						Occured:       time.Now(),
						NeighborState: v,
					}

					states[v.Ip] = nv

					//If we move from a FAILED or INCOMPLETE to a state that is neither FAILED or INCOMPLETE then send a liveness notification
					failedToLive := (previousState.NeighborState.State&unix.NUD_FAILED != 0 || previousState.NeighborState.State&unix.NUD_INCOMPLETE != 0) && v.State&(unix.NUD_FAILED|unix.NUD_INCOMPLETE) == 0
					//If a stale state changed from being stale after not having an event for awhile
					oldStaleToElse := (previousState.NeighborState.State&unix.NUD_STALE != 0 && v.State&unix.NUD_STALE == 0)

					fmt.Printf("%-11.8s|\t%s\t|\t%t\t|\t%s->%s\n", previousState.Occured.Format("15:04:05"), v.Ip, (!ok || (failedToLive || oldStaleToElse) && previousState.Occured.Before(time.Now().Add(-10*time.Minute))), global.GetState(previousState.NeighborState.State), global.GetState(v.State))
					if !ok || (failedToLive || oldStaleToElse) && previousState.Occured.Before(time.Now().Add(-10*time.Minute)) {
						changes = append(changes, nv)
					}
				}
			}

			if len(changes) == 0 {
				continue
			}

			//This sucks a bit, doesnt scale very well. But meh
			for i := range observers {

				failed := false
				for ii := range changes {
					if observers[i].Call("Notification.Do", changes[ii].NeighborState, nil) != nil {
						observers[i].Close()
						failed = true
						break
					}
				}

				if !failed {
					newObservers = append(newObservers, observers[i]) // Effectively remove any rpc clients that couldnt be contacted
				}
			}

			observers = newObservers

		}

	}
}
