package communication

import (
	"io"
	"net"
)

func Copy(dest net.Conn, source net.Conn) {
	io.Copy(dest, source)

	// received := make([]byte, 4096)
	// for {
	// 	n, err := source.Read(received)
	// 	if n > 0 {
	// 		dest.Write(received[0:n])
	// 	}
	// 	if err != nil {
	// 		break
	// 	}
	// }

	// errSource := source.Close()
	// errDest := dest.Close()

	// return errSource, errDest
}
