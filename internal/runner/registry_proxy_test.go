package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRegistryTunnelProxyAllowsOnlyPinnedHTTPSConnectHosts(t *testing.T) {
	dialer := &memoryRegistryDialer{serve: func(connection net.Conn) {
		defer connection.Close()
		payload := make([]byte, 4)
		if _, err := io.ReadFull(connection, payload); err != nil {
			return
		}
		_, _ = connection.Write([]byte("PONG"))
	}}
	proxy, err := StartRegistryTunnelProxy(context.Background(), []string{"registry.npmjs.org", "REGISTRY.NPMJS.ORG."}, dialer)
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	parsedProxyURL, parseErr := url.Parse(proxy.URL())
	if parseErr != nil || parsedProxyURL.User == nil || parsedProxyURL.User.Username() == "" || parsedProxyURL.Hostname() != "127.0.0.1" {
		t.Fatalf("proxy URL is missing lease credentials or is not loopback-only: %q parseErr=%v", proxy.URL(), parseErr)
	}

	client, reader, status := proxyRequestWithAuth(t, proxy, "CONNECT REGISTRY.NPMJS.ORG:443 HTTP/1.1\r\nHost: REGISTRY.NPMJS.ORG:443\r\n\r\n")
	defer client.Close()
	if status != "HTTP/1.1 200 Connection Established\r\n" {
		t.Fatalf("registry CONNECT status = %q", status)
	}
	if _, err = client.Write([]byte("PING")); err != nil {
		t.Fatal(err)
	}
	response := make([]byte, 4)
	if _, err = io.ReadFull(reader, response); err != nil {
		t.Fatal(err)
	}
	if string(response) != "PONG" || dialer.lastTarget() != "registry.npmjs.org:443" {
		t.Fatalf("registry tunnel response=%q target=%q", response, dialer.lastTarget())
	}
}

func TestRegistryTunnelProxyRejectsUnlistedHostsPortsAndCredentials(t *testing.T) {
	dialer := &memoryRegistryDialer{serve: func(connection net.Conn) { _ = connection.Close() }}
	proxy, err := StartRegistryTunnelProxy(context.Background(), []string{"registry.npmjs.org"}, dialer)
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()

	for _, test := range []struct {
		name    string
		request string
	}{
		{name: "unlisted host", request: "CONNECT evil.example:443 HTTP/1.1\r\nHost: evil.example:443\r\n\r\n"},
		{name: "subdomain is not exact allowlist match", request: "CONNECT mirror.registry.npmjs.org:443 HTTP/1.1\r\nHost: mirror.registry.npmjs.org:443\r\n\r\n"},
		{name: "non TLS port", request: "CONNECT registry.npmjs.org:80 HTTP/1.1\r\nHost: registry.npmjs.org:80\r\n\r\n"},
		{name: "absolute form target", request: "CONNECT http://registry.npmjs.org:443 HTTP/1.1\r\nHost: registry.npmjs.org:443\r\n\r\n"},
		{name: "IP literal", request: "CONNECT 127.0.0.1:443 HTTP/1.1\r\nHost: 127.0.0.1:443\r\n\r\n"},
		{name: "proxy credentials", request: "CONNECT registry.npmjs.org:443 HTTP/1.1\r\nHost: registry.npmjs.org:443\r\nProxy-Authorization: Basic c2VjcmV0\r\n\r\n"},
		{name: "too many duplicate fields", request: repeatedRegistryProxyHeaders(17)},
		{name: "oversized header is rejected during parsing", request: "CONNECT registry.npmjs.org:443 HTTP/1.1\r\nHost: registry.npmjs.org:443\r\nX-Oversized: " + strings.Repeat("x", registryProxyMaxHeaderBytes) + "\r\n\r\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, reader, status := proxyRequestWithAuth(t, proxy, test.request)
			defer client.Close()
			if status != "HTTP/1.1 403 Forbidden\r\n" {
				t.Fatalf("rejected CONNECT status = %q", status)
			}
			if _, readErr := reader.ReadByte(); readErr == nil {
				t.Fatal("rejected CONNECT connection remained open")
			} else if timeout, ok := readErr.(net.Error); ok && timeout.Timeout() {
				t.Fatalf("rejected CONNECT connection did not close: %v", readErr)
			}
		})
	}
	if calls := dialer.callCount(); calls != 0 {
		t.Fatalf("rejected CONNECT requests reached upstream dialer %d times", calls)
	}
}

