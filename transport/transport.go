package transport

import (
	"bytes"
	"context"
	"io"
	"log"
	"math/rand"
	"net"

	"github.com/Randomsock5/tcptunnel/constants"
	pb "github.com/Randomsock5/tcptunnel/proto"
)

const buffSize = 4096

func handleErr(err error, errCh chan error) {
	if err != nil {
		errCh <- err
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

type sendInterface interface {
	Send(*pb.Payload) error
}

func sendACK(stream sendInterface) error {
	ackData := make([]byte, rand.Intn(255)+1)
	rand.Read(ackData)

	var ack pb.Payload

	ack.Data = ackData
	ack.Flag = pb.Payload_ACK

	return stream.Send(&ack)
}

func (s *proxyService) Stream(stream pb.ProxyService_StreamServer) error {
	forwardConn, err := net.DialTimeout("tcp", s.forward, constants.ConnTimeout)
	if err != nil {
		log.Println(err)
		return err
	}
	defer forwardConn.Close()
	errCh := make(chan error, 2)

	go func() {
		defer recoverHandle()

		for {
			buf := make([]byte, buffSize)
			i, err := forwardConn.Read(buf)
			handleErr(err, errCh)

			var payload pb.Payload
			payload.Data = buf[:i]
			payload.Flag = pb.Payload_Load

			err = stream.Send(&payload)
			handleErr(err, errCh)
		}
	}()

	go func() {
		defer recoverHandle()

		for {
			payload, err := stream.Recv()
			handleErr(err, errCh)

			if payload.GetFlag() == pb.Payload_Load {
				data := payload.GetData()
				buf := bytes.NewBuffer(data)
				_, err = io.CopyN(forwardConn, buf, int64(len(data)))
				handleErr(err, errCh)

				err = sendACK(stream)
				handleErr(err, errCh)
			}
		}
	}()

	for i := 0; i < 2; i++ {
		e := <-errCh
		if e != nil {
			return e
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

	ctx, cancel := context.WithTimeout(context.Background(), constants.ConnTimeout)
	defer cancel()

	stream, err := client.Stream(ctx)
	if err != nil {
		log.Println(err)
		return
	}
	defer stream.CloseSend()
	errCh := make(chan error, 2)

	go func() {
		defer recoverHandle()

		for {
			buf := make([]byte, buffSize)
			i, err := conn.Read(buf)
			handleErr(err, errCh)

			var payload pb.Payload
			payload.Data = buf[:i]
			payload.Flag = pb.Payload_Load

			err = stream.Send(&payload)
			handleErr(err, errCh)
		}
	}()

	go func() {
		defer recoverHandle()

		for {
			payload, err := stream.Recv()
			handleErr(err, errCh)

			if payload.GetFlag() == pb.Payload_Load {
				data := payload.GetData()
				buf := bytes.NewBuffer(data)

				_, err = io.CopyN(conn, buf, int64(len(data)))
				handleErr(err, errCh)

				err = sendACK(stream)
				handleErr(err, errCh)
			}
		}
	}()

	for i := 0; i < 2; i++ {
		e := <-errCh
		if e != nil {
			return
		}
	}

	return
}
