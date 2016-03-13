package main

import (
	"flag"
	"io"
	"log"
	"net"
	"strconv"

	"github.com/Randomsock5/tcptunnel/encodes"
)

var (
	addr      string
	port      int
	localAddr string
	localPort int
	password  string
)

func init() {
	flag.StringVar(&addr, "server", "", "Set server address")
	flag.IntVar(&port, "port", 8443, "Set server port")
	flag.StringVar(&localAddr, "loacl", "", "Set loacl address")
	flag.IntVar(&localPort, "localPort", 8088, "Set loacl port")
	flag.StringVar(&password, "password", "asdfghjkl", "Password")
}

func main() {
	flag.Parse()

	localServer, err := net.Listen("tcp", localAddr+":"+strconv.Itoa(localPort))
	if err != nil {
		log.Fatal(err)
	}
	defer localServer.Close()

	for {
		conn, err := localServer.Accept()
		if err != nil {
			log.Println(err)
			continue
		}

		go func(c net.Conn) {
			aesconn, err := encodes.Dial(addr+":"+strconv.Itoa(port), password)
			if err != nil {
				log.Println(err)
				return
			}

			go copyAndClose(aesconn, c)
			go copyAndClose(c, aesconn)
		}(conn)
	}
}

func copyAndClose(w, r net.Conn) {
	io.Copy(w, r)
	r.Close()
}
