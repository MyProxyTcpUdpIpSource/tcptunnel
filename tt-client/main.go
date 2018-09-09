package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	pb "github.com/Randomsock5/tcptunnel/proto"
	"github.com/Randomsock5/tcptunnel/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	_ "net/http/pprof"
)

var (
	addr      = flag.String("server", "", "Set server address")
	port      = flag.Int("port", 8443, "Set server port")
	localAddr = flag.String("local", "", "Set local address")
	localPort = flag.Int("localPort", 8088, "Set local port")
	pac       = flag.String("pac", "./pac.txt", "Set pac path")

	certFile = flag.String("cert_file", "client2server.crt", "The TLS cert file")
	keyFile  = flag.String("key_file", "client.key", "The TLS key file")
	caFile   = flag.String("ca_file", "ca.crt", "The TLS ca file")
)

func main() {
	flag.Parse()

	go func() {
		log.Println(http.ListenAndServe(":9081", nil))
	}()

	if exist(*pac) {
		b, err := ioutil.ReadFile(*pac)
		if err != nil {
			log.Println("Can not read file: " + *pac)
		}

		pacPort := *localPort + 1
		s := string(b[:])
		if *localAddr != "" {
			s = strings.Replace(s, "__PROXY__", "PROXY "+*localAddr+":"+strconv.Itoa(*localPort)+";", 1)
			log.Printf("pac uri: %s:%d/pac\n", *localAddr, pacPort)
		} else {
			s = strings.Replace(s, "__PROXY__", "PROXY 127.0.0.1:"+strconv.Itoa(*localPort)+";", 1)
			log.Printf("pac uri: 127.0.0.1:%d/pac \n", pacPort)
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/pac", func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, s)
		})
		go http.ListenAndServe(fmt.Sprintf("%s:%d", *localAddr, pacPort), mux)
	} else {
		log.Println("Can not find file: " + *pac)
	}

	localServer, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *localAddr, *localPort))
	if err != nil {
		log.Fatal(err)
	}
	defer localServer.Close()

	caCert, err := ioutil.ReadFile(*caFile)
	if err != nil {
		log.Fatalf("read ca cert file error:%v", err)
		return
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Fatalf("load peer cert/key error:%v", err)
		return
	}

	ta := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
		ServerName:   "Unknown",
		MinVersion:   tls.VersionTLS12,
		MaxVersion:   tls.VersionTLS12,
	})

	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", *addr, *port), grpc.WithTransportCredentials(ta))
	if err != nil {
		log.Fatalln(err)
	}
	client := pb.NewProxyServiceClient(conn)

	for {
		sources, err := localServer.Accept()
		if err != nil {
			log.Println(err)
			continue
		}

		go transport.ClientProxyService(sources, client)
	}
}

func exist(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil || os.IsExist(err)
}
