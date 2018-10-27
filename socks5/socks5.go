package socks5

import (
	"fmt"
	"net"
  "log"
)

const (
	socks5Version = uint8(5)
  NoAuth        = uint8(0)
)

type Server struct {
}

// ListenAndServe is used to create a listener and serve on it
func (s *Server) ListenAndServe(addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.Serve(l)
}

// Serve is used to serve connections from a listener
func (s *Server) Serve(l net.Listener) error {
	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		go s.ServeConn(conn)
	}
	return nil
}


func (s *Server) ServeConn(conn net.Conn) error {
	defer conn.Close()

	version := []byte{0}
	if _, err := conn.Read(version); err != nil {
		log.Printf(fmt.Sprintf("[ERR]: %v \n", err))
		return err
	}

	if version[0] != socks5Version {
		err := fmt.Errorf("Unsupported SOCKS version: %v", version)
		log.Printf("[ERR]: %s \n", err)
		return err
	}

	// NoAuth
	_, err := conn.Write([]byte{socks5Version, NoAuth})
	if err != nil {
    log.Println(err)
		return err
	}

  request, err := NewRequest(conn)
  if err != nil {
    return err
  }

	return request.HandleConnect(conn)
}
