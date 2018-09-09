package transport

import (
	"bufio"
	"context"
	pb "github.com/Randomsock5/tcptunnel/proto"
	"log"
	"net"
	"time"
)

func handleErr(err error) {
	if err != nil {
		panic(err)
	}
}

func recoverHandle(ch chan error) {
	if rec := recover(); rec != nil {
		err := rec.(error)
		ch <- err
	}
	close(ch)
}

type proxyService struct {
	forward string
}

func (s *proxyService) Stream(stream pb.ProxyService_StreamServer) error {
	forwardConn, err := net.Dial("tcp", s.forward)
	if err != nil {
		log.Println(err)
		return err
	}
	defer forwardConn.Close()
	wait1 := make(chan error)
	wait2 := make(chan error)
	defer close(wait1)
	defer close(wait2)

	//
	go func() {
		defer recoverHandle(wait1)

		for {
			buf := make([]byte, 768)
			i, err := forwardConn.Read(buf)
			handleErr(err)

			var payload pb.Payload
			payload.Data = buf[:i]

			err = stream.Send(&payload)
			handleErr(err)
		}
	}()

	go func() {
		defer recoverHandle(wait2)
		writeBuff := bufio.NewWriter(forwardConn)

		for {
			payload, err := stream.Recv()
			handleErr(err)

			data := payload.GetData()
			_, err = writeBuff.Write(data)
			handleErr(err)

			err = writeBuff.Flush()
			handleErr(err)
		}
	}()

	select {
	case err = <-wait1:
		if err != nil {
			log.Println(err)
			return err
		}

	case err = <-wait2:
		if err != nil {
			log.Println(err)
			return err
		}
	}

	return nil
}

func NewServer(forward string) pb.ProxyServiceServer {
	s := &proxyService{forward: forward}
	return s
}

func ClientProxyService(conn net.Conn, client pb.ProxyServiceClient) {
	defer conn.Close()
	wait1 := make(chan error)
	wait2 := make(chan error)
	defer close(wait1)
	defer close(wait2)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stream, err := client.Stream(ctx)
	if err != nil {
		log.Println(err)
		return
	}
	defer stream.CloseSend()

	go func() {
		defer recoverHandle(wait1)

		for {
			buf := make([]byte, 768)
			i, err := conn.Read(buf)
			handleErr(err)

			var payload pb.Payload
			payload.Data = buf[:i]

			err = stream.Send(&payload)
			handleErr(err)
		}
	}()

	go func() {
		defer recoverHandle(wait2)
		writeBuff := bufio.NewWriter(conn)

		for {
			payload, err := stream.Recv()
			handleErr(err)

			data := payload.GetData()
			_, err = writeBuff.Write(data)
			handleErr(err)

			err = writeBuff.Flush()
			handleErr(err)
		}
	}()

	select {
	case err = <-wait1:
		if err != nil {
			log.Println(err)
			return
		}

	case err = <-wait2:
		if err != nil {
			log.Println(err)
			return
		}
	}

	return
}
