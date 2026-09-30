// pattern: Imperative Shell
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"polis/internal/kernel"
	"polis/internal/qqnotify"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	mode := flag.String("mode", "fake-only", "fake-only | notification-sender | store-qq-credentials")
	company := flag.String("company", "", "test company")
	steps := flag.Int("steps", 8, "maximum fake boundaries; exits immediately when idle")
	listen := flag.String("listen", "127.0.0.1:8099", "loopback-only notification sender IPC address")
	credentialRef := flag.String("credential-ref", "default", "protected QQ credential reference; never a secret value")
	flag.Parse()
	switch *mode {
	case "notification-sender":
		return runNotificationSender(*listen)
	case "store-qq-credentials":
		return storeQQCredentials(*credentialRef)
	case "fake-only":
		return runFakeOnly(*company, *steps)
	default:
		return fmt.Errorf("unknown polisd mode")
	}
}

func runFakeOnly(company string, steps int) error {
	if company == "" || steps < 1 || steps > 100 {
		return fmt.Errorf("company and steps (1..100) required")
	}
	root := os.Getenv("POLIS_BLOB_ROOT")
	if root == "" {
		return fmt.Errorf("POLIS_BLOB_ROOT required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	k, e := kernel.Open(ctx, os.Getenv("POLIS_DSN"), root)
	if e != nil {
		return e
	}
	defer k.Close()
	count := 0
	for ; count < steps; count++ {
		worked, e := k.Step(ctx, k.LocalScope(company))
		if e != nil {
			return e
		}
		if !worked {
			break
		}
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		Mode  string `json:"mode"`
		Steps int    `json:"steps"`
	}{"fake_only", count})
}

func runNotificationSender(address string) error {
	if os.Getenv("POLIS_QQ_ENABLED") != "1" {
		return fmt.Errorf("QQ notification sender is disabled")
	}
	if err := qqnotify.LoopbackAddress(address); err != nil {
		return err
	}
	key, err := base64.RawURLEncoding.DecodeString(os.Getenv("POLIS_SENDER_IPC_KEY"))
	if err != nil || len(key) < 32 {
		return fmt.Errorf("a per-session authenticated IPC key is required")
	}
	credentialRef := os.Getenv("POLIS_QQ_CREDENTIAL_REF")
	if credentialRef == "" {
		credentialRef = "default"
	}
	installationID := os.Getenv("POLIS_INSTALLATION_ID")
	if installationID == "" {
		return fmt.Errorf("notification installation is required")
	}
	senderEpoch, err := newSenderEpoch()
	if err != nil {
		return fmt.Errorf("could not create a fresh sender epoch")
	}
	revision, err := strconv.ParseInt(os.Getenv("POLIS_QQ_ROUTE_REVISION"), 10, 64)
	if err != nil || revision < 1 {
		return fmt.Errorf("a positive QQ route revision is required")
	}
	routeEnabled := os.Getenv("POLIS_QQ_ROUTE_ENABLED") == "1"
	routeStatus := os.Getenv("POLIS_QQ_ROUTE_STATUS")
	if routeStatus == "" {
		routeStatus = "disabled"
	}
	qualifiedUntil := time.Time{}
	if raw := strings.TrimSpace(os.Getenv("POLIS_QQ_QUALIFIED_UNTIL")); raw != "" {
		qualifiedUntil, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			return fmt.Errorf("QQ route qualification expiry is invalid")
		}
	}
	policy := qqnotify.SenderPolicy{
		InstallationID: installationID,
		SenderEpoch:    senderEpoch,
		RouteRevision:  revision,
		TargetOpenID:   os.Getenv("POLIS_QQ_TARGET_OPENID"),
		CredentialRef:  credentialRef,
		RouteEnabled:   routeEnabled,
		RouteStatus:    routeStatus,
		QualifiedUntil: qualifiedUntil,
	}
	server, err := qqnotify.NewSenderIPCServer(qqnotify.SenderIPCConfig{
		Key: key, Policy: policy, Guard: qqnotify.NewMemoryPermitGuard(4096),
		Transport: qqnotify.NewClient(qqnotify.Config{CredentialRef: credentialRef, LoadCredentials: func(ctx context.Context, reference string) (qqnotify.Credentials, error) {
			return qqnotify.ReadProtectedQQCredentials(reference)
		}}),
	})
	if err != nil {
		return fmt.Errorf("could not start authenticated QQ sender")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("could not bind QQ sender IPC")
	}
	defer listener.Close()
	serve := &http.Server{Addr: address, Handler: server.Handler(), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 15 * time.Second, MaxHeaderBytes: 16 << 10}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- serve.Serve(listener) }()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	select {
	case err = <-serverErrors:
		if err == http.ErrServerClosed {
			return nil
		}
		return fmt.Errorf("QQ sender IPC stopped")
	case <-signals:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return serve.Shutdown(ctx)
	}
}

func newSenderEpoch() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func storeQQCredentials(credentialRef string) error {
	decoder := json.NewDecoder(os.Stdin)
	decoder.DisallowUnknownFields()
	var credentials qqnotify.Credentials
	if err := decoder.Decode(&credentials); err != nil {
		return fmt.Errorf("invalid credential input")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("credential input must contain one JSON object")
	}
	if err := qqnotify.StoreProtectedQQCredentials(credentialRef, credentials); err != nil {
		return fmt.Errorf("could not store protected QQ credentials")
	}
	return nil
}
