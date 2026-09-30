// pattern: Functional Core
package qqnotify

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSignedPermitBindsSingleC2CTextTargetRouteAndProcessEpoch(t *testing.T) {
	now := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)
	key := []byte(strings.Repeat("k", 32))
	permit := SendPermit{
		Purpose: SendPurposeC2CText, PermitID: "permit-1", InstallationID: "install-1", SenderEpoch: "epoch-1",
		DeliveryID: "delivery-1", RouteRevision: 7, TargetOpenID: "admin-openid", CredentialRef: "default", Message: "safe notice",
		MessageSequence: 9, IssuedAt: now, ExpiresAt: now.Add(20 * time.Second),
	}
	signed, err := SignSendPermit(key, permit)
	if err != nil {
		t.Fatal(err)
	}
	policy := SenderPolicy{InstallationID: "install-1", SenderEpoch: "epoch-1", RouteRevision: 7, TargetOpenID: "admin-openid", CredentialRef: "default", RouteEnabled: true, RouteStatus: "ready", QualifiedUntil: now.Add(time.Hour)}
	if err = VerifySendPermit(key, signed, policy, now); err != nil {
		t.Fatalf("valid permit rejected: %v", err)
	}

	tampered := signed
	tampered.Permit.Message = "changed body"
	if err = VerifySendPermit(key, tampered, policy, now); err == nil {
		t.Fatal("accepted a changed notification body")
	}
	if err = VerifySendPermit(key, signed, SenderPolicy{InstallationID: "install-1", SenderEpoch: "epoch-2", RouteRevision: 7, TargetOpenID: "admin-openid", CredentialRef: "default", RouteEnabled: true, RouteStatus: "ready", QualifiedUntil: now.Add(time.Hour)}, now); err == nil {
		t.Fatal("accepted a permit from a revoked sender epoch")
	}
	if err = VerifySendPermit(key, signed, SenderPolicy{InstallationID: "install-1", SenderEpoch: "epoch-1", RouteRevision: 8, TargetOpenID: "admin-openid", CredentialRef: "default", RouteEnabled: true, RouteStatus: "ready", QualifiedUntil: now.Add(time.Hour)}, now); err == nil {
		t.Fatal("accepted a stale route revision")
	}
	if err = VerifySendPermit(key, signed, SenderPolicy{InstallationID: "install-1", SenderEpoch: "epoch-1", RouteRevision: 7, TargetOpenID: "different-admin", CredentialRef: "default", RouteEnabled: true, RouteStatus: "ready", QualifiedUntil: now.Add(time.Hour)}, now); err == nil {
		t.Fatal("accepted an alternate notification target")
	}
	if err = VerifySendPermit(key, signed, policy, now.Add(21*time.Second)); err == nil {
		t.Fatal("accepted an expired permit")
	}
}

func TestMemoryPermitGuardIsSingleUseAndRevocationClearsPendingWindow(t *testing.T) {
	guard := NewMemoryPermitGuard(4)
	permit := SendPermit{PermitID: "permit-1", ExpiresAt: time.Now().Add(time.Minute)}
	if err := guard.Consume(context.Background(), permit); err != nil {
		t.Fatal(err)
	}
	if err := guard.Consume(context.Background(), permit); err == nil {
		t.Fatal("accepted permit replay")
	}
	guard.Reset()
	if err := guard.Consume(context.Background(), permit); err != nil {
		t.Fatalf("replay guard reset did not open a new epoch: %v", err)
	}
}
