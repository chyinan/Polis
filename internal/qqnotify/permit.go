// pattern: Functional Core
package qqnotify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"
)

const (
	SendPurposeC2CText = "notification.qq.c2c.text"
	MaxPermitLifetime  = 30 * time.Second
	MaxPermitClockSkew = 5 * time.Second
)

type SendPermit struct {
	Purpose         string    `json:"purpose"`
	PermitID        string    `json:"permitId"`
	InstallationID  string    `json:"installationId"`
	SenderEpoch     string    `json:"senderEpoch"`
	DeliveryID      string    `json:"deliveryId"`
	RouteRevision   int64     `json:"routeRevision"`
	TargetOpenID    string    `json:"targetOpenId"`
	CredentialRef   string    `json:"credentialRef"`
	Message         string    `json:"message"`
	MessageDigest   string    `json:"messageDigest"`
	MessageSequence int64     `json:"messageSequence"`
	IssuedAt        time.Time `json:"issuedAt"`
	ExpiresAt       time.Time `json:"expiresAt"`
}

type SignedSendPermit struct {
	Permit    SendPermit `json:"permit"`
	Signature string     `json:"signature"`
}

type SenderPolicy struct {
	InstallationID string    `json:"installationId"`
	SenderEpoch    string    `json:"senderEpoch"`
	RouteRevision  int64     `json:"routeRevision"`
	TargetOpenID   string    `json:"targetOpenId"`
	CredentialRef  string    `json:"credentialRef"`
	RouteEnabled   bool      `json:"routeEnabled"`
	RouteStatus    string    `json:"routeStatus"`
	QualifiedUntil time.Time `json:"qualifiedUntil"`
}

type SignedPolicyUpdate struct {
	Policy    SenderPolicy `json:"policy"`
	Signature string       `json:"signature"`
}

type SenderSessionInfo struct {
	InstallationID string `json:"installationId"`
	SenderEpoch    string `json:"senderEpoch"`
	Signature      string `json:"signature"`
}

