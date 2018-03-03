package main

import (
	"flag"
	"io"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/Randomsock5/tcptunnel/transport"
)

var (
	addr     string
	port     int
	forward  string
	password string
)

func init() {
	flag.StringVar(&addr, "server", "", "Set server address")
	flag.IntVar(&port, "port", 8443, "Set server port")
	flag.StringVar(&forward, "forward", "127.0.0.1:3128", "Set forward address")
	flag.StringVar(&password, "password", "4a99a760", "Password")
}

func main() {
	flag.Parse()

	l, err := transport.Listen(addr+":"+strconv.Itoa(port), password)
	if err != nil {
		log.Fatalln(err)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println(err)
			continue
		}

		go func(aesConn net.Conn) {
			forwardConn, err := net.Dial("tcp", forward)
			if err != nil {
				log.Println(err)
				aesConn.Close()
				return
			}

			//loop check
			if aesConn.RemoteAddr().String() == forwardConn.RemoteAddr().String() {
				aesConn.Close()
				forwardConn.Close()
				return
			}

			go copyAndClose(aesConn, forwardConn)
			go copyAndClose(forwardConn, aesConn)
		}(conn)
	}
}

func copyAndClose(w, r net.Conn) {
	buf := make([]byte, 65536)
	defer w.Close()

	for {
		r.SetDeadline(time.Now().Add(120 * time.Second))

		if written, err := io.CopyBuffer(w, r, buf); err != nil || written == 0 {
			return
		}
	}
}
