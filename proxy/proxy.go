package proxy

import (
	"fmt"
	"net"
	"strings"
	"time"

	"zpaceway.com/gotunnel/communication"
	"zpaceway.com/gotunnel/constants"
)

func NewProxyJumper(tunnel string) {
	tunnelConnection, err := net.Dial("tcp", tunnel)

	if err != nil {
		time.Sleep(time.Second)
		return
	}

	_, err = tunnelConnection.Write([]byte(constants.TUNNEL_PROXY_CONNECT_MESSAGE))

	if err != nil {
		tunnelConnection.Close()
		time.Sleep(time.Second)
		return
	}

	received := make([]byte, 1024)
	var target net.Conn
	host := ""
	ssl := false

	n, err := tunnelConnection.Read(received)

	if err != nil {
		tunnelConnection.Close()
		return
	}

	go (func() {
		data := string(received[:n])

		for _, line := range strings.Split(data, "\r\n") {
			if strings.HasPrefix(line, "Host: ") {
				host = strings.TrimPrefix(line, "Host: ")
			}
			if strings.HasPrefix(line, "CONNECT") {
				ssl = true
			}
		}

		if host == "" {
			tunnelConnection.Close()
			return
		}

		if !strings.Contains(host, ":") {
			host += ":443"
		}
		fmt.Println(">>>", host)
		target, err = net.Dial("tcp", host)

		if err != nil {
			tunnelConnection.Close()
			return
		}

		go communication.Copy(target, tunnelConnection)
		go communication.Copy(tunnelConnection, target)

		if ssl {
			tunnelConnection.Write([]byte(constants.CONNECTION_ESTABLISHED_MESSAGE))
			return
		}

		tunnelConnection.Write(received[:n])
	})()
}

func CreateProxy(tunnel string, availability uint) {
	fmt.Println("Creating proxy to "+tunnel+" with availability of", availability)
	for range availability {
		go (func() {
			for {
				NewProxyJumper(tunnel)
			}
		})()
	}
}
