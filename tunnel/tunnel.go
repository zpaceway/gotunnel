package tunnel

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"zpaceway.com/gotunnel/communication"
	"zpaceway.com/gotunnel/constants"
)

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
	availableProxiesByCountry := make(map[string][]net.Conn)
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
					proxyAddress := proxyConn.RemoteAddr().String()
					proxyIpAddress := proxyAddress[:strings.LastIndex(proxyAddress, ":")]
					proxyCountryCode := cache[proxyIpAddress]
					if proxyCountryCode == "" {
						proxyCountryCode = GetIpAddressCountryCode(proxyIpAddress)
						cache[proxyIpAddress] = proxyCountryCode
					}
					fmt.Println(">>> Proxy Connected:", proxyIpAddress+"@"+proxyCountryCode)
					lock.Lock()
					availableProxiesByCountry[proxyCountryCode] = append(availableProxiesByCountry[proxyCountryCode], proxyConn)
					lock.Unlock()
				}
			})()

		}
	})()

	go (func() {
		for {
			clientConn, err := clientListener.Accept()
			var proxyConn net.Conn

			if err != nil || clientConn == nil {
				continue
			}

			go (func() {

				head := make([]byte, 1024)
				n, err := clientConn.Read(head)

				if err != nil {
					clientConn.Close()
					return
				}

				data := string(head[0:n])
				fmt.Println(data)

				for _, line := range strings.Split(data, "\r\n") {
					if strings.HasPrefix(line, "Proxy-Authorization: Basic ") {
						formattedIdentifierAndPasswordEncoded := strings.TrimPrefix(line, "Proxy-Authorization: Basic ")
						formattedIdentifierAndPasswordDecoded, err := base64.RawStdEncoding.DecodeString(formattedIdentifierAndPasswordEncoded)
						if err != nil {
							fmt.Println(err)
							clientConn.Close()
							return
						}

						formattedIdentifierAndPassword := strings.Split(string(formattedIdentifierAndPasswordDecoded), ":")

						formattedIdentifier := formattedIdentifierAndPassword[0]
						password := formattedIdentifierAndPassword[1]

						parts := strings.Split(formattedIdentifier, "-")
						if len(parts) != 2 {
							clientConn.Close()
							return
						}

						username := strings.TrimPrefix(parts[0], "U!")
						countryCode := strings.TrimPrefix(parts[1], "C!")

						fmt.Println(">>> Client Connected:", username+":"+password+"@"+countryCode)

						lock.Lock()
						var elapsedTimeRetrying time.Duration = 0
						for len(availableProxiesByCountry[countryCode]) == 0 {
							time.Sleep(time.Second / 10)
							elapsedTimeRetrying += time.Second / 10
							if elapsedTimeRetrying >= connectionTimeout {
								clientConn.Close()
								return
							}
						}

						proxyConn = availableProxiesByCountry[countryCode][0]
						availableProxiesByCountry[countryCode] = availableProxiesByCountry[countryCode][1:]
						lock.Unlock()

						break
					}

				}

				if proxyConn == nil {
					clientConn.Close()
					return
				}

				proxyConn.Write(head[0:n])
				go communication.Copy(proxyConn, clientConn)
				go communication.Copy(clientConn, proxyConn)
			})()
		}
	})()
}
