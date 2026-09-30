//go:build windows

// pattern: Imperative Shell
package runner

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"
)

func TestVerifyWindowsTCPListenerOwnerMatchesExactLoopbackProcess(t *testing.T) {
	for _, fixture := range []struct {
		name    string
		network string
		address string
	}{
		{name: "ipv4", network: "tcp4", address: "127.0.0.1"},
		{name: "ipv6", network: "tcp6", address: "::1"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			listener, err := net.Listen(fixture.network, net.JoinHostPort(fixture.address, "0"))
			if err != nil {
				if fixture.network == "tcp6" {
					t.Skipf("IPv6 loopback is unavailable: %v", err)
				}
				t.Fatal(err)
			}
			defer listener.Close()
			port := listener.Addr().(*net.TCPAddr).Port

			if err = VerifyWindowsTCPListenerOwner(context.Background(), os.Getpid(), fixture.address, uint16(port)); err != nil {
				t.Fatalf("verify current process listener on %s:%d: %v", fixture.address, port, err)
			}
			if err = VerifyWindowsTCPListenerOwner(context.Background(), os.Getpid()+1000, fixture.address, uint16(port)); !errors.Is(err, ErrTCPListenerOwnerUnverified) {
				t.Fatalf("wrong process accepted as listener owner: %v", err)
			}
			if err = VerifyWindowsTCPListenerOwner(context.Background(), os.Getpid(), fixture.address, uint16(port+1)); !errors.Is(err, ErrTCPListenerOwnerUnverified) {
				t.Fatalf("wrong port accepted as listener owner: %v", err)
			}
		})
	}
}

func TestVerifyWindowsTCPListenerOwnerRejectsNonLoopbackTargets(t *testing.T) {
	if err := VerifyWindowsTCPListenerOwner(context.Background(), os.Getpid(), "0.0.0.0", 80); !errors.Is(err, ErrTCPListenerOwnerUnverified) {
		t.Fatalf("non-loopback target error=%v", err)
	}
}

func TestVerifyWindowsTCPConnectionOwnerBindsServerTupleToProcess(t *testing.T) {
	for _, fixture := range []struct {
		name    string
		network string
		address string
	}{
		{name: "ipv4", network: "tcp4", address: "127.0.0.1"},
		{name: "ipv6", network: "tcp6", address: "::1"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			listener, err := net.Listen(fixture.network, net.JoinHostPort(fixture.address, "0"))
			if err != nil {
				if fixture.network == "tcp6" {
					t.Skipf("IPv6 loopback is unavailable: %v", err)
				}
				t.Fatal(err)
			}
			defer listener.Close()
			accepted := make(chan net.Conn, 1)
			go func() {
				connection, acceptErr := listener.Accept()
				if acceptErr == nil {
					accepted <- connection
				}
			}()
			client, err := net.DialTimeout(fixture.network, listener.Addr().String(), time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			var server net.Conn
			select {
			case server = <-accepted:
			case <-time.After(time.Second):
				t.Fatal("server did not accept local connection")
			}
			defer server.Close()
			local := client.LocalAddr().(*net.TCPAddr)
			remote := client.RemoteAddr().(*net.TCPAddr)
			port := uint16(remote.Port)
			if err = VerifyWindowsTCPConnectionOwner(context.Background(), os.Getpid(), fixture.address, port, local.IP.String(), uint16(local.Port)); err != nil {
				t.Fatalf("verify exact established connection owner: %v", err)
			}
			if err = VerifyWindowsTCPConnectionOwner(context.Background(), os.Getpid()+1000, fixture.address, port, local.IP.String(), uint16(local.Port)); !errors.Is(err, ErrTCPListenerOwnerUnverified) {
				t.Fatalf("wrong process accepted as established connection owner: %v", err)
			}
		})
	}
}
