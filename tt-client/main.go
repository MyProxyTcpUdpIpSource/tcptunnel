package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	pb "github.com/Randomsock5/tcptunnel/proto"
	"github.com/Randomsock5/tcptunnel/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	_ "net/http/pprof"
)

var (
	addr      = flag.String("server", "127.0.0.1", "Set server address")
	port      = flag.Int("port", 8443, "Set server port")
	localAddr = flag.String("local", "", "Set local address")
	localPort = flag.Int("localPort", 8088, "Set local port")
	pac       = flag.String("pac", "./pac.txt", "Set pac path")
	password  = flag.String("password", "password", "password")

	certFile = flag.String("cert_file", "client2server.crt", "The TLS cert file")
	keyFile  = flag.String("key_file", "client.key", "The TLS key file")
	caFile   = flag.String("ca_file", "ca.crt", "The TLS ca file")
)

func main() {
	flag.Parse()

	go func() {
		log.Println(http.ListenAndServe(fmt.Sprintf("%s:%d", *localAddr, *localPort+1), nil))
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

	sessionCache := tls.NewLRUClientSessionCache(64)
	ta := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
		ServerName:   "Unknown",
		MinVersion:   tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{
			tls.CurveP521,
			tls.CurveP384,
			tls.CurveP256,
		},
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		},
		PreferServerCipherSuites:    true,
		ClientSessionCache:          sessionCache,
		DynamicRecordSizingDisabled: false,
	})

	var opts []grpc.DialOption
	opts = []grpc.DialOption{
		grpc.WithTransportCredentials(ta),
		grpc.WithDialer(func(addr string, duration time.Duration) (net.Conn, error) {
			aesConn, err := transport.Dial(addr, *password, duration)
			return aesConn, err
		}),
	}

	conn, err := grpc.Dial(
		fmt.Sprintf("%s:%d", *addr, *port),
		opts...)
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
