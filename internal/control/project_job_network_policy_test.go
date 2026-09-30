// pattern: Functional Core
package control

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"polis/internal/environment"
)

func TestServiceJobNetworkPolicyDigestBindsFixedListenerTarget(t *testing.T) {
	spec := environment.ServiceProbeSpec{
		BindAddress: "127.0.0.1", Port: 43123, Path: "/health", ExpectedStatusCode: 200,
		ExpectedBodySHA256: digestNetworkPolicyTestBody("ok"), TimeoutMS: 500, LeaseDurationMS: 5000,
	}
	first, err := projectJobServiceNetworkPolicyDigest("windows_appcontainer@1", spec)
	if err != nil {
		t.Fatal(err)
	}
	second, err := projectJobServiceNetworkPolicyDigest("windows_appcontainer@1", spec)
	if err != nil || second != first {
		t.Fatalf("identical service policy digest changed: first=%s second=%s error=%v", first, second, err)
	}
	changedPort := spec
	changedPort.Port++
	portDigest, err := projectJobServiceNetworkPolicyDigest("windows_appcontainer@1", changedPort)
	if err != nil || portDigest == first {
		t.Fatalf("service policy digest did not bind port: first=%s changed=%s error=%v", first, portDigest, err)
	}
	changedAddress := spec
	changedAddress.BindAddress = "::1"
	addressDigest, err := projectJobServiceNetworkPolicyDigest("windows_appcontainer@1", changedAddress)
	if err != nil || addressDigest == first {
		t.Fatalf("service policy digest did not bind bind address: first=%s changed=%s error=%v", first, addressDigest, err)
	}
}

func digestNetworkPolicyTestBody(body string) string {
	digest := sha256.Sum256([]byte(body))
	return hex.EncodeToString(digest[:])
}
