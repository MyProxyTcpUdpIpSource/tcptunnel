package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/Randomsock5/tcptunnel/constants"

	pb "github.com/Randomsock5/tcptunnel/proto"
	"github.com/Randomsock5/tcptunnel/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	_ "net/http/pprof"
)

var (
	addr     = flag.String("server", "", "Set server address")
	port     = flag.Int("port", 8443, "Set server port")
	forward  = flag.String("forward", "127.0.0.1:3128", "Set forward address")
	password = flag.String("password", "password", "password")

	certFile = flag.String("cert_file", "server2client.crt", "The TLS cert file")
	keyFile  = flag.String("key_file", "server.key", "The TLS key file")
	caFile   = flag.String("ca_file", "ca.crt", "The TLS ca file")
)

func main() {
	flag.Parse()

	go func() {
		log.Println(http.ListenAndServe(fmt.Sprintf("%s:%d", *addr, *port+1), nil))
	}()

	listen, err := transport.Listen(fmt.Sprintf("%s:%d", *addr, *port), *password)
	if err != nil {
		log.Fatalln(err)
		return
	}
	defer listen.Close()

	caCert, err := ioutil.ReadFile(*caFile)
	if err != nil {
		log.Fatalf("read ca cert file error:%v", err)
		return
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	if err != nil {
		log.Fatalf("read ca cert file error:%v", err)
		return
	}

	ta := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
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
		DynamicRecordSizingDisabled: false,
	})

	var opts []grpc.ServerOption
	opts = []grpc.ServerOption{
		grpc.Creds(ta),
		grpc.ConnectionTimeout(6 * constants.ConnTimeout),
		grpc.WriteBufferSize(1024),
		grpc.ReadBufferSize(1024),
	}

	grpcServer := grpc.NewServer(opts...)
	pb.RegisterProxyServiceServer(grpcServer, transport.NewServer(*forward))

	grpcServer.Serve(listen)
}
