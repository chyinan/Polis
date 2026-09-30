//go:build service_ingress_fixture

// pattern: Imperative Shell
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"polis/internal/control"
	"polis/internal/environment"
)

const browserFixtureAddress = "127.0.0.1:45169"

type fixtureServiceOwner struct {
	processID int
	address   string
	port      uint16
}

func (owner fixtureServiceOwner) VerifyServiceEndpointOwner(_ context.Context, processID int, address string, port uint16) error {
	if processID != owner.processID || address != owner.address || port != owner.port {
		return environment.ErrServiceEndpointOwnerUnverified
	}
	return nil
}

func (owner fixtureServiceOwner) VerifyServiceEndpointConnectionOwner(_ context.Context, processID int, address string, port uint16, localAddress string, localPort uint16) error {
	if processID != owner.processID || address != owner.address || port != owner.port || localPort == 0 || net.ParseIP(localAddress) == nil || !net.ParseIP(localAddress).IsLoopback() {
		return environment.ErrServiceEndpointOwnerUnverified
	}
	return nil
}

func main() {
	service := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/health":
			_, _ = response.Write([]byte("ok"))
		case "/meta":
			response.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(response).Encode(map[string]string{"fetchSite": request.Header.Get("Sec-Fetch-Site")})
		case "/":
			response.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = response.Write([]byte(`<!doctype html><meta charset="utf-8"><title>Polis browser ingress fixture</title><h1>Polis browser ingress fixture</h1><output id="result">loading</output><script>fetch('/meta').then(response => response.json()).then(meta => { document.querySelector('#result').textContent = 'rendered; fetch-site=' + meta.fetchSite; });</script>`))
		default:
			http.NotFound(response, request)
		}
	}))
	defer service.Close()
	address, portText, err := net.SplitHostPort(strings.TrimPrefix(service.URL, "http://"))
	if err != nil {
		panic(err)
	}
	portValue, err := strconv.ParseUint(portText, 10, 16)
	if err != nil || portValue == 0 {
		panic("fixture service port is invalid")
	}
	bodyDigest := sha256.Sum256([]byte("ok"))
	ingress, err := control.NewServiceBrowserIngress(1, environment.ServiceProbeSpec{
		BindAddress: address, Port: uint16(portValue), Path: "/health", ExpectedStatusCode: http.StatusOK,
		ExpectedBodySHA256: hex.EncodeToString(bodyDigest[:]), TimeoutMS: 1000, LeaseDurationMS: 60000,
	}, fixtureServiceOwner{processID: 1, address: address, port: uint16(portValue)})
	if err != nil {
		panic(err)
	}
	defer ingress.Close()
	session, err := ingress.CreateSession("browser-smoke-session")
	if err != nil {
		panic(err)
	}
	listener, err := net.Listen("tcp4", browserFixtureAddress)
	if err != nil {
		panic(fmt.Errorf("browser smoke fixture address is unavailable: %w", err))
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(response http.ResponseWriter, _ *http.Request) { response.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("/", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(response, "<!doctype html><meta charset=\"utf-8\"><title>Workbench fixture</title><a id=\"open-service\" href=\"%s\">Open service</a>", session.URL)
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}
	if err = server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}
