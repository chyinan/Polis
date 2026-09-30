// pattern: Imperative Shell
package environment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const MaxServiceProbeBodyBytes = 64 << 10

var (
	ErrInvalidServiceProbe            = errors.New("invalid bounded service probe contract")
	ErrServiceEndpointOwnerUnverified = errors.New("service endpoint does not belong to the authorized process")
)

type ServiceProbeSpec struct {
	BindAddress        string `json:"bindAddress"`
	Port               uint16 `json:"port"`
	Path               string `json:"path"`
	ExpectedStatusCode int    `json:"expectedStatusCode"`
	ExpectedBodySHA256 string `json:"expectedBodySha256"`
	TimeoutMS          int    `json:"timeoutMs"`
	LeaseDurationMS    int    `json:"leaseDurationMs"`
}

type ServiceEndpointOwnerVerifier interface {
	VerifyServiceEndpointOwner(context.Context, int, string, uint16) error
}

type ServiceEndpointConnectionOwnerVerifier interface {
	VerifyServiceEndpointConnectionOwner(context.Context, int, string, uint16, string, uint16) error
}

type ServiceProbeEvidence struct {
	ProcessID         int              `json:"processId"`
	BindAddress       string           `json:"bindAddress"`
	Port              uint16           `json:"port"`
	HealthcheckSHA256 string           `json:"healthcheckSha256"`
	Readiness         ServiceReadiness `json:"readiness"`
	HTTPStatus        int              `json:"httpStatus,omitempty"`
	ResponseSHA256    string           `json:"responseSha256,omitempty"`
	ReasonCode        string           `json:"reasonCode"`
	ProbedAt          time.Time        `json:"probedAt"`
	LeaseExpiresAt    time.Time        `json:"leaseExpiresAt,omitempty"`
}

func ServiceProbeSpecSHA256(spec ServiceProbeSpec) (string, error) {
	if !validServiceProbeSpec(spec) {
		return "", ErrInvalidServiceProbe
	}
	canonical, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	return digestServiceProbeBytes(canonical), nil
}

