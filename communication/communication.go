package communication

import (
	"net"
)

func Copy(dest net.Conn, source net.Conn) {
	received := make([]byte, 1024)
	for {
		n, err := source.Read(received)
		if n > 0 {
			_, err := dest.Write(received[0:n])

			if err != nil {
				dest.Close()
			}

		}
		if err != nil {
			source.Close()
		}
	}
}
