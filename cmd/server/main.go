// A small SSH daemon providing bash sessions
//
// Server:
// cd my/new/dir/
// #generate server keypair
// ssh-keygen -t rsa
// go get -v .
// go run sshd.go
//
// Client:
// ssh foo@localhost -p 2200 #pass=bar

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/rpc"

	"github.com/NHAS/all-seeing-eye-router/internal/global"
	"github.com/NHAS/all-seeing-eye-router/internal/server"

	"golang.org/x/crypto/ssh"
)

func main() {

	//Taken from the server example, authorized keys are required for controllers
	authorizedKeysBytes, err := ioutil.ReadFile("authorized_keys")
	if err != nil {
		log.Fatalf("Failed to load authorized_keys, err: %v", err)
	}

	authorizedKeysMap := map[string]bool{}
	for len(authorizedKeysBytes) > 0 {
		pubKey, _, _, rest, err := ssh.ParseAuthorizedKey(authorizedKeysBytes)
		if err != nil {
			log.Fatal(err)
		}

		authorizedKeysMap[string(pubKey.Marshal())] = true
		authorizedKeysBytes = rest
	}

	// In the latest version of crypto/ssh (after Go 1.3), the SSH server type has been removed
	// in favour of an SSH connection type. A ssh.ServerConn is created by passing an existing
	// net.Conn and a ssh.ServerConfig to ssh.NewServerConn, in effect, upgrading the net.Conn
	// into an ssh.ServerConn
	config := &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if authorizedKeysMap[string(key.Marshal())] {
				return &ssh.Permissions{}, nil
			}
			return nil, fmt.Errorf("unknown public key for %q", conn.User())
		},
	}

	// You can generate a keypair with 'ssh-keygen -t rsa'
	privateBytes, err := ioutil.ReadFile("id_ed25519")
	if err != nil {
		log.Fatal("Failed to load private key (./id_ed25519)")
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key")
	}

	config.AddHostKey(private)

	// Once a ServerConfig has been configured, connections can be accepted.
	listener, err := net.Listen("tcp", "0.0.0.0:3232")
	if err != nil {
		log.Fatalf("Failed to listen on 2200 (%s)", err)
	}

	r := rpc.NewServer()
	err = r.Register(&server.Devices{})
	if err != nil {
		log.Fatal(err)
	}

	notifyClientsChan := make(chan *rpc.Client)
	go server.DeviceWatcher(notifyClientsChan)

	// Accept all connections
	log.Print("Listening on 2200...")
	for {
		tcpConn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept incoming connection (%s)", err)
			continue
		}
		// Before use, a handshake must be performed on the incoming net.Conn.
		sshConn, chans, reqs, err := ssh.NewServerConn(tcpConn, config)
		if err != nil {
			log.Printf("Failed to handshake (%s)", err)
			continue
		}

		log.Printf("New SSH connection from %s (%s)", sshConn.RemoteAddr(), sshConn.ClientVersion())
		// Discard all global out-of-band Requests
		go ssh.DiscardRequests(reqs)
		// Accept all channels
		go global.HandleChannels(chans, r)

		go func() {
			data, rs, err := sshConn.OpenChannel("rpc", nil)
			if err != nil {
				log.Fatalf("Unable to start a new rpc channel: %s\n", err)
			}
			//Data channel close intentionally missing, there would be no way for this go chan to be paused/told to stop
			go ssh.DiscardRequests(rs)

			notifyClientsChan <- rpc.NewClient(data)
		}()
	}
}
