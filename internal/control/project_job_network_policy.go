// pattern: Functional Core
package control

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"polis/internal/environment"
)

const appContainerServiceListenerPolicyRevision = "appcontainer-fixed-loopback-listener@1"

type projectJobServiceNetworkPolicy struct {
	IsolationProfile string `json:"isolationProfile"`
	NetworkPolicy    string `json:"networkPolicy"`
	ListenerPolicy   string `json:"listenerPolicy"`
	BindAddress      string `json:"bindAddress"`
	Port             uint16 `json:"port"`
	Protocol         string `json:"protocol"`
}

func projectJobServiceNetworkPolicyDigest(isolationProfile string, spec environment.ServiceProbeSpec) (string, error) {
	if isolationProfile != environment.WindowsNodeIsolationProfile {
		return "", errors.New("fixed AppContainer service listener policy is unsupported for this isolation profile")
	}
	if _, err := environment.ServiceProbeSpecSHA256(spec); err != nil {
		return "", errors.Join(environment.ErrInvalidServiceProbe, err)
	}
	manifest := projectJobServiceNetworkPolicy{
		IsolationProfile: isolationProfile,
		NetworkPolicy:    "deny_all",
		ListenerPolicy:   appContainerServiceListenerPolicyRevision,
		BindAddress:      spec.BindAddress,
		Port:             spec.Port,
		Protocol:         "tcp",
	}
	canonical, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}
