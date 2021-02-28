package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/rpc"

	"github.com/NHAS/all-seeing-eye-router/internal/client"
	"github.com/NHAS/all-seeing-eye-router/internal/global"

	"golang.org/x/crypto/ssh"
)

func main() {

	addr := flag.String("address", "localhost:3232", "Client connection address")

	flag.Parse()

	privateBytes, err := ioutil.ReadFile("id_ed25519")
	if err != nil {
		log.Fatal("Failed to load private key (./id_ed25519)")
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key")
	}

	config := &ssh.ClientConfig{
		User: "rpcuser",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(private),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			//	if ssh.FingerprintSHA256(key) != serverPubKey {
			//	return fmt.Errorf("Server public key invalid, expected: %s, got: %s", serverPubKey, internal.FingerprintSHA256Hex(key))
			//}

			return nil
		},
	}

	fmt.Printf("Connecting to %s....", *addr)
	conn, err := net.DialTimeout("tcp", *addr, config.Timeout)
	if err != nil {
		log.Fatalf("Unable to connect TCP: %s\n", err)

	}
	defer conn.Close()
	fmt.Printf("Success!\n")

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, *addr, config)
	if err != nil {
		log.Fatalf("Unable to start a new client connection: %s\n", err)
	}
	defer sshConn.Close()
	go ssh.DiscardRequests(reqs) // Then go on to ignore everything else

	rpcServer := rpc.NewServer()
	m, err := global.LoadManfactureDatabase()
	if err != nil {
		log.Fatal(err)
	}

	err = rpcServer.Register(&client.Notification{
		ManufacturerDB: m,
	})

	if err != nil {
		log.Fatalln(err)
	}

	global.HandleChannels(chans, rpcServer)
}
