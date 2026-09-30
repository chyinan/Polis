// pattern: Imperative Shell
package runner

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// RegistryProxyMaxHosts bounds the externally approved registry host set.
	RegistryProxyMaxHosts       = 8
	registryProxyMaxConnections = 16
	registryProxyMaxRejections  = 16
	registryProxyMaxHeaderBytes = 16 << 10
	registryProxyMaxHeaderCount = 32
	registryProxyHandshakeLimit = 12 * time.Second
	registryProxyDialLimit      = 15 * time.Second
)

var errRegistryProxyPolicy = errors.New("invalid registry proxy policy")

// RegistryTunnelDialer is the only path from the broker to an upstream. Tests
// inject an in-memory dialer so registry policy can be verified without any
// external request.
type RegistryTunnelDialer interface {
	DialContext(context.Context, string, string) (net.Conn, error)
}

// RegistryTunnelProxy accepts authenticated HTTPS CONNECT requests only for
// exact hosts in the approved registry list. The caller must separately
// restrict the AppContainer SID so its only reachable network endpoint is this
// loopback listener; this proxy alone does not prevent a sandbox process from
// opening a direct socket. The registry-only AppContainer constructor pairs
// this broker with a package-SID-scoped WFP lease.
type RegistryTunnelProxy struct {
	listener      net.Listener
	dialer        RegistryTunnelDialer
	hosts         map[string]struct{}
	proxyURL      string
	authorization string
	ctx           context.Context
	cancel        context.CancelFunc

	mu       sync.Mutex
	closed   bool
	active   map[net.Conn]struct{}
	limit    chan struct{}
	rejects  chan struct{}
	closeOne sync.Once
	closedCh chan struct{}
	wg       sync.WaitGroup
}

// StartRegistryTunnelProxy starts a loopback-only CONNECT broker for the
// canonical exact-host allowlist. No DNS or upstream connection happens until
// an approved host is requested. Active tunnels and overflow rejections have
// separate bounds. A queued, header-only CONNECT request for an approved host
// receives 503 after bounded parsing; a full rejection queue, invalid request,
// buffered extra data, or incomplete request is closed without a status line.
func StartRegistryTunnelProxy(ctx context.Context, allowedHosts []string, dialer RegistryTunnelDialer) (*RegistryTunnelProxy, error) {
	if ctx == nil {
		return nil, errRegistryProxyPolicy
	}
	hosts, err := canonicalRegistryProxyHosts(allowedHosts)
	if err != nil {
		return nil, err
	}
	if dialer == nil {
		dialer = newRegistryHostDialer()
	}
	var usernameBytes [18]byte
	var passwordBytes [32]byte
	if _, err := rand.Read(usernameBytes[:]); err != nil {
		return nil, errors.New("registry proxy could not create lease credentials")
	}
	if _, err := rand.Read(passwordBytes[:]); err != nil {
		return nil, errors.New("registry proxy could not create lease credentials")
	}
	username := base64.RawURLEncoding.EncodeToString(usernameBytes[:])
	password := base64.RawURLEncoding.EncodeToString(passwordBytes[:])
	authorization := "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+password))
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, errors.New("registry proxy could not bind to loopback")
	}
	proxyContext, cancel := context.WithCancel(ctx)
	proxyURL := (&url.URL{Scheme: "http", User: url.UserPassword(username, password), Host: listener.Addr().String()}).String()
	proxy := &RegistryTunnelProxy{
		listener: listener, dialer: dialer, hosts: hosts, proxyURL: proxyURL, authorization: authorization, ctx: proxyContext, cancel: cancel,
		active: make(map[net.Conn]struct{}), limit: make(chan struct{}, registryProxyMaxConnections), rejects: make(chan struct{}, registryProxyMaxRejections), closedCh: make(chan struct{}),
	}
	proxy.wg.Add(1)
	go proxy.acceptLoop()
	go func() {
		select {
		case <-ctx.Done():
			proxy.shutdown()
		case <-proxy.closedCh:
		}
	}()
	return proxy, nil
}

func (p *RegistryTunnelProxy) Addr() net.Addr {
	if p == nil || p.listener == nil {
		return nil
	}
	return p.listener.Addr()
}

func (p *RegistryTunnelProxy) isActive() bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.closed && p.listener != nil
}

// URL is suitable for the HTTP_PROXY and HTTPS_PROXY environment variables.
func (p *RegistryTunnelProxy) URL() string {
	if p == nil {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return ""
	}
	return p.proxyURL
}

func (p *RegistryTunnelProxy) Close() error {
	if p == nil {
		return nil
	}
	p.shutdown()
	p.wg.Wait()
	return nil
}

