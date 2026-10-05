// pattern: Imperative Shell
package control

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"polis/internal/environment"
	"polis/internal/installationauth"
)

const (
	serviceBrowserSessionTTL         = 5 * time.Minute
	serviceBrowserMaxSessions        = 8
	serviceBrowserMaxConcurrent      = 8
	serviceBrowserMaxRequestDuration = 30 * time.Second
	serviceBrowserMaxHeaderBytes     = 16 << 10
	serviceBrowserMaxRequestBytes    = 8 << 20
	serviceBrowserMaxResponseBytes   = 8 << 20
)

var errServiceBrowserIngressClosed = errors.New("service browser ingress is closed")

type ServiceBrowserSession struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type serviceBrowserTicket struct {
	cookieHash  [32]byte
	cookieValue string
	expiresAt   time.Time
}

type ServiceBrowserIngress struct {
	mu         sync.Mutex
	processID  int
	spec       environment.ServiceProbeSpec
	owner      environment.ServiceEndpointOwnerVerifier
	listener   net.Listener
	server     *http.Server
	proxy      *httputil.ReverseProxy
	origin     string
	cookieName string
	target     *url.URL
	tickets    map[[32]byte]serviceBrowserTicket
	cookies    map[[32]byte]time.Time
	requestIDs map[string]ServiceBrowserSession
	semaphore  chan struct{}
	closed     bool
	now        func() time.Time
}

func NewServiceBrowserIngress(processID int, spec environment.ServiceProbeSpec, owner environment.ServiceEndpointOwnerVerifier) (*ServiceBrowserIngress, error) {
	connectionOwner, ok := owner.(environment.ServiceEndpointConnectionOwnerVerifier)
	if processID <= 0 || owner == nil || !ok {
		return nil, errors.New("service browser ingress requires a process-bound connection owner verifier")
	}
	if _, err := environment.ServiceProbeSpecSHA256(spec); err != nil {
		return nil, environment.ErrInvalidServiceProbe
	}
	if err := owner.VerifyServiceEndpointOwner(context.Background(), processID, spec.BindAddress, spec.Port); err != nil {
		return nil, errors.Join(environment.ErrServiceEndpointOwnerUnverified, err)
	}
	listener, err := listenServiceBrowserLoopback()
	if err != nil {
		return nil, errors.New("could not bind the service browser ingress to an isolated IPv4 loopback address")
	}
	listenerAddress, ok := listener.Addr().(*net.TCPAddr)
	if !ok || !listenerAddress.IP.IsLoopback() || listenerAddress.Port < 1 || listenerAddress.Port > 65535 {
		_ = listener.Close()
		return nil, errors.New("service browser ingress did not receive a loopback listener")
	}
	cookieNonce, _, nonceErr := newServiceBrowserSecret()
	if nonceErr != nil {
		_ = listener.Close()
		return nil, errors.New("could not create a service browser cookie name")
	}
	target := &url.URL{Scheme: "http", Host: net.JoinHostPort(spec.BindAddress, strconv.Itoa(int(spec.Port)))}
	ingress := &ServiceBrowserIngress{
		processID: processID, spec: spec, owner: owner, listener: listener, origin: "http://" + listener.Addr().String(),
		cookieName: "polis_job_" + cookieNonce[:24],
		target:     target, tickets: make(map[[32]byte]serviceBrowserTicket), cookies: make(map[[32]byte]time.Time),
		requestIDs: make(map[string]ServiceBrowserSession), semaphore: make(chan struct{}, serviceBrowserMaxConcurrent), now: time.Now,
	}
	ingress.proxy = ingress.newPinnedProxy(connectionOwner)
	ingress.server = &http.Server{
		Handler: ingress, ReadHeaderTimeout: 3 * time.Second, IdleTimeout: 30 * time.Second,
		ReadTimeout: serviceBrowserMaxRequestDuration, WriteTimeout: serviceBrowserMaxRequestDuration, MaxHeaderBytes: serviceBrowserMaxHeaderBytes,
	}
	go func() { _ = ingress.server.Serve(listener) }()
	return ingress, nil
}

