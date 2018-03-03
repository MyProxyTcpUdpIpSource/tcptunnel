package main

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Randomsock5/tcptunnel/transport"
	"github.com/Randomsock5/tcptunnel/github.com/xtaci/smux/smux"
)

var (
	addr      string
	port      int
	localAddr string
	localPort int
	password  string
	pac       string
)

func init() {
	flag.StringVar(&addr, "server", "", "Set server address")
	flag.IntVar(&port, "port", 8443, "Set server port")
	flag.StringVar(&localAddr, "local", "", "Set local address")
	flag.IntVar(&localPort, "localPort", 8088, "Set local port")
	flag.StringVar(&pac, "pac", "./pac.txt", "Set pac path")
	flag.StringVar(&password, "password", "4a99a760", "Password")
}

func main() {
	flag.Parse()

	if exist(pac) {
		b, err := ioutil.ReadFile(pac)
		if err != nil {
			log.Println("Can not read file: " + pac)
		}

		pacPort := localPort + 1
		s := string(b[:])
		if localAddr != "" {
			s = strings.Replace(s, "__PROXY__", "PROXY "+localAddr+":"+strconv.Itoa(localPort)+";", 1)
			log.Println("pac uri: " + localAddr + ":" + strconv.Itoa(pacPort) + "/pac")
		} else {
			s = strings.Replace(s, "__PROXY__", "PROXY 127.0.0.1:"+strconv.Itoa(localPort)+";", 1)
			log.Println("pac uri: 127.0.0.1:" + strconv.Itoa(pacPort) + "/pac")
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/pac", func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, s)
		})
		go http.ListenAndServe(localAddr+":"+strconv.Itoa(pacPort), mux)
	} else {
		log.Println("Can not find file: " + pac)
	}

	localServer, err := net.Listen("tcp", localAddr+":"+strconv.Itoa(localPort))
	if err != nil {
		log.Fatal(err)
	}
	defer localServer.Close()

	handleCh := clientCreate()
	for {
		conn, err := localServer.Accept()
		if err != nil {
			log.Println(err)
			continue
		}

		handleCh <- conn
	}

	close(handleCh)
}

func clientCreate() chan net.Conn {
	input := make(chan net.Conn)
	go clientHandle(input)
	return input
}


func clientHandle(input chan net.Conn){
	var transportUp = false
	var session *smux.Session
	var stream *smux.Stream
	var aesConn net.Conn
	var err error

	smuxConfig := &smux.Config{
		KeepAliveInterval: 10 * time.Second,
		KeepAliveTimeout:  30 * time.Second,
		MaxFrameSize:      4096,
		MaxReceiveBuffer:  4194304,
	}

	for conn := range input {
		if !transportUp {
			aesConn, err = transport.Dial(addr+":"+strconv.Itoa(port), password)
			if err != nil {
				log.Println(err)
				conn.Close()
				continue
			}
			session, err = smux.Client(aesConn, smuxConfig)
			if err != nil {
				log.Println(err)
				conn.Close()
				continue
			}
			transportUp = true
		} else {
			stream, err = session.OpenStream()
			if err != nil {
				log.Println(err)
				conn.Close()
				if session.IsClosed() {
					aesConn.Close()
					transportUp = false
				}
				continue
			}

			go copyAndClose(stream, conn)
			go copyAndClose(conn, stream)
		}
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

func exist(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil || os.IsExist(err)
}