func TestRegistryTunnelProxyRequiresPerLeaseProxyAuthorization(t *testing.T) {
	dialer := &memoryRegistryDialer{serve: func(connection net.Conn) { _ = connection.Close() }}
	proxy, err := StartRegistryTunnelProxy(context.Background(), []string{"registry.npmjs.org"}, dialer)
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	request := "CONNECT registry.npmjs.org:443 HTTP/1.1\r\nHost: registry.npmjs.org:443\r\n%s\r\n"
	for _, test := range []struct {
		name       string
		authorize  func() string
		wantStatus string
	}{
		{name: "missing credentials", authorize: func() string { return "" }, wantStatus: "HTTP/1.1 403 Forbidden\r\n"},
		{name: "wrong credentials", authorize: func() string { return "Proxy-Authorization: Basic c2VjcmV0\r\n" }, wantStatus: "HTTP/1.1 403 Forbidden\r\n"},
		{name: "lease credentials", authorize: func() string { return "Proxy-Authorization: " + registryProxyAuthorizationHeader(proxy) + "\r\n" }, wantStatus: "HTTP/1.1 200 Connection Established\r\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			client, _, status := proxyRequest(t, proxy.Addr(), fmt.Sprintf(request, test.authorize()))
			defer client.Close()
			if status != test.wantStatus {
				t.Fatalf("CONNECT status = %q, want %q", status, test.wantStatus)
			}
		})
	}
	if calls := dialer.callCount(); calls != 1 {
		t.Fatalf("unauthorized requests reached the upstream dialer: calls=%d, want 1", calls)
	}
}

func TestRegistryProxyCountingReaderBoundsBytesConsumed(t *testing.T) {
	content := bytes.Repeat([]byte{'x'}, registryProxyMaxHeaderBytes*4)
	reader := &registryProxyCountingReader{reader: bytes.NewReader(content), limit: registryProxyMaxHeaderBytes}
	consumed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if len(consumed) != registryProxyMaxHeaderBytes || reader.read != registryProxyMaxHeaderBytes {
		t.Fatalf("bounded header reader consumed bytes=%d count=%d, want %d", len(consumed), reader.read, registryProxyMaxHeaderBytes)
	}
}

func TestRegistryTunnelProxyHeaderFieldLimitIncludesHost(t *testing.T) {
	for _, test := range []struct {
		extraFields int
		wantStatus  string
		wantDials   int
	}{
		{extraFields: registryProxyMaxHeaderCount - 2, wantStatus: "HTTP/1.1 200 Connection Established\r\n", wantDials: 1},
		{extraFields: registryProxyMaxHeaderCount - 1, wantStatus: "HTTP/1.1 403 Forbidden\r\n", wantDials: 0},
	} {
		t.Run(fmt.Sprint(test.extraFields), func(t *testing.T) {
			dialer := &memoryRegistryDialer{serve: func(connection net.Conn) { _ = connection.Close() }}
			proxy, err := StartRegistryTunnelProxy(context.Background(), []string{"registry.npmjs.org"}, dialer)
			if err != nil {
				t.Fatal(err)
			}
			defer proxy.Close()
			client, reader, status := proxyRequestWithAuth(t, proxy, registryProxyHeaderRequest(test.extraFields))
			defer client.Close()
			if status != test.wantStatus {
				t.Fatalf("CONNECT with %d non-Host headers status = %q, want %q", test.extraFields, status, test.wantStatus)
			}
			if dialer.callCount() != test.wantDials {
				t.Fatalf("upstream dial count = %d, want %d", dialer.callCount(), test.wantDials)
			}
			if test.wantDials == 0 {
				if _, readErr := reader.ReadByte(); readErr == nil {
					t.Fatal("rejected CONNECT connection remained open")
				} else if timeout, ok := readErr.(net.Error); ok && timeout.Timeout() {
					t.Fatalf("rejected CONNECT connection did not close: %v", readErr)
				}
			}
		})
	}
}

func TestRegistryTunnelProxyRequiresCanonicalDNSAllowlist(t *testing.T) {
	for _, hosts := range [][]string{
		nil,
		{},
		{""},
		{"https://registry.npmjs.org"},
		{"127.0.0.1"},
		{"127.1"},
		{"2130706433"},
		{"0x7f000001"},
		{"0x0a.1"},
		{"0177.1"},
		{"localhost"},
		{"registry.localhost"},
		{"*.npmjs.org"},
	} {
		t.Run(fmt.Sprint(hosts), func(t *testing.T) {
			if _, err := StartRegistryTunnelProxy(context.Background(), hosts, &memoryRegistryDialer{}); err == nil {
				t.Fatalf("accepted registry allowlist %q", hosts)
			}
		})
	}
}

