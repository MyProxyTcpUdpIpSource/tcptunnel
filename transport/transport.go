package transport

import (
	"bytes"
	"context"
	"io"
	"log"
	"math/rand"
	"net"
	"time"

	pb "github.com/Randomsock5/tcptunnel/proto"

	"sync"
)

const buffSize = 4096

func handleErr(err error) {
	if err != nil {
		panic(err)
	}
}

func recoverHandle() {
	if rec := recover(); rec != nil {
		err := rec.(error)
		if err != io.EOF {
			log.Println(err)
		}
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
			buf := make([]byte, buffSize)
			i, err := forwardConn.Read(buf)
			handleErr(err)

			var payload pb.Payload
			payload.Data = buf[:i]
			payload.Flag = pb.Payload_Load

			err = stream.Send(&payload)
			handleErr(err)
		}
	}()

	go func() {
		defer wg.Done()
		defer recoverHandle()

		for {
			payload, err := stream.Recv()
			handleErr(err)

			if payload.GetFlag() == pb.Payload_Load {
				data := payload.GetData()
				buf := bytes.NewBuffer(data)
				_, err = io.CopyN(forwardConn, buf, int64(len(data)))
				handleErr(err)
			}
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
		defer wg.Done()
		defer recoverHandle()

		for {
			buf := make([]byte, buffSize)
			i, err := conn.Read(buf)
			handleErr(err)

			var payload pb.Payload
			payload.Data = buf[:i]
			payload.Flag = pb.Payload_Load

			err = stream.Send(&payload)
			handleErr(err)
		}
	}()

	go func() {
		defer wg.Done()
		defer recoverHandle()

		for {
			payload, err := stream.Recv()
			handleErr(err)

			if payload.GetFlag() == pb.Payload_Load {
				data := payload.GetData()
				buf := bytes.NewBuffer(data)

				_, err = io.CopyN(conn, buf, int64(len(data)))
				handleErr(err)

				ackData := make([]byte, rand.Intn(127)+1)
				rand.Read(ackData)

				var ack pb.Payload

				ack.Data = ackData
				ack.Flag = pb.Payload_ACK

				stream.SendMsg(&ack)
			}
		}
	}()

	wg.Add(2)
	wg.Wait()

	return
}