func (ingress *ServiceBrowserIngress) CreateSession(requestID string) (ServiceBrowserSession, error) {
	if ingress == nil || !validServiceBrowserRequestID(requestID) {
		return ServiceBrowserSession{}, errors.New("invalid service browser session request")
	}
	ingress.mu.Lock()
	defer ingress.mu.Unlock()
	if ingress.closed {
		return ServiceBrowserSession{}, errServiceBrowserIngressClosed
	}
	if replay, exists := ingress.requestIDs[requestID]; exists {
		if ingress.now().Before(replay.ExpiresAt) {
			return replay, nil
		}
		delete(ingress.requestIDs, requestID)
	}
	ingress.pruneExpiredSessionsLocked()
	if len(ingress.requestIDs) >= serviceBrowserMaxSessions {
		return ServiceBrowserSession{}, errors.New("service browser session limit is reached")
	}
	if err := ingress.owner.VerifyServiceEndpointOwner(context.Background(), ingress.processID, ingress.spec.BindAddress, ingress.spec.Port); err != nil {
		return ServiceBrowserSession{}, errors.Join(environment.ErrServiceEndpointOwnerUnverified, err)
	}
	ticket, ticketHash, err := newServiceBrowserSecret()
	if err != nil {
		return ServiceBrowserSession{}, errors.New("could not create a service browser ticket")
	}
	cookieValue, cookieHash, err := newServiceBrowserSecret()
	if err != nil {
		return ServiceBrowserSession{}, errors.New("could not create a service browser session cookie")
	}
	expiresAt := ingress.now().Add(serviceBrowserSessionTTL).UTC()
	ingress.tickets[ticketHash] = serviceBrowserTicket{cookieHash: cookieHash, cookieValue: cookieValue, expiresAt: expiresAt}
	ingress.cookies[cookieHash] = expiresAt
	session := ServiceBrowserSession{
		URL:       ingress.origin + "/_polis/open/" + ticket,
		ExpiresAt: expiresAt,
	}
	ingress.requestIDs[requestID] = session
	return session, nil
}

func (ingress *ServiceBrowserIngress) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	if ingress == nil {
		http.Error(response, "service ingress unavailable", http.StatusGone)
		return
	}
	select {
	case ingress.semaphore <- struct{}{}:
		defer func() { <-ingress.semaphore }()
	default:
		http.Error(response, "service ingress is busy", http.StatusServiceUnavailable)
		return
	}
	if !ingress.isLoopbackRemote(request.RemoteAddr) || request.Host != ingress.listener.Addr().String() {
		http.Error(response, "service ingress accepts loopback requests only", http.StatusForbidden)
		return
	}
	ingress.mu.Lock()
	closed := ingress.closed
	ingress.mu.Unlock()
	if closed {
		http.Error(response, "service ingress lease is revoked", http.StatusGone)
		return
	}
	if request.Method == http.MethodConnect || request.Method == http.MethodTrace || request.Method == http.MethodOptions || !validServiceBrowserMethod(request.Method) {
		http.Error(response, "method is not supported by service ingress", http.StatusMethodNotAllowed)
		return
	}
	if strings.HasPrefix(request.URL.Path, "/_polis/open/") {
		ingress.openOneTimeTicket(response, request)
		return
	}
	if !ingress.hasValidCookie(request) {
		http.Error(response, "service browser session is required", http.StatusUnauthorized)
		return
	}
	fetchSite := request.Header.Get("Sec-Fetch-Site")
	if fetchSite != "same-origin" && fetchSite != "none" {
		http.Error(response, "cross-site service browser request is denied", http.StatusForbidden)
		return
	}
	if request.Header.Get("Upgrade") != "" || strings.Contains(strings.ToLower(request.Header.Get("Connection")), "upgrade") {
		http.Error(response, "service ingress does not support protocol upgrades", http.StatusNotImplemented)
		return
	}
	if request.Method != http.MethodGet && request.Method != http.MethodHead {
		if request.Header.Get("Origin") != ingress.origin {
			http.Error(response, "cross-site service mutation is denied", http.StatusForbidden)
			return
		}
	}
	if request.ContentLength > serviceBrowserMaxRequestBytes {
		http.Error(response, "service request exceeds the ingress size limit", http.StatusRequestEntityTooLarge)
		return
	}
	if request.Body != nil {
		body, err := io.ReadAll(io.LimitReader(request.Body, serviceBrowserMaxRequestBytes+1))
		if err != nil {
			http.Error(response, "service request body could not be read", http.StatusBadRequest)
			return
		}
		if len(body) > serviceBrowserMaxRequestBytes {
			http.Error(response, "service request exceeds the ingress size limit", http.StatusRequestEntityTooLarge)
			return
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
		request.ContentLength = int64(len(body))
	}
	if err := ingress.owner.VerifyServiceEndpointOwner(request.Context(), ingress.processID, ingress.spec.BindAddress, ingress.spec.Port); err != nil {
		http.Error(response, "service endpoint owner is no longer verified", http.StatusGone)
		return
	}
	request.Header.Del("Forwarded")
	request.Header.Del("X-Forwarded-For")
	request.Header.Del("X-Forwarded-Host")
	request.Header.Del("X-Forwarded-Proto")
	request.Header.Del("Proxy-Authorization")
	request.Header.Del("Proxy-Connection")
	request.Header.Del(installationauth.OwnerCSRFHeaderName)
	stripServiceBrowserCookie(request, ingress.cookieName)
	proxyContext, cancel := context.WithTimeout(request.Context(), serviceBrowserMaxRequestDuration)
	defer cancel()
	ingress.proxy.ServeHTTP(response, request.WithContext(proxyContext))
}

