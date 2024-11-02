package tunnel

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"zpaceway.com/gotunnel/communication"
	"zpaceway.com/gotunnel/constants"
)

type LinkedList[T comparable] struct {
	Head *Node[T]
	Tail *Node[T]
}

type Node[T any] struct {
	Value *T
	Next  *Node[T]
}

func (list *LinkedList[T]) Add(value *T) {
	node := &Node[T]{Value: value}
	if list.Head == nil {
		list.Head = node
		list.Tail = node
		return
	}
	list.Tail.Next = node
	list.Tail = node
}

func (list *LinkedList[T]) Pop() *T {
	if list.Head == nil {
		return nil
	}
	value := list.Head.Value
	list.Head = list.Head.Next
	return value
}

func GetPublicIpAddress() string {
	resp, err := http.Get("https://checkip.amazonaws.com/")
	if err != nil {
		fmt.Println("Error fetching public IP address:", err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return ""
	}

	return strings.TrimSpace(string(body))
}

func GetIpAddressCountryCode(ipAddress string) string {

	if ipAddress == "" || ipAddress == "127.0.0.1" || ipAddress == "localhost" {
		ipAddress = GetPublicIpAddress()
	}

	type IpApiResponse struct {
		CountryCode string `json:"countryCode"`
	}

	resp, err := http.Get(fmt.Sprintf("http://ip-api.com/json/%s", ipAddress))
	if err != nil {
		fmt.Println("Error fetching IP information:", err)
		return "US"
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return "US"
	}

	var ipApiResponse IpApiResponse
	err = json.Unmarshal(body, &ipApiResponse)
	if err != nil {
		fmt.Println("Error unmarshalling JSON:", err)
		return "US"
	}

	return ipApiResponse.CountryCode
}

func CreateTunnel(clientPort string, proxyPort string, connectionTimeout time.Duration) {
	cache := make(map[string]string)
	availableProxiesByCountry := make(map[string]*LinkedList[net.Conn])

	for _, country := range constants.SUPPORTED_COUNTRIES {
		availableProxiesByCountry[country] = &LinkedList[net.Conn]{}
	}

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
					proxyConn.Close()
					return
				}

				if string(connectionMessage[0:n]) != constants.TUNNEL_PROXY_CONNECT_MESSAGE {
					proxyConn.Close()
					return
				}

				proxyAddress := proxyConn.RemoteAddr().String()
				proxyIpAddress := proxyAddress[:strings.LastIndex(proxyAddress, ":")]
				proxyCountryCode := cache[proxyIpAddress]

				if proxyCountryCode == "" {
					proxyCountryCode = GetIpAddressCountryCode(proxyIpAddress)
					cache[proxyIpAddress] = proxyCountryCode
				}

				for _, country := range constants.SUPPORTED_COUNTRIES {
					if country != proxyCountryCode {
						continue
					}
					availableProxiesByCountry[country].Add(&proxyConn)
					return
				}

				proxyConn.Close()
			})()

		}
	})()

	go (func() {
		for {
			clientConn, err := clientListener.Accept()
			fmt.Println(">>> Incomming connection...", clientConn.RemoteAddr().String())
			var proxyConn net.Conn

			if err != nil || clientConn == nil {
				continue
			}

			go (func() {
				headBytes := make([]byte, 1024)
				n, err := clientConn.Read(headBytes)

				if err != nil {
					clientConn.Close()
					return
				}

				head := string(headBytes[0:n])

				proxyAuthorizationLine := ""
				for index, line := range strings.Split(head, "\r\n") {
					if index == 0 {
						fmt.Println(">>> Request:", line)
					}

					if !strings.HasPrefix(line, "Proxy-Authorization: Basic ") {
						continue
					}
					proxyAuthorizationLine = line
				}

				formattedIdentifierEncoded := strings.TrimPrefix(proxyAuthorizationLine, "Proxy-Authorization: Basic ")
				formattedIdentifierDecoded, err := base64.StdEncoding.DecodeString(formattedIdentifierEncoded)

				if err != nil {
					clientConn.Close()
					return
				}

				formattedIdentifierParts := strings.Split(string(formattedIdentifierDecoded), "-")
				if len(formattedIdentifierParts) != 3 {
					clientConn.Close()
					return
				}

				username := strings.TrimPrefix(formattedIdentifierParts[0], "U!")
				countryCode := strings.TrimPrefix(formattedIdentifierParts[1], "C!")
				key := strings.TrimPrefix(formattedIdentifierParts[2], "K!")

				fmt.Println(">>> Client Connected:", username+":"+key+"@"+countryCode)

				var elapsedTimeRetrying time.Duration = 0
				for {
					proxyConnPointer := availableProxiesByCountry[countryCode].Pop()
					if proxyConnPointer != nil {
						proxyConn = *proxyConnPointer
						break
					}

					time.Sleep(time.Second / 5)
					elapsedTimeRetrying += time.Second / 5
					if elapsedTimeRetrying >= connectionTimeout {
						clientConn.Close()
						return
					}
				}

				proxyConn.Write(headBytes[0:n])
				go communication.Copy(proxyConn, clientConn)
				go communication.Copy(clientConn, proxyConn)
			})()
		}
	})()

	select {}
}
