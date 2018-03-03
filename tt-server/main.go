package main

import (
	"flag"
	"io"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/Randomsock5/tcptunnel/transport"
	"github.com/Randomsock5/tcptunnel/github.com/xtaci/smux/smux"

	"net/http"
	_ "net/http/pprof"
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

	go func() {
		log.Println(http.ListenAndServe(":8080", nil))
	}()

	listen, err := transport.Listen(addr+":"+strconv.Itoa(port), password)
	if err != nil {
		log.Fatalln(err)
		return
	}
	defer listen.Close()

	smuxConfig := &smux.Config{
		KeepAliveInterval: 10 * time.Second,
		KeepAliveTimeout:  30 * time.Second,
		MaxFrameSize:      4096,
		MaxReceiveBuffer:  4194304,
	}

	for {
		conn, err := listen.Accept()
		if err != nil {
			log.Println(err)
			continue
		}

		session, err := smux.Server(conn, smuxConfig)
		if err != nil {
			log.Println(err)
			continue
		}
		go handleSession(session)
	}
}

func handleSession(session *smux.Session){
	for {
		stream, err := session.AcceptStream()
		if err != nil {
			continue
		}

		go func(conn net.Conn) {
			forwardConn, err := net.Dial("tcp", forward)
			if err != nil {
				log.Println(err)
				conn.Close()
				return
			}

			//loop check
			if conn.RemoteAddr().String() == forwardConn.RemoteAddr().String() {
				conn.Close()
				forwardConn.Close()
				return
			}

			go copyAndClose(conn, forwardConn)
			go copyAndClose(forwardConn, conn)
		}(stream)
	}
}

func copyAndClose(w, r net.Conn) {
	defer w.Close()
	for {
		buf := make([]byte, 64)

		// written == 0 fix cpu profile bug
		if written, err := io.CopyBuffer(w, r, buf); err != nil || written ==0 {
			break
		}
	}
}