func (ingress *ServiceBrowserIngress) openOneTimeTicket(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet || request.URL.RawQuery != "" || request.URL.Fragment != "" || request.URL.RawPath != "" {
		http.Error(response, "service browser ticket request is invalid", http.StatusBadRequest)
		return
	}
	ticket := strings.TrimPrefix(request.URL.Path, "/_polis/open/")
	if len(ticket) != 64 || strings.ContainsRune(ticket, '/') {
		http.NotFound(response, request)
		return
	}
	decoded, err := hex.DecodeString(ticket)
	if err != nil || len(decoded) != 32 {
		http.NotFound(response, request)
		return
	}
	ticketHash := sha256.Sum256(decoded)
	ingress.mu.Lock()
	defer ingress.mu.Unlock()
	issued, exists := ingress.tickets[ticketHash]
	delete(ingress.tickets, ticketHash)
	if !exists || !ingress.now().Before(issued.expiresAt) || ingress.closed {
		http.NotFound(response, request)
		return
	}
	if issued.cookieHash == ([32]byte{}) || issued.cookieValue == "" {
		http.Error(response, "service browser session is incomplete", http.StatusGone)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Referrer-Policy", "no-referrer")
	response.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
	response.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.SetCookie(response, &http.Cookie{Name: ingress.cookieName, Value: issued.cookieValue, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Expires: issued.expiresAt, MaxAge: max(1, int(issued.expiresAt.Sub(ingress.now()).Seconds()))})
	response.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(response, "<!doctype html><meta charset=\"utf-8\"><title>Opening local service</title><script>location.replace('/')</script><noscript><a href=\"/\">Open local service</a></noscript>")
}

func (ingress *ServiceBrowserIngress) hasValidCookie(request *http.Request) bool {
	cookie, err := request.Cookie(ingress.cookieName)
	if err != nil || len(cookie.Value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(cookie.Value)
	if err != nil || len(decoded) != 32 {
		return false
	}
	hash := sha256.Sum256(decoded)
	ingress.mu.Lock()
	defer ingress.mu.Unlock()
	expiresAt, exists := ingress.cookies[hash]
	return exists && ingress.now().Before(expiresAt)
}

func (ingress *ServiceBrowserIngress) newPinnedProxy(connectionOwner environment.ServiceEndpointConnectionOwnerVerifier) *httputil.ReverseProxy {
	transport := &http.Transport{
		Proxy: nil, DisableKeepAlives: true, DisableCompression: true,
		ResponseHeaderTimeout: 10 * time.Second, TLSHandshakeTimeout: 0, MaxResponseHeaderBytes: 32 << 10,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			if address != ingress.target.Host || (network != "tcp" && network != "tcp4" && network != "tcp6") {
				return nil, errors.New("service ingress refused a non-pinned upstream")
			}
			if err := ingress.owner.VerifyServiceEndpointOwner(ctx, ingress.processID, ingress.spec.BindAddress, ingress.spec.Port); err != nil {
				return nil, errors.Join(environment.ErrServiceEndpointOwnerUnverified, err)
			}
			connection, err := (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 0}).DialContext(ctx, network, ingress.target.Host)
			if err != nil {
				return nil, err
			}
			local, localOK := connection.LocalAddr().(*net.TCPAddr)
			remote, remoteOK := connection.RemoteAddr().(*net.TCPAddr)
			if !localOK || !remoteOK || local.Port < 1 || local.Port > 65535 || remote.Port != int(ingress.spec.Port) || remote.IP.String() != ingress.spec.BindAddress || !local.IP.IsLoopback() {
				_ = connection.Close()
				return nil, errors.Join(environment.ErrServiceEndpointOwnerUnverified, errors.New("service ingress upstream tuple changed"))
			}
			if err = connectionOwner.VerifyServiceEndpointConnectionOwner(ctx, ingress.processID, ingress.spec.BindAddress, ingress.spec.Port, local.IP.String(), uint16(local.Port)); err != nil {
				_ = connection.Close()
				return nil, errors.Join(environment.ErrServiceEndpointOwnerUnverified, err)
			}
			return connection, nil
		},
	}
	return &httputil.ReverseProxy{
		Transport: transport,
		Director: func(request *http.Request) {
			request.URL.Scheme = ingress.target.Scheme
			request.URL.Host = ingress.target.Host
			request.Host = ingress.target.Host
			request.RequestURI = ""
			request.Header.Del("Forwarded")
			request.Header.Del("X-Forwarded-For")
			request.Header.Del("X-Forwarded-Host")
			request.Header.Del("X-Forwarded-Proto")
			request.Header.Del(installationauth.OwnerCSRFHeaderName)
			stripServiceBrowserCookie(request, ingress.cookieName)
		},
		ModifyResponse: func(response *http.Response) error {
			if response.StatusCode == http.StatusSwitchingProtocols {
				return errors.New("service endpoint attempted an unsupported protocol upgrade")
			}
			if response.ContentLength > serviceBrowserMaxResponseBytes {
				return errors.New("service response exceeds the ingress size limit")
			}
			if err := ingress.owner.VerifyServiceEndpointOwner(response.Request.Context(), ingress.processID, ingress.spec.BindAddress, ingress.spec.Port); err != nil {
				return errors.Join(environment.ErrServiceEndpointOwnerUnverified, err)
			}
			if location := response.Header.Get("Location"); location != "" {
				parsed, err := url.Parse(location)
				if err != nil {
					return errors.New("service response contained an invalid redirect")
				}
				if parsed.Host != "" && !strings.EqualFold(parsed.Host, ingress.target.Host) {
					return errors.New("service response attempted to redirect outside its pinned endpoint")
				}
				if parsed.IsAbs() || parsed.Host != "" {
					parsed.Scheme = "http"
					parsed.Host = ingress.listener.Addr().String()
					response.Header.Set("Location", parsed.String())
				}
			}
			cookies := response.Header.Values("Set-Cookie")
			response.Header.Del("Set-Cookie")
			for _, value := range cookies {
				if isProtectedServiceBrowserCookie(serviceCookieName(value), ingress.cookieName) {
					continue
				}
				response.Header.Add("Set-Cookie", stripCookieDomain(value))
			}
			hasBody := response.Request.Method != http.MethodHead && response.StatusCode != http.StatusNoContent && response.StatusCode != http.StatusNotModified
			if hasBody {
				body, err := io.ReadAll(io.LimitReader(response.Body, serviceBrowserMaxResponseBytes+1))
				if err != nil {
					_ = response.Body.Close()
					return errors.New("service response body could not be bounded")
				}
				_ = response.Body.Close()
				if len(body) > serviceBrowserMaxResponseBytes {
					return errors.New("service response exceeds the ingress size limit")
				}
				response.Body = io.NopCloser(bytes.NewReader(body))
				response.ContentLength = int64(len(body))
				response.Header.Set("Content-Length", strconv.Itoa(len(body)))
				response.Header.Del("Transfer-Encoding")
				response.TransferEncoding = nil
			}
			if err := ingress.owner.VerifyServiceEndpointOwner(response.Request.Context(), ingress.processID, ingress.spec.BindAddress, ingress.spec.Port); err != nil {
				return errors.Join(environment.ErrServiceEndpointOwnerUnverified, err)
			}
			response.Header.Set("Referrer-Policy", "no-referrer")
			response.Header.Set("X-Content-Type-Options", "nosniff")
			return nil
		},
		ErrorHandler: func(response http.ResponseWriter, request *http.Request, err error) {
			status := http.StatusBadGateway
			if errors.Is(err, environment.ErrServiceEndpointOwnerUnverified) {
				status = http.StatusGone
			}
			http.Error(response, "pinned service request failed", status)
		},
	}
}

