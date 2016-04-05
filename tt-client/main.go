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

	"github.com/Randomsock5/tcptunnel/encodes"
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
	flag.StringVar(&password, "password", "asdfghjkl", "Password")
}

func main() {
	flag.Parse()

	if exist(pac) {
		b, err := ioutil.ReadFile(pac)
		if err != nil {
			log.Println("Can not read file: " + pac)
		}

		pacport := localPort + 1
		s := string(b[:])
		if localAddr != "" {
			s = strings.Replace(s, "__PROXY__", "PROXY "+localAddr+":"+strconv.Itoa(localPort)+";", 1)
			log.Println("pac uri: " + localAddr + ":" + strconv.Itoa(pacport) + "/pac")
		} else {
			s = strings.Replace(s, "__PROXY__", "PROXY 127.0.0.1:"+strconv.Itoa(localPort)+";", 1)
			log.Println("pac uri: 127.0.0.1:" + strconv.Itoa(pacport) + "/pac")
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/pac", func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, s)
		})
		go http.ListenAndServe(localAddr+":"+strconv.Itoa(pacport), mux)
	} else {
		log.Println("Can not find file: " + pac)
	}

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
				c.Close()
				return
			}

			//loop check
			if c.RemoteAddr().String() == aesconn.RemoteAddr().String() {
				aesconn.Close()
				c.Close()
				return
			}

			go copyAndClose(aesconn, c)
			go copyAndClose(c, aesconn)
		}(conn)
	}
}

func copyAndClose(w, r net.Conn) {
	buf := make([]byte, 65536)
	defer w.Close()

	for {
		r.SetDeadline(time.Now().Add(120 * time.Second))

		if written, err := io.CopyBuffer(w, r, buf); err != nil || written == 0 {
			break
		}
	}
}

func exist(filepath string) bool {
	_, err := os.Stat(filepath)
	return err == nil || os.IsExist(err)
}
