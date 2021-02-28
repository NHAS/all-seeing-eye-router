package global

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/rpc"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/sys/unix"
)

type NeighInfo struct {
	Ip     string
	LLaddr string
	State  uint16
}

var states = []uint16{
	unix.NUD_INCOMPLETE,
	unix.NUD_REACHABLE,
	unix.NUD_STALE,
	unix.NUD_DELAY,
	unix.NUD_PROBE,
	unix.NUD_FAILED,
	unix.NUD_NOARP,
	unix.NUD_PERMANENT,
}

func LookupNUDState(i uint16) string {
	switch i {
	case unix.NUD_INCOMPLETE:
		return "INCOMPLETE"
	case unix.NUD_REACHABLE:
		return "REACHABLE"
	case unix.NUD_STALE:
		return "STALE"
	case unix.NUD_DELAY:
		return "DELAY"
	case unix.NUD_PROBE:
		return "PROBE"
	case unix.NUD_FAILED:
		return "FAILED"
	case unix.NUD_NOARP:
		return "NOARP"
	case unix.NUD_PERMANENT:
		return "PERMANENT"
	default:
		return "UNKNOWN STATE"
	}
}

func GetState(s uint16) (out string) {
	//https://man7.org/linux/man-pages/man7/rtnetlink.7.html

	applicableStates := []string{}
	for _, v := range states {
		if s&v != 0 {
			applicableStates = append(applicableStates, LookupNUDState(v))
		}
	}

	for i := range applicableStates {
		out += applicableStates[i]
		if i != len(applicableStates)-1 {
			out += "|"
		}
	}
	return out
}

func LoadManfactureDatabase() (m map[string]string, err error) {
	fs, err := os.Open("/usr/share/wireshark/manuf")
	if err != nil {
		return nil, err
	}

	m = make(map[string]string)

	reader := bufio.NewReader(fs)
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatal(err)
		}

		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			continue
		}

		m[parts[0]] = parts[1]
	}

	return m, nil
}

func HandleChannels(chans <-chan ssh.NewChannel, rpcServer *rpc.Server) {
	// Service the incoming Channel channel in go routine
	for newChannel := range chans {
		go HandleChannel(newChannel, rpcServer)
	}
}

func HandleChannel(newChannel ssh.NewChannel, rpcServer *rpc.Server) {
	// Since we're handling a shell, we expect a
	// channel type of "session". The also describes
	// "x11", "direct-tcpip" and "forwarded-tcpip"
	// channel types.
	if t := newChannel.ChannelType(); t != "rpc" {
		newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %s", t))
		return
	}

	// At this point, we have the opportunity to reject the client's
	// request for another logical connection
	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Printf("Could not accept channel (%s)", err)
		return
	}
	go ssh.DiscardRequests(requests)

	rpcServer.ServeConn(connection)

	log.Println("Client disconnected")
}