func (ingress *ServiceBrowserIngress) Close() error {
	if ingress == nil {
		return nil
	}
	ingress.mu.Lock()
	if ingress.closed {
		ingress.mu.Unlock()
		return nil
	}
	ingress.closed = true
	ingress.tickets = make(map[[32]byte]serviceBrowserTicket)
	ingress.cookies = make(map[[32]byte]time.Time)
	ingress.requestIDs = make(map[string]ServiceBrowserSession)
	ingress.mu.Unlock()
	if ingress.proxy != nil && ingress.proxy.Transport != nil {
		if transport, ok := ingress.proxy.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}
	if ingress.server == nil {
		return ingress.listener.Close()
	}
	return ingress.server.Close()
}

func (ingress *ServiceBrowserIngress) isLoopbackRemote(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return false
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}

func (ingress *ServiceBrowserIngress) pruneExpiredSessionsLocked() {
	now := ingress.now()
	for ticketHash, ticket := range ingress.tickets {
		if !now.Before(ticket.expiresAt) {
			delete(ingress.tickets, ticketHash)
			delete(ingress.cookies, ticket.cookieHash)
		}
	}
	for requestID, session := range ingress.requestIDs {
		if !now.Before(session.ExpiresAt) {
			delete(ingress.requestIDs, requestID)
		}
	}
	for cookieHash, expiresAt := range ingress.cookies {
		if !now.Before(expiresAt) {
			delete(ingress.cookies, cookieHash)
		}
	}
}

