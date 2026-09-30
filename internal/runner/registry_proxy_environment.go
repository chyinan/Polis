// pattern: Functional Core
package runner

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func BuildRegistryProxyEnvironment(current []string, proxyURL string) ([]string, error) {
	if _, err := validateRegistryProxyURL(proxyURL); err != nil {
		return nil, err
	}
	result := append([]string(nil), current...)
	found := map[string]bool{}
	for _, entry := range result {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			return nil, errors.New("AppContainer environment contains an invalid proxy setting")
		}
		switch strings.ToUpper(name) {
		case "HTTP_PROXY", "HTTPS_PROXY":
			if value != proxyURL || found[strings.ToUpper(name)] {
				return nil, errors.New("AppContainer proxy environment conflicts with its registry lease")
			}
			found[strings.ToUpper(name)] = true
		case "NO_PROXY":
			if strings.TrimSpace(value) != "" {
				return nil, errors.New("AppContainer proxy bypass is not supported by the registry lease")
			}
		}
	}
	if !found["HTTP_PROXY"] {
		result = append(result, "HTTP_PROXY="+proxyURL)
	}
	if !found["HTTPS_PROXY"] {
		result = append(result, "HTTPS_PROXY="+proxyURL)
	}
	return result, nil
}

func validateRegistryProxyURL(proxyURL string) (RegistryEgressPlan, error) {
	parsed, err := url.Parse(proxyURL)
	if err != nil || parsed.Scheme != "http" || parsed.Opaque != "" || parsed.User == nil || parsed.Hostname() != "127.0.0.1" || parsed.Port() == "" || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return RegistryEgressPlan{}, errors.New("registry egress proxy URL must include lease credentials and a loopback endpoint")
	}
	if username := parsed.User.Username(); username == "" {
		return RegistryEgressPlan{}, errors.New("registry egress proxy URL must include lease credentials and a loopback endpoint")
	}
	if password, ok := parsed.User.Password(); !ok || password == "" {
		return RegistryEgressPlan{}, errors.New("registry egress proxy URL must include lease credentials and a loopback endpoint")
	}
	address := net.JoinHostPort(parsed.Hostname(), parsed.Port())
	plan, err := BuildRegistryEgressPlan(address)
	if err != nil {
		return RegistryEgressPlan{}, err
	}
	canonical := (&url.URL{Scheme: "http", User: parsed.User, Host: address}).String()
	if canonical != proxyURL {
		return RegistryEgressPlan{}, errors.New("registry egress proxy URL is not canonical")
	}
	return plan, nil
}

func formatRegistryProxyPort(port uint16) string {
	return strconv.FormatUint(uint64(port), 10)
}
