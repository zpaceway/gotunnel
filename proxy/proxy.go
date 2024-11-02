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

		for index, line := range strings.Split(data, "\r\n") {
			if index == 0 {
				if strings.HasPrefix(line, "CONNECT") {
					ssl = true
				}
				parts := strings.Fields(line)
				if len(parts) > 1 {
					host = parts[1]
					break
				}
			}
			if host == "" && strings.HasPrefix(line, "Host: ") {
				host = strings.TrimPrefix(line, "Host: ")
				break
			}
		}

		if host == "" {
			tunnelConnection.Close()
			return
		}

		host = strings.TrimPrefix(host, "http://")
		host = strings.TrimPrefix(host, "https://")
		host = strings.Split(host, "/")[0]
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

	select {}
}
