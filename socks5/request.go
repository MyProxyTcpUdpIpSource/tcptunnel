package socks5

import (
	"fmt"
	"net"
  "io"

  "strconv"
)

const (
  ConnectCommand   = uint8(1)
  BindCommand      = uint8(2)
  AssociateCommand = uint8(3)
  ipv4Address      = uint8(1)
  fqdnAddress      = uint8(3)
  ipv6Address      = uint8(4)
)

const (
	successReply uint8 = iota
	serverFailure
	ruleFailure
	networkUnreachable
	hostUnreachable
	connectionRefused
	ttlExpired
	commandNotSupported
	addrTypeNotSupported
)

var (
	unrecognizedAddrType = fmt.Errorf("Unrecognized address type")
)

type AddrSpec struct {
	FQDN string
	IP   net.IP
	Port int
}

func (a *AddrSpec) String() string {
	if a.FQDN != "" {
		return fmt.Sprintf("%s (%s):%d", a.FQDN, a.IP, a.Port)
	}
	return fmt.Sprintf("%s:%d", a.IP, a.Port)
}

func (a AddrSpec) Address() string {
	if 0 != len(a.IP) {
		return net.JoinHostPort(a.IP.String(), strconv.Itoa(a.Port))
	}
	return net.JoinHostPort(a.FQDN, strconv.Itoa(a.Port))
}

type Request struct {
	DestAddr *AddrSpec
}

func NewRequest(conn io.ReadWriter) (*Request, error) {
	header := []byte{0, 0, 0}
	if _, err := io.ReadAtLeast(conn, header, 3); err != nil {
		return nil, fmt.Errorf("Failed to get command version: %s", err)
	}

	if header[0] != socks5Version {
		return nil, fmt.Errorf("Unsupported command version: %d", header[0])
	}

  if header[1] != ConnectCommand {
    if err := sendReply(conn, commandNotSupported, nil); err != nil {
    		return nil, fmt.Errorf("Failed to send reply: %s", err)
    }
    return nil, fmt.Errorf("Unsupported command: %d", header[1])
  }

	// Read in the destination address
	dest, err := readAddrSpec(conn)
	if err != nil {
		return nil, err
	}

	request := &Request{
		DestAddr: dest,
	}

  if err := sendReply(conn, successReply, dest); err != nil {
		return nil, fmt.Errorf("Failed to send reply: %s", err)
  }

	return request, nil
}

func (req *Request) HandleConnect(conn io.ReadWriter) error {
  forwardConn, err := net.Dial("tcp", req.DestAddr.Address())
  if err != nil {
    return fmt.Errorf("Connect to %v failed: %v", req.DestAddr, err)
  }
  defer forwardConn.Close()

  errCh := make(chan error, 2)
  go proxy(forwardConn, conn, errCh)
  go proxy(conn, forwardConn, errCh)

  for i := 0; i < 2; i++ {
    e := <-errCh
    if e != nil {
      return e
    }
  }

  return nil
}


func readAddrSpec(r io.Reader) (*AddrSpec, error) {
	d := &AddrSpec{}

	// Get the address type
	addrType := []byte{0}
	if _, err := r.Read(addrType); err != nil {
		return nil, err
	}

	switch addrType[0] {
	case ipv4Address:
		addr := make([]byte, 4)
		if _, err := io.ReadAtLeast(r, addr, len(addr)); err != nil {
			return nil, err
		}
		d.IP = net.IP(addr)

	case ipv6Address:
		addr := make([]byte, 16)
		if _, err := io.ReadAtLeast(r, addr, len(addr)); err != nil {
			return nil, err
		}
		d.IP = net.IP(addr)

	case fqdnAddress:
		if _, err := r.Read(addrType); err != nil {
			return nil, err
		}
		addrLen := int(addrType[0])
		fqdn := make([]byte, addrLen)
		if _, err := io.ReadAtLeast(r, fqdn, addrLen); err != nil {
			return nil, err
		}
		d.FQDN = string(fqdn)

	default:
		return nil, unrecognizedAddrType
	}

	port := []byte{0, 0}
	if _, err := io.ReadAtLeast(r, port, 2); err != nil {
		return nil, err
	}
	d.Port = (int(port[0]) << 8) | int(port[1])

	return d, nil
}

func sendReply(w io.Writer, resp uint8, addr *AddrSpec) error {
	// Format the address
	var addrType uint8
	var addrBody []byte
	var addrPort uint16
	switch {
	case addr == nil:
		addrType = ipv4Address
		addrBody = []byte{0, 0, 0, 0}
		addrPort = 0

	case addr.FQDN != "":
		addrType = fqdnAddress
		addrBody = append([]byte{byte(len(addr.FQDN))}, addr.FQDN...)
		addrPort = uint16(addr.Port)

	case addr.IP.To4() != nil:
		addrType = ipv4Address
		addrBody = []byte(addr.IP.To4())
		addrPort = uint16(addr.Port)

	case addr.IP.To16() != nil:
		addrType = ipv6Address
		addrBody = []byte(addr.IP.To16())
		addrPort = uint16(addr.Port)

	default:
		return fmt.Errorf("Failed to format address: %v", addr)
	}

	msg := make([]byte, 6+len(addrBody))
	msg[0] = socks5Version
	msg[1] = resp
	msg[2] = 0
	msg[3] = addrType
	copy(msg[4:], addrBody)
	msg[4+len(addrBody)] = byte(addrPort >> 8)
	msg[4+len(addrBody)+1] = byte(addrPort & 0xff)

	_, err := w.Write(msg)
	return err
}

type closeWriter interface {
	CloseWrite() error
}

func proxy(dst io.Writer, src io.Reader, errCh chan error) {
	_, err := io.Copy(dst, src)
	if tcpConn, ok := dst.(closeWriter); ok {
		tcpConn.CloseWrite()
	}
	errCh <- err
}
