package socks5

import (
  "testing"
)

func TestSOCKS5_Connect(t *testing.T) {
  server := &Server{}
  server.ListenAndServe("::3980")
}
