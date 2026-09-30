// pattern: Imperative Shell
package runner

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"sort"
	"strings"
)

const (
	registryDialerMaxAddresses = 16
	// RegistryAddressPolicyRevision binds the fixed IPv4 special-use ranges.
	RegistryAddressPolicyRevision = "iana-ipv4-special-purpose@2025-10-09"
)

var registryNonGlobalPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"), netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"), netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"), netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"), netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("255.255.255.255/32"),
}

var registryGlobalSpecialExceptions = []netip.Prefix{
	netip.MustParsePrefix("192.0.0.9/32"), netip.MustParsePrefix("192.0.0.10/32"),
}

type registryAddressResolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type registryHostDialer struct {
	resolver  registryAddressResolver
	connector RegistryTunnelDialer
}

func newRegistryHostDialer() RegistryTunnelDialer {
	return registryHostDialer{resolver: net.DefaultResolver, connector: &net.Dialer{Timeout: registryProxyDialLimit, KeepAlive: 30}}
}

// DialContext pins the TCP dial to the resolved addresses checked here, so
// the OS resolver cannot return a different address after policy validation.
// The current Windows Node/npm profile is IPv4-only and rejects destinations
// in known non-globally-reachable IANA special-use ranges as of 2025-10-09.
// IPv6/DNS64 and private registries need a separately qualified policy.
func (d registryHostDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if ctx == nil || network != "tcp" || d.resolver == nil || d.connector == nil {
		return nil, errRegistryProxyPolicy
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil || port != "443" || !validRegistryProxyHostname(strings.ToLower(host)) {
		return nil, errRegistryProxyPolicy
	}
	addresses, err := d.resolver.LookupNetIP(ctx, "ip4", host)
	if err != nil || len(addresses) == 0 || len(addresses) > registryDialerMaxAddresses {
		return nil, errRegistryProxyPolicy
	}
	for _, address := range addresses {
		if !address.Is4() || forbiddenRegistryAddress(address) {
			return nil, errRegistryProxyPolicy
		}
	}
	addresses = uniqueSortedRegistryAddresses(addresses)
	var dialErrors []error
	for _, ip := range addresses {
		connection, dialErr := d.connector.DialContext(ctx, "tcp", net.JoinHostPort(ip.String(), port))
		if dialErr == nil {
			return connection, nil
		}
		dialErrors = append(dialErrors, dialErr)
		if ctx.Err() != nil {
			break
		}
	}
	return nil, errors.Join(dialErrors...)
}

func forbiddenRegistryAddress(address netip.Addr) bool {
	if !address.IsValid() || !address.Is4() || address.Zone() != "" {
		return true
	}
	if !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsMulticast() || address.IsUnspecified() {
		return true
	}
	if registryGlobalSpecialAddress(address) {
		return false
	}
	for _, prefix := range registryNonGlobalPrefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func registryGlobalSpecialAddress(address netip.Addr) bool {
	for _, prefix := range registryGlobalSpecialExceptions {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func uniqueSortedRegistryAddresses(addresses []netip.Addr) []netip.Addr {
	unique := make(map[netip.Addr]struct{}, len(addresses))
	result := make([]netip.Addr, 0, len(addresses))
	for _, address := range addresses {
		address = address.Unmap()
		if _, exists := unique[address]; exists {
			continue
		}
		unique[address] = struct{}{}
		result = append(result, address)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Compare(result[j]) < 0 })
	return result
}
