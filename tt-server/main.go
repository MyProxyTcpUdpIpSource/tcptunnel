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
	"io/ioutil"
	"log"
	"net"
	"net/http"
)

var (
	addr    = flag.String("server", "", "Set server address")
	port    = flag.Int("port", 8443, "Set server port")
	forward = flag.String("forward", "127.0.0.1:3128", "Set forward address")

	certFile = flag.String("cert_file", "server2client.crt", "The TLS cert file")
	keyFile  = flag.String("key_file", "server.key", "The TLS key file")
	caFile   = flag.String("ca_file", "ca.crt", "The TLS ca file")
)

func main() {
	flag.Parse()

	go func() {
		log.Println(http.ListenAndServe(":8080", nil))
	}()

	listen, err := net.Listen("tcp", fmt.Sprintf("%s:%d", *addr, *port))
	if err != nil {
		log.Fatalln(err)
		return
	}
	defer listen.Close()

	var opts []grpc.ServerOption

	caCert, err := ioutil.ReadFile(*caFile)
	if err != nil {
		log.Fatalf("read ca cert file error:%v", err)
		return
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	cert, err := tls.LoadX509KeyPair(*certFile, *keyFile)
	ta := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caCertPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ServerName:   "Unknown",
		MinVersion:   tls.VersionTLS12,
		MaxVersion:   tls.VersionTLS12,
	})
	opts = []grpc.ServerOption{grpc.Creds(ta)}

	grpcServer := grpc.NewServer(opts...)
	pb.RegisterProxyServiceServer(grpcServer, transport.NewServer(*forward))

	grpcServer.Serve(listen)
}
