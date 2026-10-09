package checker

import (
	"context"
	"net"
	"strconv"
	"testing"

	"github.com/cng1985/nwatch/internal/model"
)

func TestTCPCheckerConnectedAndRefused(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()
	_, portText, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portText)
	c := NewTCPChecker()
	res, err := c.Check(context.Background(), &model.Monitor{Host: "127.0.0.1", Port: port, Timeout: 2})
	if err != nil || !res.Success || res.Status != "CONNECTED" {
		t.Fatalf("connected %+v %v", res, err)
	}

	res, err = c.Check(context.Background(), &model.Monitor{Host: "127.0.0.1", Port: 1, Timeout: 2})
	if err != nil || res.Success || (res.Status != "REFUSED" && res.Status != "TIMEOUT" && res.Status != "ERROR") {
		t.Fatalf("closed port %+v %v", res, err)
	}
}