func ProbeServiceEndpoint(ctx context.Context, processID int, spec ServiceProbeSpec, owner ServiceEndpointOwnerVerifier) (ServiceProbeEvidence, error) {
	if ctx == nil || processID <= 0 || owner == nil || !validServiceProbeSpec(spec) {
		return ServiceProbeEvidence{}, ErrInvalidServiceProbe
	}
	connectionOwner, ok := owner.(ServiceEndpointConnectionOwnerVerifier)
	if !ok {
		return ServiceProbeEvidence{ProcessID: processID, BindAddress: spec.BindAddress, Port: spec.Port, Readiness: ServiceUnhealthy, ReasonCode: "endpoint_connection_owner_unverified", ProbedAt: time.Now().UTC()}, ErrServiceEndpointOwnerUnverified
	}
	configDigest, err := ServiceProbeSpecSHA256(spec)
	if err != nil {
		return ServiceProbeEvidence{}, err
	}
	probeContext, cancel := context.WithTimeout(ctx, time.Duration(spec.TimeoutMS)*time.Millisecond)
	defer cancel()
	evidence := ServiceProbeEvidence{
		ProcessID: processID, BindAddress: spec.BindAddress, Port: spec.Port,
		HealthcheckSHA256: configDigest, Readiness: ServiceUnhealthy, ProbedAt: time.Now().UTC(),
		ReasonCode: "probe_not_completed",
	}
	target := net.JoinHostPort(spec.BindAddress, itoaPort(spec.Port))
	if err = owner.VerifyServiceEndpointOwner(probeContext, processID, spec.BindAddress, spec.Port); err != nil {
		if contextErr := probeContext.Err(); contextErr != nil {
			evidence.ReasonCode = "probe_timeout"
			return evidence, contextErr
		}
		evidence.ReasonCode = "endpoint_owner_unverified"
		return evidence, errors.Join(ErrServiceEndpointOwnerUnverified, err)
	}
	if err = probeContext.Err(); err != nil {
		evidence.ReasonCode = "probe_timeout"
		return evidence, err
	}
	endpoint := url.URL{Scheme: "http", Host: target, Path: spec.Path}
	transport := &http.Transport{
		Proxy:                 nil,
		DisableKeepAlives:     true,
		DisableCompression:    true,
		ResponseHeaderTimeout: time.Duration(spec.TimeoutMS) * time.Millisecond,
		DialContext: func(dialContext context.Context, network, address string) (net.Conn, error) {
			if network != "tcp" && network != "tcp4" && network != "tcp6" || address != target {
				return nil, ErrInvalidServiceProbe
			}
			connection, dialErr := (&net.Dialer{}).DialContext(dialContext, network, target)
			if dialErr != nil {
				return nil, dialErr
			}
			local, localOK := connection.LocalAddr().(*net.TCPAddr)
			remote, remoteOK := connection.RemoteAddr().(*net.TCPAddr)
			if !localOK || !remoteOK || local.Port < 1 || local.Port > 65535 || remote.Port != int(spec.Port) || !remote.IP.Equal(net.ParseIP(spec.BindAddress)) || !local.IP.IsLoopback() {
				_ = connection.Close()
				return nil, errors.Join(ErrServiceEndpointOwnerUnverified, ErrInvalidServiceProbe)
			}
			if verifyErr := connectionOwner.VerifyServiceEndpointConnectionOwner(dialContext, processID, spec.BindAddress, spec.Port, local.IP.String(), uint16(local.Port)); verifyErr != nil {
				_ = connection.Close()
				if contextErr := dialContext.Err(); contextErr != nil {
					return nil, contextErr
				}
				return nil, errors.Join(ErrServiceEndpointOwnerUnverified, verifyErr)
			}
			if contextErr := dialContext.Err(); contextErr != nil {
				_ = connection.Close()
				return nil, contextErr
			}
			return connection, nil
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(spec.TimeoutMS) * time.Millisecond,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	request, err := http.NewRequestWithContext(probeContext, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return ServiceProbeEvidence{}, ErrInvalidServiceProbe
	}
	request.Header.Set("User-Agent", "Polis-Service-Probe/1")
	response, err := client.Do(request)
	if err != nil {
		if errors.Is(err, ErrServiceEndpointOwnerUnverified) {
			evidence.ReasonCode = "endpoint_connection_owner_unverified"
			return evidence, errors.Join(ErrServiceEndpointOwnerUnverified, err)
		}
		evidence.ReasonCode = serviceProbeTransportReason(probeContext, err)
		if probeContext.Err() == nil {
			if ownerErr := confirmServiceProbeOwner(probeContext, processID, spec, owner, &evidence); ownerErr != nil {
				return evidence, ownerErr
			}
		}
		return evidence, nil
	}
	defer response.Body.Close()
	evidence.HTTPStatus = response.StatusCode
	body, readErr := io.ReadAll(io.LimitReader(response.Body, MaxServiceProbeBodyBytes+1))
	if readErr != nil {
		evidence.ReasonCode = serviceProbeTransportReason(probeContext, readErr)
		if evidence.ReasonCode != "probe_timeout" {
			evidence.ReasonCode = "response_read_failed"
		}
		if probeContext.Err() == nil {
			if ownerErr := confirmServiceProbeOwner(probeContext, processID, spec, owner, &evidence); ownerErr != nil {
				return evidence, ownerErr
			}
		}
		return evidence, nil
	}
	if len(body) > MaxServiceProbeBodyBytes {
		evidence.ReasonCode = "response_too_large"
		if ownerErr := confirmServiceProbeOwner(probeContext, processID, spec, owner, &evidence); ownerErr != nil {
			return evidence, ownerErr
		}
		return evidence, nil
	}
	evidence.ResponseSHA256 = digestServiceProbeBytes(body)
	if ownerErr := confirmServiceProbeOwner(probeContext, processID, spec, owner, &evidence); ownerErr != nil {
		return evidence, ownerErr
	}
	if response.StatusCode != spec.ExpectedStatusCode {
		evidence.ReasonCode = "http_status_mismatch"
		return evidence, nil
	}
	if evidence.ResponseSHA256 != spec.ExpectedBodySHA256 {
		evidence.ReasonCode = "response_digest_mismatch"
		return evidence, nil
	}
	if err = probeContext.Err(); err != nil {
		evidence.ReasonCode = "probe_timeout"
		return evidence, err
	}
	evidence.Readiness = ServiceReady
	evidence.ReasonCode = "healthcheck_matched"
	return evidence, nil
}

func confirmServiceProbeOwner(ctx context.Context, processID int, spec ServiceProbeSpec, owner ServiceEndpointOwnerVerifier, evidence *ServiceProbeEvidence) error {
	if err := owner.VerifyServiceEndpointOwner(ctx, processID, spec.BindAddress, spec.Port); err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			evidence.ReasonCode = "probe_timeout"
			return contextErr
		}
		evidence.ReasonCode = "endpoint_owner_changed"
		return errors.Join(ErrServiceEndpointOwnerUnverified, err)
	}
	if err := ctx.Err(); err != nil {
		evidence.ReasonCode = "probe_timeout"
		return err
	}
	evidence.LeaseExpiresAt = evidence.ProbedAt.Add(time.Duration(spec.LeaseDurationMS) * time.Millisecond)
	return nil
}

func validServiceProbeSpec(spec ServiceProbeSpec) bool {
	if (spec.BindAddress != "127.0.0.1" && spec.BindAddress != "::1") || spec.Port == 0 ||
		spec.ExpectedStatusCode < 200 || spec.ExpectedStatusCode >= 300 || !validServiceProbeDigest(spec.ExpectedBodySHA256) ||
		spec.TimeoutMS < 100 || spec.TimeoutMS > 5000 || spec.LeaseDurationMS < 1000 || spec.LeaseDurationMS > 600000 || spec.LeaseDurationMS < spec.TimeoutMS {
		return false
	}
	if spec.Path == "" || len(spec.Path) > 256 || !utf8.ValidString(spec.Path) || strings.HasPrefix(spec.Path, "//") || !strings.HasPrefix(spec.Path, "/") || strings.ContainsAny(spec.Path, "\\?#%\x00") || strings.IndexFunc(spec.Path, unicode.IsControl) >= 0 || path.Clean(spec.Path) != spec.Path {
		return false
	}
	for _, segment := range strings.Split(spec.Path, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func validServiceProbeDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil && strings.ToLower(value) == value
}

func digestServiceProbeBytes(content []byte) string {
	digest := sha256.Sum256(content)
	return hex.EncodeToString(digest[:])
}

func serviceProbeTransportReason(ctx context.Context, err error) string {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "probe_timeout"
	}
	var netError net.Error
	if errors.As(err, &netError) && netError.Timeout() {
		return "probe_timeout"
	}
	return "connection_failed"
}

func itoaPort(port uint16) string {
	return strconv.Itoa(int(port))
}
