package transport

import (
	"bufio"
	"context"
	pb "github.com/Randomsock5/tcptunnel/proto"
	"log"
	"net"
	"time"

	"sync"
)

func handleErr(err error) {
	if err != nil {
		panic(err)
	}
}

func recoverHandle() {
	if rec := recover(); rec != nil {
		err := rec.(error)
		log.Println(err)
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
	wg := &sync.WaitGroup{}

	go func() {
		defer wg.Done()
		defer recoverHandle()

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
		defer recoverHandle()
		defer wg.Done()
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

	wg.Add(2)
	wg.Wait()

	return nil
}

func NewServer(forward string) pb.ProxyServiceServer {
	s := &proxyService{forward: forward}
	return s
}

func ClientProxyService(conn net.Conn, client pb.ProxyServiceClient) {
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stream, err := client.Stream(ctx)
	if err != nil {
		log.Println(err)
		return
	}
	defer stream.CloseSend()
	wg := &sync.WaitGroup{}

	go func() {
		defer recoverHandle()
		defer wg.Done()

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
		defer recoverHandle()
		defer wg.Done()
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

	wg.Add(2)
	wg.Wait()

	return
}
