package transport

import (
	"bufio"
	"context"
	pb "github.com/Randomsock5/tcptunnel/proto"
	"io"
	"log"
	"net"
	"time"
)

func handleErr(err error) {
	if err != nil {
		panic(err)
	}
}

func handleEOF(err error, ch chan error) {
	if err == io.EOF {
		close(ch)
		panic(nil)
	}
}

func recoverHandle(ch chan error) {
	if rec := recover(); rec != nil {
		err := rec.(error)
		ch <- err
	}
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

	//
	go func() {
		defer recoverHandle(wait1)

		for {
			buf := make([]byte, 768)
			i, err := forwardConn.Read(buf)
			handleEOF(err, wait1)
			handleErr(err)

			var payload pb.Payload
			payload.Data = buf[:i]

			err = stream.Send(&payload)
			handleEOF(err, wait1)
			handleErr(err)
		}
	}()

	go func() {
		defer recoverHandle(wait2)
		writeBuff := bufio.NewWriter(forwardConn)

		for {
			payload, err := stream.Recv()
			handleEOF(err, wait2)
			handleErr(err)

			data := payload.GetData()
			_, err = writeBuff.Write(data)
			handleEOF(err, wait2)
			handleErr(err)

			err = writeBuff.Flush()
			handleEOF(err, wait2)
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

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stream, err := client.Stream(ctx)
	if err != nil {
		log.Println(err)
		return
	}
	defer stream.CloseSend()

	//
	go func() {
		defer recoverHandle(wait1)

		for {
			buf := make([]byte, 768)
			i, err := conn.Read(buf)
			handleEOF(err, wait1)
			handleErr(err)

			var payload pb.Payload
			payload.Data = buf[:i]

			err = stream.Send(&payload)
			handleEOF(err, wait1)
			handleErr(err)
		}
	}()

	go func() {
		defer recoverHandle(wait2)
		writeBuff := bufio.NewWriter(conn)

		for {
			payload, err := stream.Recv()
			handleEOF(err, wait2)
			handleErr(err)

			data := payload.GetData()
			_, err = writeBuff.Write(data)
			handleEOF(err, wait2)
			handleErr(err)

			err = writeBuff.Flush()
			handleEOF(err, wait2)
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