func SignSendPermit(key []byte, permit SendPermit) (SignedSendPermit, error) {
	if len(key) < 32 {
		return SignedSendPermit{}, errors.New("sender IPC signing key must be at least 32 bytes")
	}
	permit.MessageDigest = digestText(permit.Message)
	if err := validatePermitShape(permit); err != nil {
		return SignedSendPermit{}, err
	}
	encoded, err := json.Marshal(permit)
	if err != nil {
		return SignedSendPermit{}, err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(encoded)
	return SignedSendPermit{Permit: permit, Signature: base64.RawURLEncoding.EncodeToString(mac.Sum(nil))}, nil
}

func VerifySendPermit(key []byte, signed SignedSendPermit, policy SenderPolicy, now time.Time) error {
	if len(key) < 32 || validatePermitShape(signed.Permit) != nil {
		return errors.New("invalid sender permit")
	}
	permit := signed.Permit
	if !policy.RouteEnabled || policy.RouteStatus != "ready" || permit.InstallationID != policy.InstallationID || permit.SenderEpoch != policy.SenderEpoch || permit.RouteRevision != policy.RouteRevision || permit.TargetOpenID != policy.TargetOpenID || permit.CredentialRef != policy.CredentialRef || !now.Before(policy.QualifiedUntil) {
		return errors.New("sender permit is stale or outside the approved route")
	}
	if permit.IssuedAt.After(now.Add(MaxPermitClockSkew)) || !now.Before(permit.ExpiresAt) || permit.ExpiresAt.Sub(permit.IssuedAt) > MaxPermitLifetime {
		return errors.New("sender permit is expired or outside its short lifetime")
	}
	encoded, err := json.Marshal(permit)
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(encoded)
	want := mac.Sum(nil)
	provided, err := base64.RawURLEncoding.DecodeString(signed.Signature)
	if err != nil || !hmac.Equal(provided, want) {
		return errors.New("sender permit signature is invalid")
	}
	return nil
}

func SignPolicyUpdate(key []byte, policy SenderPolicy) (SignedPolicyUpdate, error) {
	if len(key) < 32 || !validShortID(policy.InstallationID) || !validShortID(policy.SenderEpoch) || policy.RouteRevision < 0 {
		return SignedPolicyUpdate{}, errors.New("sender policy update fields are invalid")
	}
	encoded, err := json.Marshal(policy)
	if err != nil {
		return SignedPolicyUpdate{}, err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(encoded)
	return SignedPolicyUpdate{Policy: policy, Signature: base64.RawURLEncoding.EncodeToString(mac.Sum(nil))}, nil
}

func VerifyPolicyUpdate(key []byte, signed SignedPolicyUpdate, current SenderPolicy, now time.Time) error {
	if len(key) < 32 || !validShortID(signed.Policy.InstallationID) || !validShortID(signed.Policy.SenderEpoch) || signed.Policy.RouteRevision < current.RouteRevision {
		return errors.New("sender policy update is invalid or stale")
	}
	if signed.Policy.InstallationID != current.InstallationID || signed.Policy.SenderEpoch != current.SenderEpoch {
		return errors.New("sender policy update belongs to another sender session")
	}
	if signed.Policy.RouteRevision == current.RouteRevision && signed.Policy.RouteEnabled && current.RouteEnabled && (signed.Policy.TargetOpenID != current.TargetOpenID || signed.Policy.CredentialRef != current.CredentialRef) {
		return errors.New("target or credential changes require a new route revision")
	}
	if signed.Policy.RouteEnabled {
		if signed.Policy.RouteStatus != "ready" || !validTarget(signed.Policy.TargetOpenID) || !ValidCredentialRef(signed.Policy.CredentialRef) || signed.Policy.QualifiedUntil.IsZero() || !now.Before(signed.Policy.QualifiedUntil) {
			return errors.New("enabled sender policy is not currently qualified")
		}
	} else if signed.Policy.RouteStatus != "disabled" && signed.Policy.RouteStatus != "revoked" && signed.Policy.RouteStatus != "revocation_pending" {
		return errors.New("disabled sender policy has an invalid status")
	}
	encoded, err := json.Marshal(signed.Policy)
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(encoded)
	want := mac.Sum(nil)
	provided, err := base64.RawURLEncoding.DecodeString(signed.Signature)
	if err != nil || !hmac.Equal(provided, want) {
		return errors.New("sender policy signature is invalid")
	}
	return nil
}

func SignSenderSession(key []byte, info SenderSessionInfo) (SenderSessionInfo, error) {
	if len(key) < 32 || !validShortID(info.InstallationID) || !validShortID(info.SenderEpoch) {
		return SenderSessionInfo{}, errors.New("sender session identity is invalid")
	}
	info.Signature = ""
	encoded, err := json.Marshal(info)
	if err != nil {
		return SenderSessionInfo{}, err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(encoded)
	info.Signature = base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return info, nil
}

func VerifySenderSession(key []byte, info SenderSessionInfo) error {
	if len(key) < 32 || !validShortID(info.InstallationID) || !validShortID(info.SenderEpoch) {
		return errors.New("sender session identity is invalid")
	}
	signature := info.Signature
	info.Signature = ""
	encoded, err := json.Marshal(info)
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(encoded)
	want := mac.Sum(nil)
	provided, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil || !hmac.Equal(provided, want) {
		return errors.New("sender session signature is invalid")
	}
	return nil
}

func validatePermitShape(permit SendPermit) error {
	if permit.Purpose != SendPurposeC2CText || !validShortID(permit.PermitID) || !validShortID(permit.InstallationID) || !validShortID(permit.SenderEpoch) || !validShortID(permit.DeliveryID) || permit.RouteRevision <= 0 || permit.MessageSequence <= 0 || !validTarget(permit.TargetOpenID) || !ValidCredentialRef(permit.CredentialRef) || !validOutboundText(permit.Message) || permit.IssuedAt.IsZero() || permit.ExpiresAt.IsZero() || !permit.ExpiresAt.After(permit.IssuedAt) || permit.ExpiresAt.Sub(permit.IssuedAt) > MaxPermitLifetime {
		return errors.New("sender permit fields are invalid")
	}
	if !validSHA256(permit.MessageDigest) || permit.MessageDigest != digestText(permit.Message) {
		return errors.New("sender permit message digest does not match")
	}
	return nil
}

func validShortID(value string) bool {
	if value == "" || len(value) > 96 {
		return false
	}
	for _, character := range value {
		if (character >= 'A' && character <= 'Z') || (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || strings.ContainsRune("_-.", character) {
			continue
		}
		return false
	}
	return true
}

func digestText(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func MessageDigest(value string) string {
	return digestText(value)
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

type PermitGuard interface {
	Consume(ctx context.Context, permit SendPermit) error
}

type MemoryPermitGuard struct {
	mu       sync.Mutex
	used     map[string]time.Time
	capacity int
	now      func() time.Time
}

func NewMemoryPermitGuard(capacity int) *MemoryPermitGuard {
	if capacity < 1 {
		capacity = 1
	}
	return &MemoryPermitGuard{used: make(map[string]time.Time), capacity: capacity, now: time.Now}
}

func (g *MemoryPermitGuard) Consume(ctx context.Context, permit SendPermit) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validShortID(permit.PermitID) || !permit.ExpiresAt.After(g.now()) {
		return errors.New("permit guard rejected invalid or expired permit")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	for id, expiresAt := range g.used {
		if !now.Before(expiresAt) {
			delete(g.used, id)
		}
	}
	if _, used := g.used[permit.PermitID]; used {
		return ErrPermitReplay
	}
	if len(g.used) >= g.capacity {
		return ErrPermitGuardFull
	}
	g.used[permit.PermitID] = permit.ExpiresAt
	return nil
}

func (g *MemoryPermitGuard) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.used = make(map[string]time.Time)
}