func (p *RegistryTunnelProxy) shutdown() {
	p.closeOne.Do(func() {
		p.cancel()
		p.mu.Lock()
		p.closed = true
		for connection := range p.active {
			_ = connection.Close()
		}
		p.mu.Unlock()
		_ = p.listener.Close()
		close(p.closedCh)
	})
}

func (p *RegistryTunnelProxy) acceptLoop() {
	defer p.wg.Done()
	for {
		client, err := p.listener.Accept()
		if err != nil {
			if p.ctx.Err() != nil {
				return
			}
			continue
		}
		select {
		case p.limit <- struct{}{}:
		default:
			select {
			case p.rejects <- struct{}{}:
			default:
				// Once the bounded rejection queue is full, close without parsing
				// untrusted input or allocating another handler.
				_ = client.Close()
				continue
			}
			if !p.track(client) {
				<-p.rejects
				_ = client.Close()
				return
			}
			p.wg.Add(1)
			go p.rejectBusyClient(client)
			continue
		}
		if !p.track(client) {
			<-p.limit
			_ = client.Close()
			return
		}
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			defer func() { <-p.limit }()
			defer p.untrack(client)
			defer client.Close()
			p.handleClient(client)
		}()
	}
}

func (p *RegistryTunnelProxy) rejectBusyClient(client net.Conn) {
	defer p.wg.Done()
	defer func() { <-p.rejects }()
	defer p.untrack(client)
	defer client.Close()
	if err := client.SetDeadline(time.Now().Add(registryProxyHandshakeLimit)); err != nil {
		return
	}
	request, reader, headerBytes, err := readRegistryProxyRequest(client)
	if err != nil || !validRegistryProxyRequest(request, headerBytes, p.authorization) || reader.Buffered() != 0 {
		return
	}
	host, err := registryProxyConnectHost(request)
	if err != nil {
		return
	}
	if _, allowed := p.hosts[host]; !allowed {
		return
	}
	writeProxyResponse(client, http.StatusServiceUnavailable)
}

func (p *RegistryTunnelProxy) track(connection net.Conn) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return false
	}
	p.active[connection] = struct{}{}
	return true
}

func (p *RegistryTunnelProxy) untrack(connection net.Conn) {
	p.mu.Lock()
	delete(p.active, connection)
	p.mu.Unlock()
}

func (p *RegistryTunnelProxy) handleClient(client net.Conn) {
	if err := client.SetDeadline(time.Now().Add(registryProxyHandshakeLimit)); err != nil {
		return
	}
	request, reader, headerBytes, err := readRegistryProxyRequest(client)
	if err != nil || !validRegistryProxyRequest(request, headerBytes, p.authorization) {
		writeProxyResponse(client, http.StatusForbidden)
		return
	}
	host, err := registryProxyConnectHost(request)
	if err != nil {
		writeProxyResponse(client, http.StatusForbidden)
		return
	}
	if _, allowed := p.hosts[host]; !allowed {
		writeProxyResponse(client, http.StatusForbidden)
		return
	}
	upstream, err := p.dialer.DialContext(p.ctx, "tcp", net.JoinHostPort(host, "443"))
	if err != nil {
		writeProxyResponse(client, http.StatusBadGateway)
		return
	}
	if !p.track(upstream) {
		_ = upstream.Close()
		return
	}
	defer p.untrack(upstream)
	defer upstream.Close()
	if err = client.SetDeadline(time.Time{}); err != nil {
		return
	}
	if _, err = io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	p.tunnel(client, upstream, io.MultiReader(reader, client))
}

func readRegistryProxyRequest(client net.Conn) (*http.Request, *bufio.Reader, uint64, error) {
	counted := &registryProxyCountingReader{reader: client, limit: registryProxyMaxHeaderBytes}
	reader := bufio.NewReaderSize(counted, 4096)
	request, err := http.ReadRequest(reader)
	if err != nil {
		return nil, nil, counted.read, err
	}
	buffered := uint64(reader.Buffered())
	if buffered > counted.read {
		return nil, nil, counted.read, errRegistryProxyPolicy
	}
	return request, reader, counted.read - buffered, nil
}

