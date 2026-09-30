// pattern: Imperative Shell
package qqnotify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

const maxPermitBodyBytes = 8 << 10

var (
	ErrPermitReplay    = errors.New("sender permit already consumed")
	ErrPermitGuardFull = errors.New("sender permit replay window full")
)

type PermitTransport interface {
	SendSignedPermit(ctx context.Context, key []byte, signed SignedSendPermit, policy SenderPolicy) SendResult
}

type SenderIPCConfig struct {
	Key       []byte
	Policy    SenderPolicy
	Guard     PermitGuard
	Transport PermitTransport
	Now       func() time.Time
}

type SenderIPCServer struct {
	key       []byte
	guard     PermitGuard
	transport PermitTransport
	now       func() time.Time
	dispatch  sync.Mutex
	policy    SenderPolicy
}

func NewSenderIPCServer(config SenderIPCConfig) (*SenderIPCServer, error) {
	if len(config.Key) < 32 || config.Guard == nil || config.Transport == nil || !validShortID(config.Policy.InstallationID) || !validShortID(config.Policy.SenderEpoch) || config.Policy.RouteRevision <= 0 {
		return nil, errors.New("QQ sender IPC configuration is incomplete")
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	key := append([]byte(nil), config.Key...)
	return &SenderIPCServer{key: key, guard: config.Guard, transport: config.Transport, now: now, policy: config.Policy}, nil
}

func (s *SenderIPCServer) UpdatePolicy(policy SenderPolicy) error {
	s.dispatch.Lock()
	defer s.dispatch.Unlock()
	return s.applyPolicyLocked(policy)
}

func (s *SenderIPCServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/healthz", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeSenderJSON(response, http.StatusOK, map[string]string{"service": "polisd_qq_sender", "status": "ready"})
	})
	mux.HandleFunc("/v1/session", func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeSenderJSON(response, http.StatusOK, s.SessionInfo())
	})
	mux.HandleFunc("/v1/policy", s.handlePolicyUpdate)
	mux.HandleFunc("/v1/send", s.handleSend)
	return mux
}

func (s *SenderIPCServer) handlePolicyUpdate(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, maxPermitBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var signed SignedPolicyUpdate
	if err := decoder.Decode(&signed); err != nil {
		writeSenderJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid sender policy payload"})
		return
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		writeSenderJSON(response, http.StatusBadRequest, map[string]string{"error": "policy payload must contain exactly one object"})
		return
	}
	s.dispatch.Lock()
	defer s.dispatch.Unlock()
	if err := VerifyPolicyUpdate(s.key, signed, s.policy, s.now()); err != nil {
		writeSenderJSON(response, http.StatusForbidden, map[string]string{"error": "sender policy update denied"})
		return
	}
	if err := s.applyPolicyLocked(signed.Policy); err != nil {
		writeSenderJSON(response, http.StatusForbidden, map[string]string{"error": "sender policy update denied"})
		return
	}
	writeSenderJSON(response, http.StatusOK, map[string]any{"status": "applied", "routeRevision": s.policy.RouteRevision, "routeEnabled": s.policy.RouteEnabled})
}

func (s *SenderIPCServer) applyPolicyLocked(policy SenderPolicy) error {
	if !validShortID(policy.InstallationID) || !validShortID(policy.SenderEpoch) || policy.RouteRevision < s.policy.RouteRevision || policy.InstallationID != s.policy.InstallationID || policy.SenderEpoch != s.policy.SenderEpoch {
		return errors.New("sender policy update is stale or belongs to another sender session")
	}
	if policy.RouteRevision == s.policy.RouteRevision && policy.RouteEnabled && s.policy.RouteEnabled && (policy.TargetOpenID != s.policy.TargetOpenID || policy.CredentialRef != s.policy.CredentialRef) {
		return errors.New("target or credential changes require a new route revision")
	}
	if policy.RouteEnabled {
		if policy.RouteStatus != "ready" || !validTarget(policy.TargetOpenID) || !ValidCredentialRef(policy.CredentialRef) || policy.QualifiedUntil.IsZero() || !s.now().Before(policy.QualifiedUntil) {
			return errors.New("enabled sender policy is not qualified")
		}
	} else if policy.RouteStatus != "disabled" && policy.RouteStatus != "revoked" && policy.RouteStatus != "revocation_pending" {
		return errors.New("disabled sender policy status is invalid")
	}
	s.policy = policy
	return nil
}

func (s *SenderIPCServer) SessionInfo() SenderSessionInfo {
	s.dispatch.Lock()
	defer s.dispatch.Unlock()
	info, _ := SignSenderSession(s.key, SenderSessionInfo{InstallationID: s.policy.InstallationID, SenderEpoch: s.policy.SenderEpoch})
	return info
}

func (s *SenderIPCServer) handleSend(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		response.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, maxPermitBodyBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var signed SignedSendPermit
	if err := decoder.Decode(&signed); err != nil {
		writeSenderJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid permit payload"})
		return
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		writeSenderJSON(response, http.StatusBadRequest, map[string]string{"error": "permit payload must contain exactly one object"})
		return
	}
	s.dispatch.Lock()
	defer s.dispatch.Unlock()
	if err := VerifySendPermit(s.key, signed, s.policy, s.now()); err != nil {
		writeSenderJSON(response, http.StatusForbidden, map[string]string{"error": "permit denied"})
		return
	}
	if err := s.guard.Consume(request.Context(), signed.Permit); err != nil {
		if errors.Is(err, ErrPermitReplay) {
			writeSenderJSON(response, http.StatusConflict, map[string]string{"error": "permit already consumed"})
			return
		}
		writeSenderJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "permit guard unavailable"})
		return
	}
	result := s.transport.SendSignedPermit(request.Context(), s.key, signed, s.policy)
	writeSenderJSON(response, http.StatusOK, result)
}

func writeSenderJSON(response http.ResponseWriter, status int, body any) {
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(body)
}

// LoopbackAddress rejects hostnames and non-loopback listeners for the local IPC server.
func LoopbackAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return errors.New("sender IPC address must be an explicit loopback IP and port")
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return errors.New("sender IPC must bind to a loopback IP")
	}
	return nil
}
