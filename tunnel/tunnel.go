package tunnel

import (
	"fmt"
	"net"
	"sync"
	"time"

	"zpaceway.com/gotunnel/communication"
	"zpaceway.com/gotunnel/constants"
)

func CreateTunnel(clientPort string, proxyPort string, connectionTimeout time.Duration) {
	availableProxies := make([]net.Conn, 0)
	lock := sync.Mutex{}

	clientListener, err := net.Listen("tcp", "0.0.0.0:"+clientPort)
	if err != nil {
		fmt.Println("Error listening on port", clientPort)
		panic(err)
	}
	fmt.Println("Client Tunnel listening on port "+clientPort+" with timeout of", connectionTimeout)
	proxylistener, err := net.Listen("tcp", "0.0.0.0:"+proxyPort)
	if err != nil {
		fmt.Println("Error listening on port", clientPort)
		panic(err)
	}
	fmt.Println("Proxy Tunnel listening on port " + proxyPort)

	go (func() {
		for {
			proxyConn, err := proxylistener.Accept()

			if err != nil {
				continue
			}

			go (func() {
				connectionMessage := make([]byte, 1024)
				n, err := proxyConn.Read(connectionMessage)

				if err != nil {
					return
				}

				if string(connectionMessage[0:n]) == constants.TUNNEL_PROXY_CONNECT_MESSAGE {
					lock.Lock()
					availableProxies = append(availableProxies, proxyConn)
					lock.Unlock()
				}
			})()

		}
	})()

	go (func() {
		for {
			clientConn, err := clientListener.Accept()

			if err != nil || clientConn == nil {
				continue
			}

			var elapsedTimeRetrying time.Duration = 0
			for len(availableProxies) == 0 {
				time.Sleep(time.Second / 10)
				elapsedTimeRetrying += time.Second / 10
				if elapsedTimeRetrying >= connectionTimeout {
					clientConn.Close()
					break
				}
			}

			if elapsedTimeRetrying >= connectionTimeout {
				continue
			}

			proxyConn := availableProxies[0]
			availableProxies = availableProxies[1:]
			go communication.Copy(proxyConn, clientConn)
			go communication.Copy(clientConn, proxyConn)
		}
	})()
}