func (p *RegistryTunnelProxy) tunnel(client, upstream net.Conn, reader io.Reader) {
	type copyResult struct{ err error }
	results := make(chan copyResult, 2)
	go func() {
		_, err := io.Copy(upstream, reader)
		if closeWriter, ok := upstream.(interface{ CloseWrite() error }); ok {
			_ = closeWriter.CloseWrite()
		}
		results <- copyResult{err: err}
	}()
	go func() {
		_, err := io.Copy(client, upstream)
		if closeWriter, ok := client.(interface{ CloseWrite() error }); ok {
			_ = closeWriter.CloseWrite()
		}
		results <- copyResult{err: err}
	}()
	first := <-results
	if first.err != nil {
		_ = client.Close()
		_ = upstream.Close()
	}
	second := <-results
	if second.err != nil {
		_ = client.Close()
		_ = upstream.Close()
	}
}

func validRegistryProxyRequest(request *http.Request, headerBytes uint64, expectedAuthorization string) bool {
	if request == nil || request.URL == nil || request.Host == "" || request.Method != http.MethodConnect || request.ProtoMajor != 1 || request.ProtoMinor != 1 || request.URL.Scheme != "" || request.URL.Opaque != "" || request.URL.Path != "" || request.URL.RawQuery != "" || headerBytes > registryProxyMaxHeaderBytes || request.ContentLength > 0 || len(request.TransferEncoding) != 0 || len(request.Header) > registryProxyMaxHeaderCount {
		return false
	}
	// net/http stores the Host field on Request.Host rather than in Header.
	fieldCount := 1
	authorizationValues := request.Header.Values("Proxy-Authorization")
	if expectedAuthorization == "" || len(authorizationValues) != 1 || subtle.ConstantTimeCompare([]byte(authorizationValues[0]), []byte(expectedAuthorization)) != 1 {
		return false
	}
	for name, values := range request.Header {
		fieldCount += len(values)
		if strings.EqualFold(name, "Authorization") || fieldCount > registryProxyMaxHeaderCount {
			return false
		}
		for _, value := range values {
			if len(value) > 4096 {
				return false
			}
		}
	}
	return true
}

func registryProxyConnectHost(request *http.Request) (string, error) {
	if request == nil || request.URL == nil {
		return "", errRegistryProxyPolicy
	}
	target := request.Host
	if target == "" {
		target = request.URL.Host
	}
	if request.URL.Host != "" && !strings.EqualFold(target, request.URL.Host) {
		return "", errRegistryProxyPolicy
	}
	host, port, err := net.SplitHostPort(target)
	if err != nil || port != "443" || strings.ContainsAny(host, "%/@\\") {
		return "", errRegistryProxyPolicy
	}
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if !validRegistryProxyHostname(host) {
		return "", errRegistryProxyPolicy
	}
	return host, nil
}

func canonicalRegistryProxyHosts(values []string) (map[string]struct{}, error) {
	if len(values) == 0 || len(values) > RegistryProxyMaxHosts {
		return nil, errRegistryProxyPolicy
	}
	hosts := make(map[string]struct{}, len(values))
	for _, value := range values {
		host := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
		if !validRegistryProxyHostname(host) {
			return nil, errRegistryProxyPolicy
		}
		hosts[host] = struct{}{}
	}
	if len(hosts) == 0 {
		return nil, errRegistryProxyPolicy
	}
	return hosts, nil
}

func validRegistryProxyHostname(host string) bool {
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") || numericRegistryHost(host) || len(host) > 253 || strings.Contains(host, "..") || strings.HasPrefix(host, ".") || strings.HasSuffix(host, ".") {
		return false
	}
	if net.ParseIP(host) != nil {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if !((character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-') {
				return false
			}
		}
	}
	return true
}

func numericRegistryHost(host string) bool {
	labels := strings.Split(host, ".")
	if len(labels) == 0 || len(labels) > 4 {
		return false
	}
	for _, label := range labels {
		base := 10
		value := label
		if strings.HasPrefix(value, "0x") {
			base, value = 16, value[2:]
		} else if len(value) > 1 && value[0] == '0' {
			base = 8
		}
		if value == "" {
			return false
		}
		if _, err := strconv.ParseUint(value, base, 32); err != nil {
			return false
		}
	}
	return true
}

func writeProxyResponse(connection net.Conn, status int) {
	phrase := http.StatusText(status)
	_, _ = fmt.Fprintf(connection, "HTTP/1.1 %d %s\r\nConnection: close\r\nContent-Length: 0\r\n\r\n", status, phrase)
}

type registryProxyCountingReader struct {
	reader io.Reader
	limit  uint64
	read   uint64
}

func (r *registryProxyCountingReader) Read(buffer []byte) (int, error) {
	if r.read >= r.limit {
		return 0, io.EOF
	}
	remaining := r.limit - r.read
	if uint64(len(buffer)) > remaining {
		buffer = buffer[:int(remaining)]
	}
	count, err := r.reader.Read(buffer)
	r.read += uint64(count)
	return count, err
}
