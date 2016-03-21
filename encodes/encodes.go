package encodes

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"net"
	"time"
)

const (
	IVLength = 1400
)

type AESConn struct {
	conn net.Conn
	r    *cipher.StreamReader
	w    *cipher.StreamWriter
}

func (ac *AESConn) Read(dst []byte) (n int, err error) {
	n, err = ac.r.Read(dst)
	return n, err
}

func (ac *AESConn) Write(src []byte) (n int, err error) {
	n, err = ac.w.Write(src)
	return n, err
}

func (ac *AESConn) Close() error {
	return ac.conn.Close()
}

func (ac *AESConn) LocalAddr() net.Addr {
	return ac.conn.LocalAddr()
}

func (ac *AESConn) RemoteAddr() net.Addr {
	return ac.conn.RemoteAddr()
}

func (ac *AESConn) SetDeadline(t time.Time) error {
	return ac.conn.SetDeadline(t)
}

func (ac *AESConn) SetReadDeadline(t time.Time) error {
	return ac.conn.SetReadDeadline(t)
}

func (ac *AESConn) SetWriteDeadline(t time.Time) error {
	return ac.conn.SetWriteDeadline(t)
}

func NewAESConn(key string, iv [aes.BlockSize]byte, conn net.Conn) (aesconn *AESConn, err error) {
	keybin := []byte(key)

	hashkey := sha256.Sum256(keybin)
	salt := sha256.Sum256([]byte(fmt.Sprintf("%x", key)))

	hashsalt := append(hashkey[:], salt[:]...)

	aeskey := sha256.Sum256(hashsalt)

	rblock, err := aes.NewCipher(aeskey[:])
	wblock, err := aes.NewCipher(aeskey[:])
	if err != nil {
		return nil, err
	}

	rstream := cipher.NewOFB(rblock, iv[:])
	wstream := cipher.NewOFB(wblock, iv[:])

	aesconn = &AESConn{
		conn: conn,
		r:    &cipher.StreamReader{S: rstream, R: conn},
		w:    &cipher.StreamWriter{S: wstream, W: conn},
	}
	return aesconn, err
}

type Listener struct {
	netl net.Listener
	key  string
}

func (l *Listener) Accept() (aesconn net.Conn, err error) {
	conn, err := l.netl.Accept()
	if err != nil {
		return nil, err
	}

	ivvector := make([]byte, IVLength)
	var ri = 0
	for ri < IVLength {
		conn.SetReadDeadline(time.Now().Add(30 * time.Microsecond))
		r, err := conn.Read(ivvector[ri:])
		if err != nil {
			return nil, err
		}
		ri += r
	}

	passwd := sha256.Sum256([]byte(l.key))
	longiv := append(passwd[:], ivvector...)

	hash := sha256.Sum256(longiv)

	var iv [aes.BlockSize]byte
	copy(iv[:], hash[:aes.BlockSize])

	aesconn, err = NewAESConn(l.key, iv, conn)

	return aesconn, err
}

func (l *Listener) Close() error {
	return l.netl.Close()
}

func (l *Listener) Addr() net.Addr {
	return l.netl.Addr()
}

func Listen(laddr string, key string) (l *Listener, err error) {
	netl, err := net.Listen("tcp", laddr)
	if err != nil {
		return nil, err
	}

	l = &Listener{
		netl: netl,
		key:  key,
	}
	return l, nil
}

func Dial(address string, key string) (aesconn net.Conn, err error) {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, err
	}

	ivvector := make([]byte, IVLength)
	_, err = rand.Read(ivvector[:])

	if err != nil {
		return nil, err
	}

	var wi = 0
	for wi < IVLength {
		r, err := conn.Write(ivvector[wi:])
		if err != nil {
			return nil, err
		}
		wi += r
	}

	passwd := sha256.Sum256([]byte(key))
	longiv := append(passwd[:], ivvector...)

	hash := sha256.Sum256(longiv)

	var iv [aes.BlockSize]byte
	copy(iv[:], hash[:aes.BlockSize])

	aesconn, err = NewAESConn(key, iv, conn)
	return aesconn, err
}