func newServiceBrowserSecret() (string, [32]byte, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", [32]byte{}, err
	}
	return hex.EncodeToString(raw[:]), sha256.Sum256(raw[:]), nil
}

func validServiceBrowserRequestID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') || character == '-' || character == '_' {
			continue
		}
		return false
	}
	return true
}

func validServiceBrowserMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func stripCookieDomain(value string) string {
	parts := strings.Split(value, ";")
	kept := parts[:0]
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if strings.HasPrefix(strings.ToLower(trimmed), "domain=") {
			continue
		}
		kept = append(kept, trimmed)
	}
	return strings.Join(kept, "; ")
}

func stripServiceBrowserCookie(request *http.Request, cookieName string) {
	cookies := request.Cookies()
	if len(cookies) == 0 {
		request.Header.Del("Cookie")
		return
	}
	forwarded := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		if isProtectedServiceBrowserCookie(cookie.Name, cookieName) {
			continue
		}
		forwarded = append(forwarded, cookie.Name+"="+cookie.Value)
	}
	if len(forwarded) == 0 {
		request.Header.Del("Cookie")
		return
	}
	request.Header.Set("Cookie", strings.Join(forwarded, "; "))
}

func isProtectedServiceBrowserCookie(name, ingressCookieName string) bool {
	return name == ingressCookieName || name == installationauth.OwnerSessionCookieName || name == installationauth.OwnerCSRFCookieName
}

func listenServiceBrowserLoopback() (net.Listener, error) {
	for attempt := 0; attempt < 16; attempt++ {
		nonce, _, err := newServiceBrowserSecret()
		if err != nil {
			return nil, err
		}
		addressBytes, err := hex.DecodeString(nonce[:6])
		if err != nil || len(addressBytes) != 3 {
			return nil, errors.New("could not create an isolated service browser address")
		}
		if addressBytes[2] == 0 || addressBytes[2] == 255 || addressBytes[0] == 0 && addressBytes[1] == 0 && addressBytes[2] == 1 {
			continue
		}
		address := net.JoinHostPort(net.IPv4(127, addressBytes[0], addressBytes[1], addressBytes[2]).String(), "0")
		listener, err := net.Listen("tcp4", address)
		if err == nil {
			return listener, nil
		}
	}
	return nil, errors.New("could not reserve a randomized isolated IPv4 loopback address")
}

func serviceCookieName(value string) string {
	name, _, found := strings.Cut(value, "=")
	if !found {
		return ""
	}
	return strings.TrimSpace(name)
}
