package transport

import (
	"bufio"
	"context"
	"io"
	"log"
	"net"
	"time"

	pb "github.com/Randomsock5/tcptunnel/proto"
)

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
		for {
			buf := make([]byte, 64)
			i, err := forwardConn.Read(buf)
			if err != io.EOF {
				close(wait1)
				return
			}

			if err != nil {
				wait1 <- err
				return
			}

			var payload pb.Payload
			payload.Data = buf[:i]

			stream.Send(&payload)
		}
	}()

	go func() {
		writeBuff := bufio.NewWriter(forwardConn)

		for {
			payload, err := stream.Recv()
			if err != io.EOF {
				close(wait2)
			}
			if err != nil {
				wait2 <- err
				return
			}

			data := payload.GetData()
			_, err = writeBuff.Write(data)

			if err != io.EOF {
				close(wait2)
			}
			if err != nil {
				wait2 <- err
				return
			}
		}
	}()

	select {
	case err = <-wait1:
		if err != nil {
			return err
		}

	case err = <-wait2:
		if err != nil {
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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	stream, err := client.Stream(ctx)

	//
	go func() {
		for {
			buf := make([]byte, 64)
			i, err := conn.Read(buf)
			if err != io.EOF {
				close(wait1)
				return
			}

			if err != nil {
				wait1 <- err
				return
			}

			var payload pb.Payload
			payload.Data = buf[:i]

			stream.Send(&payload)
		}
	}()

	go func() {
		writeBuff := bufio.NewWriter(conn)

		for {
			payload, err := stream.Recv()
			if err != io.EOF {
				close(wait2)
			}
			if err != nil {
				wait2 <- err
				return
			}

			data := payload.GetData()
			_, err = writeBuff.Write(data)

			if err != io.EOF {
				close(wait2)
			}
			if err != nil {
				wait2 <- err
				return
			}
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