func TestRegistryTunnelProxyCloseTerminatesActiveLoopbackTunnels(t *testing.T) {
	upstreamClosed := make(chan struct{})
	dialer := &memoryRegistryDialer{serve: func(connection net.Conn) {
		defer close(upstreamClosed)
		defer connection.Close()
		_, _ = io.Copy(io.Discard, connection)
	}}
	proxy, err := StartRegistryTunnelProxy(context.Background(), []string{"registry.npmjs.org"}, dialer)
	if err != nil {
		t.Fatal(err)
	}
	client, _, status := proxyRequestWithAuth(t, proxy, "CONNECT registry.npmjs.org:443 HTTP/1.1\r\nHost: registry.npmjs.org:443\r\n\r\n")
	if status != "HTTP/1.1 200 Connection Established\r\n" {
		_ = client.Close()
		_ = proxy.Close()
		t.Fatalf("registry CONNECT status = %q", status)
	}
	if err = proxy.Close(); err != nil {
		_ = client.Close()
		t.Fatal(err)
	}
	defer client.Close()
	select {
	case <-upstreamClosed:
	case <-time.After(2 * time.Second):
		t.Fatal("closing the proxy left its upstream tunnel open")
	}
	if _, err = net.DialTimeout("tcp", proxy.Addr().String(), 50*time.Millisecond); err == nil {
		t.Fatal("proxy listener accepted a connection after close")
	}
}

