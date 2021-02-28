package client

import (
	"fmt"
	"log"
	"net"
	"os/exec"

	"github.com/NHAS/all-seeing-eye-router/internal/global"
)

//Notification is used in the RPC client to recieve notifications from the server
type Notification struct {
	ManufacturerDB map[string]string
}

//Do is the method called to present a notification on desktop
func (n *Notification) Do(a global.NeighInfo, b *bool) error {

	addrs, _ := net.LookupAddr(a.Ip)

	if len(addrs) > 0 {
		a.Ip = fmt.Sprintf("%s (%s)", addrs[0], a.Ip)
	}

	message :=
		`
	Address: %s
	MAC: %s
	`
	log.Println(a)
	cmd := exec.Command("/usr/bin/dunstify", "-t", "20000", "Connection Event", fmt.Sprintf(message, a.Ip, a.LLaddr))
	err := cmd.Start()
	if err != nil {
		log.Fatal(err)
	}

	return nil
}
