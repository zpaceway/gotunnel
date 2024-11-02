package main

import (
	"flag"
	"fmt"
	"time"

	"zpaceway.com/gotunnel/proxy"
	"zpaceway.com/gotunnel/tunnel"
)

var (
	mode         = flag.String("mode", "all", "Mode to run: all, tunnel, proxy")
	tcport       = flag.String("tcport", "1080", "Tunnel client port")
	tpport       = flag.String("tpport", "1081", "Tunnel proxy port")
	timeout      = flag.Duration("timeout", time.Second*5, "Connection timeout in seconds")
	thost        = flag.String("thost", "localhost:1081", "Proxy host")
	availability = flag.Uint("availability", 5, "Proxy availability")
)

func init() {
	flag.Parse()
}

func main() {
	switch *mode {
	case "all":
		go tunnel.CreateTunnel(*tcport, *tpport, *timeout)
		go proxy.CreateProxy(*thost, *availability)
	case "tunnel":
		go tunnel.CreateTunnel(*tcport, *tpport, *timeout)
	case "proxy":
		go proxy.CreateProxy(*thost, *availability)
	default:
		fmt.Println("Invalid mode. Use one of: all, tunnel, proxy")
		return
	}

	select {}
}
