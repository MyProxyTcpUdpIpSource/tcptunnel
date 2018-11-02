package transport

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/Randomsock5/tcptunnel/constants"
)

type AESConn struct {
	conn net.Conn
	r    *cipher.StreamReader
	w    *cipher.StreamWriter
}

func (ac *AESConn) Read(dst []byte) (int, error) {
	n, err := ac.r.Read(dst)
	return n, err
}

func (ac *AESConn) Write(src []byte) (int, error) {
	n, err := ac.w.Write(src)
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

func NewAESConn(key string, iv [aes.BlockSize]byte, conn net.Conn) (*AESConn, error) {
	keyBin := []byte(key)
	t := time.Now().UTC()

	hashKey := sha256.Sum256(keyBin)
	salt := sha256.Sum256(
		[]byte(fmt.Sprintf("%d-%02d-%02d", t.Year(), t.Month(), t.Day())))

	hashSalt := append(hashKey[:], salt[:]...)

	aesKey := sha256.Sum256(hashSalt)

	rBlock, err := aes.NewCipher(aesKey[:])
	if err != nil {
		return nil, err
	}

	wBlock, err := aes.NewCipher(aesKey[:])
	if err != nil {
		return nil, err
	}

	rStream := cipher.NewOFB(rBlock, iv[:])
	wStream := cipher.NewOFB(wBlock, iv[:])

	aesConn := &AESConn{
		conn: conn,
		r:    &cipher.StreamReader{S: rStream, R: conn},
		w:    &cipher.StreamWriter{S: wStream, W: conn},
	}
	return aesConn, err
}

type Listener struct {
	listener net.Listener
	key      string
}

func (l *Listener) Accept() (net.Conn, error) {
	conn, err := l.listener.Accept()
	if err != nil {
		return nil, err
	}
	err = conn.SetReadDeadline(time.Now().Add(constants.ConnTimeout))
	if err != nil {
		return nil, err
	}

	ivVector := make([]byte, constants.IVLength)
	if _, err := io.ReadFull(conn, ivVector); err != nil {
		log.Fatal(err)
	}

	password := sha256.Sum256([]byte(l.key))
	longIv := append(password[:], ivVector...)

	hash := sha256.Sum256(longIv)

	var iv [aes.BlockSize]byte
	copy(iv[:], hash[:aes.BlockSize])

	aesConn, err := NewAESConn(l.key, iv, conn)
	return aesConn, err
}

func (l *Listener) Close() error {
	return l.listener.Close()
}

func (l *Listener) Addr() net.Addr {
	return l.listener.Addr()
}

func Listen(listenAddress string, key string) (*Listener, error) {
	listen, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return nil, err
	}

	l := &Listener{
		listener: listen,
		key:      key,
	}
	return l, nil
}

func Dial(address string, key string, timeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, err
	}

	ivVector := make([]byte, constants.IVLength)
	_, err = rand.Read(ivVector[:])

	if err != nil {
		return nil, err
	}

	password := sha256.Sum256([]byte(key))
	longIv := append(password[:], ivVector...)
	hash := sha256.Sum256(longIv)

	buf := bytes.NewBuffer(ivVector)
	if _, err := io.CopyN(conn, buf, constants.IVLength); err != nil {
		log.Fatal(err)
	}

	var iv [aes.BlockSize]byte
	copy(iv[:], hash[:aes.BlockSize])

	aesConn, err := NewAESConn(key, iv, conn)
	return aesConn, err
}