func TestRegistryTunnelProxyCapsConcurrentTunnels(t *testing.T) {
	dialer := &memoryRegistryDialer{serve: func(connection net.Conn) {
		defer connection.Close()
		_, _ = io.Copy(io.Discard, connection)
	}}
	proxy, err := StartRegistryTunnelProxy(context.Background(), []string{"registry.npmjs.org"}, dialer)
	if err != nil {
		t.Fatal(err)
	}
	clients := make([]net.Conn, 0, registryProxyMaxConnections)
	for range registryProxyMaxConnections {
		client, _, status := proxyRequestWithAuth(t, proxy, "CONNECT registry.npmjs.org:443 HTTP/1.1\r\nHost: registry.npmjs.org:443\r\n\r\n")
		if status != "HTTP/1.1 200 Connection Established\r\n" {
			_ = client.Close()
			_ = proxy.Close()
			t.Fatalf("under-limit CONNECT status = %q", status)
		}
		clients = append(clients, client)
	}
	client, _, status := proxyRequestWithAuth(t, proxy, "CONNECT registry.npmjs.org:443 HTTP/1.1\r\nHost: registry.npmjs.org:443\r\n\r\n")
	defer client.Close()
	if status != "HTTP/1.1 503 Service Unavailable\r\n" {
		_ = proxy.Close()
		t.Fatalf("over-limit CONNECT status = %q", status)
	}
	if calls := dialer.callCount(); calls != registryProxyMaxConnections {
		_ = proxy.Close()
		t.Fatalf("upstream dial count = %d, want %d", calls, registryProxyMaxConnections)
	}
	for _, connection := range clients {
		_ = connection.Close()
	}
	if err = proxy.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryTunnelProxyClosesWhenBoundedRejectionQueueIsFull(t *testing.T) {
	dialer := &memoryRegistryDialer{serve: func(connection net.Conn) {
		defer connection.Close()
		_, _ = io.Copy(io.Discard, connection)
	}}
	proxy, err := StartRegistryTunnelProxy(context.Background(), []string{"registry.npmjs.org"}, dialer)
	if err != nil {
		t.Fatal(err)
	}
	clients := make([]net.Conn, 0, registryProxyMaxConnections+registryProxyMaxRejections+1)
	t.Cleanup(func() {
		_ = proxy.Close()
		for _, client := range clients {
			_ = client.Close()
		}
	})
	request := "CONNECT registry.npmjs.org:443 HTTP/1.1\r\nHost: registry.npmjs.org:443\r\n\r\n"
	for range registryProxyMaxConnections {
		client, _, status := proxyRequestWithAuth(t, proxy, request)
		if status != "HTTP/1.1 200 Connection Established\r\n" {
			t.Fatalf("under-limit CONNECT status = %q", status)
		}
		clients = append(clients, client)
	}
	for index := 0; index < registryProxyMaxRejections; index++ {
		client, dialErr := net.DialTimeout("tcp", proxy.Addr().String(), time.Second)
		if dialErr != nil {
			t.Fatal(dialErr)
		}
		clients = append(clients, client)
		if _, err = io.WriteString(client, registryProxyAuthenticatedHeaderPrefix(proxy)); err != nil {
			t.Fatal(err)
		}
		waitForRegistryProxyRejections(t, proxy, index+1)
	}

	excess, err := net.DialTimeout("tcp", proxy.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	clients = append(clients, excess)
	if _, err = io.WriteString(excess, registryProxyAuthenticatedHeaderPrefix(proxy)); err != nil {
		t.Fatal(err)
	}
	if err = excess.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err = excess.Read(make([]byte, 1)); err == nil {
		t.Fatal("proxy returned data for a client rejected after the bounded queue filled")
	} else {
		var networkError net.Error
		if errors.As(err, &networkError) && networkError.Timeout() {
			t.Fatalf("proxy left an excess overflow connection open after the rejection queue filled: %v", err)
		}
	}
}

func TestRegistryTunnelProxyDoesNotReturnOverloadStatusForInvalidRequest(t *testing.T) {
	dialer := &memoryRegistryDialer{serve: func(connection net.Conn) {
		defer connection.Close()
		_, _ = io.Copy(io.Discard, connection)
	}}
	proxy, err := StartRegistryTunnelProxy(context.Background(), []string{"registry.npmjs.org"}, dialer)
	if err != nil {
		t.Fatal(err)
	}
	clients := make([]net.Conn, 0, registryProxyMaxConnections+1)
	t.Cleanup(func() {
		_ = proxy.Close()
		for _, client := range clients {
			_ = client.Close()
		}
	})
	request := "CONNECT registry.npmjs.org:443 HTTP/1.1\r\nHost: registry.npmjs.org:443\r\n\r\n"
	for range registryProxyMaxConnections {
		client, _, status := proxyRequestWithAuth(t, proxy, request)
		if status != "HTTP/1.1 200 Connection Established\r\n" {
			t.Fatalf("under-limit CONNECT status = %q", status)
		}
		clients = append(clients, client)
	}
	invalidRequests := map[string]string{
		"non-CONNECT": "GET / HTTP/1.1\r\nHost: registry.npmjs.org\r\n\r\n",
		"body":        "CONNECT registry.npmjs.org:443 HTTP/1.1\r\nHost: registry.npmjs.org:443\r\nContent-Length: 4\r\n\r\nDATA",
		"extra bytes": "CONNECT registry.npmjs.org:443 HTTP/1.1\r\nHost: registry.npmjs.org:443\r\n\r\nDATA",
	}
	for name, request := range invalidRequests {
		t.Run(name, func(t *testing.T) {
			overflow, err := net.DialTimeout("tcp", proxy.Addr().String(), time.Second)
			if err != nil {
				t.Fatal(err)
			}
			clients = append(clients, overflow)
			if err = overflow.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if _, err = io.WriteString(overflow, registryProxyAuthenticatedRequest(proxy, request)); err != nil {
				t.Fatal(err)
			}
			status, readErr := bufio.NewReader(overflow).ReadString('\n')
			if readErr == nil {
				t.Fatalf("overload path returned status %q for an invalid CONNECT request", status)
			}
			var networkError net.Error
			if errors.As(readErr, &networkError) && networkError.Timeout() {
				t.Fatalf("invalid overflow request remained open: %v", readErr)
			}
		})
	}
}

func waitForRegistryProxyRejections(t *testing.T, proxy *RegistryTunnelProxy, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for len(proxy.rejects) < want {
		if time.Now().After(deadline) {
			t.Fatalf("queued rejection handlers = %d, want at least %d", len(proxy.rejects), want)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestRegistryTunnelProxyCancellationStopsPendingDial(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	dialer := &blockingRegistryDialer{started: make(chan struct{}), canceled: make(chan struct{})}
	proxy, err := StartRegistryTunnelProxy(ctx, []string{"registry.npmjs.org"}, dialer)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	client, err := net.Dial("tcp", proxy.Addr().String())
	if err != nil {
		cancel()
		_ = proxy.Close()
		t.Fatal(err)
	}
	if err = client.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		cancel()
		_ = client.Close()
		_ = proxy.Close()
		t.Fatal(err)
	}
	if _, err = io.WriteString(client, registryProxyAuthenticatedRequest(proxy, "CONNECT registry.npmjs.org:443 HTTP/1.1\r\nHost: registry.npmjs.org:443\r\n\r\n")); err != nil {
		cancel()
		_ = client.Close()
		_ = proxy.Close()
		t.Fatal(err)
	}
	select {
	case <-dialer.started:
	case <-time.After(2 * time.Second):
		cancel()
		_ = client.Close()
		_ = proxy.Close()
		t.Fatal("proxy did not start the injected dial")
	}
	cancel()
	if err = proxy.Close(); err != nil {
		_ = client.Close()
		t.Fatal(err)
	}
	select {
	case <-dialer.canceled:
	case <-time.After(2 * time.Second):
		_ = client.Close()
		t.Fatal("proxy shutdown did not cancel the pending dial")
	}
	_ = client.Close()
}

func proxyRequest(t *testing.T, address net.Addr, request string) (net.Conn, *bufio.Reader, string) {
	t.Helper()
	client, err := net.Dial("tcp", address.String())
	if err != nil {
		t.Fatal(err)
	}
	if err = client.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
		_ = client.Close()
		t.Fatal(err)
	}
	if _, err = io.WriteString(client, request); err != nil {
		_ = client.Close()
		t.Fatal(err)
	}
	reader := bufio.NewReader(client)
	status, err := reader.ReadString('\n')
	if err != nil {
		_ = client.Close()
		t.Fatal(err)
	}
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			_ = client.Close()
			t.Fatal(readErr)
		}
		if line == "\r\n" {
			break
		}
	}
	return client, reader, status
}

func proxyRequestWithAuth(t *testing.T, proxy *RegistryTunnelProxy, request string) (net.Conn, *bufio.Reader, string) {
	t.Helper()
	return proxyRequest(t, proxy.Addr(), registryProxyAuthenticatedRequest(proxy, request))
}

func registryProxyAuthenticatedRequest(proxy *RegistryTunnelProxy, request string) string {
	return strings.Replace(request, "\r\n\r\n", "\r\nProxy-Authorization: "+registryProxyAuthorizationHeader(proxy)+"\r\n\r\n", 1)
}

func registryProxyAuthenticatedHeaderPrefix(proxy *RegistryTunnelProxy) string {
	return "CONNECT registry.npmjs.org:443 HTTP/1.1\r\nHost: registry.npmjs.org:443\r\nProxy-Authorization: " + registryProxyAuthorizationHeader(proxy) + "\r\n"
}

func registryProxyAuthorizationHeader(proxy *RegistryTunnelProxy) string {
	parsed, _ := url.Parse(proxy.URL())
	username := ""
	password := ""
	if parsed != nil && parsed.User != nil {
		username = parsed.User.Username()
		password, _ = parsed.User.Password()
	}
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
}

func repeatedRegistryProxyHeaders(count int) string {
	var request strings.Builder
	request.WriteString("CONNECT registry.npmjs.org:443 HTTP/1.1\r\nHost: registry.npmjs.org:443\r\n")
	for range count {
		request.WriteString("X-First: a\r\n")
		request.WriteString("X-Second: b\r\n")
	}
	request.WriteString("\r\n")
	return request.String()
}

func registryProxyHeaderRequest(extraFields int) string {
	var request strings.Builder
	request.WriteString("CONNECT registry.npmjs.org:443 HTTP/1.1\r\nHost: registry.npmjs.org:443\r\n")
	for range extraFields {
		request.WriteString("X-Extra: value\r\n")
	}
	request.WriteString("\r\n")
	return request.String()
}

type memoryRegistryDialer struct {
	mu     sync.Mutex
	calls  int
	target string
	serve  func(net.Conn)
}

type blockingRegistryDialer struct {
	started  chan struct{}
	canceled chan struct{}
	once     sync.Once
}

func (d *blockingRegistryDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	d.once.Do(func() { close(d.started) })
	<-ctx.Done()
	close(d.canceled)
	return nil, ctx.Err()
}

func (d *memoryRegistryDialer) DialContext(_ context.Context, network, target string) (net.Conn, error) {
	d.mu.Lock()
	d.calls++
	d.target = target
	serve := d.serve
	d.mu.Unlock()
	if network != "tcp" {
		return nil, fmt.Errorf("unexpected network %q", network)
	}
	client, upstream := net.Pipe()
	if serve != nil {
		go serve(upstream)
	} else {
		_ = upstream.Close()
	}
	return client, nil
}

func (d *memoryRegistryDialer) callCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.calls
}

func (d *memoryRegistryDialer) lastTarget() string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return strings.ToLower(d.target)
}
