package main

import (
	"flag"
	"io"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/Randomsock5/tcptunnel/encodes"
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
	flag.StringVar(&password, "password", "asdfghjkl", "Password")
}

func main() {
	flag.Parse()

	l, err := encodes.Listen(addr+":"+strconv.Itoa(port), password)
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

		go func(aesconn net.Conn) {
			forwardconn, err := net.Dial("tcp", forward)
			if err != nil {
				log.Println(err)
				aesconn.Close()
				return
			}

			//loop check
			if aesconn.RemoteAddr().String() == forwardconn.RemoteAddr().String() {
				aesconn.Close()
				forwardconn.Close()
				return
			}

			go copyAndClose(aesconn, forwardconn)
			go copyAndClose(forwardconn, aesconn)
		}(conn)
	}
}

func copyAndClose(w, r net.Conn) {
	buf := make([]byte, 65536)
	defer w.Close()

	for {
		r.SetReadDeadline(time.Now().Add(60 * time.Second))

		if written, err := io.CopyBuffer(w, r, buf); err != nil || written == 0 {
			return
		}
	}
}
