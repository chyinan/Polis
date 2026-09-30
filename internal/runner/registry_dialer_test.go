package runner

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"testing"
)

func TestRegistryHostDialerPinsExactHostToResolvedAddress(t *testing.T) {
	connector := &memoryRegistryDialer{serve: func(connection net.Conn) { _ = connection.Close() }}
	resolverNetwork := ""
	dialer := registryHostDialer{
		resolver:  fixedRegistryResolver{addresses: []netip.Addr{netip.MustParseAddr("93.184.216.34")}, observedNetwork: &resolverNetwork},
		connector: connector,
	}
	connection, err := dialer.DialContext(context.Background(), "tcp", "registry.npmjs.org:443")
	if err != nil {
		t.Fatal(err)
	}
	_ = connection.Close()
	if target := connector.lastTarget(); target != "93.184.216.34:443" {
		t.Fatalf("dialer re-resolved hostname instead of using its checked address: %q", target)
	}
	if resolverNetwork != "ip4" {
		t.Fatalf("registry profile queried DNS network %q, want IPv4 only", resolverNetwork)
	}
}

func TestRegistryHostDialerRejectsPrivateRegistryIPInCurrentProfile(t *testing.T) {
	connector := &memoryRegistryDialer{serve: func(connection net.Conn) { _ = connection.Close() }}
	dialer := registryHostDialer{
		resolver:  fixedRegistryResolver{addresses: []netip.Addr{netip.MustParseAddr("10.20.0.15")}},
		connector: connector,
	}
	if _, err := dialer.DialContext(context.Background(), "tcp", "registry.corp.example:443"); !errors.Is(err, errRegistryProxyPolicy) {
		t.Fatalf("private registry error = %v, want deny", err)
	}
	if calls := connector.callCount(); calls != 0 {
		t.Fatalf("private registry reached connector %d times", calls)
	}
}

func TestRegistryHostDialerRejectsLoopbackLinkLocalAndMixedAnswers(t *testing.T) {
	for _, test := range []struct {
		name      string
		addresses []netip.Addr
	}{
		{name: "IPv4 loopback", addresses: []netip.Addr{netip.MustParseAddr("127.0.0.1")}},
		{name: "IPv4 shorthand result", addresses: []netip.Addr{netip.MustParseAddr("127.0.0.1")}},
		{name: "link local metadata address", addresses: []netip.Addr{netip.MustParseAddr("169.254.169.254")}},
		{name: "unspecified", addresses: []netip.Addr{netip.IPv4Unspecified()}},
		{name: "multicast", addresses: []netip.Addr{netip.MustParseAddr("224.0.0.1")}},
		{name: "shared address space", addresses: []netip.Addr{netip.MustParseAddr("100.64.0.1")}},
		{name: "protocol assignment range", addresses: []netip.Addr{netip.MustParseAddr("192.0.0.1")}},
		{name: "documentation range", addresses: []netip.Addr{netip.MustParseAddr("192.0.2.1")}},
		{name: "deprecated relay range", addresses: []netip.Addr{netip.MustParseAddr("192.88.99.1")}},
		{name: "benchmark range", addresses: []netip.Addr{netip.MustParseAddr("198.18.0.1")}},
		{name: "private registry requires a distinct profile", addresses: []netip.Addr{netip.MustParseAddr("10.20.0.15")}},
		{name: "mixed DNS answers", addresses: []netip.Addr{netip.MustParseAddr("93.184.216.34"), netip.MustParseAddr("127.0.0.1")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			connector := &memoryRegistryDialer{}
			dialer := registryHostDialer{resolver: fixedRegistryResolver{addresses: test.addresses}, connector: connector}
			if _, err := dialer.DialContext(context.Background(), "tcp", "registry.npmjs.org:443"); !errors.Is(err, errRegistryProxyPolicy) {
				t.Fatalf("registry dial error = %v, want deny", err)
			}
			if calls := connector.callCount(); calls != 0 {
				t.Fatalf("unsafe DNS answers reached connector %d times", calls)
			}
		})
	}
}

func TestRegistryHostDialerAllowsGloballyReachableSpecialExceptions(t *testing.T) {
	for _, address := range []netip.Addr{netip.MustParseAddr("192.0.0.9"), netip.MustParseAddr("192.0.0.10")} {
		t.Run(address.String(), func(t *testing.T) {
			connector := &memoryRegistryDialer{serve: func(connection net.Conn) { _ = connection.Close() }}
			dialer := registryHostDialer{resolver: fixedRegistryResolver{addresses: []netip.Addr{address}}, connector: connector}
			connection, err := dialer.DialContext(context.Background(), "tcp", "registry.npmjs.org:443")
			if err != nil {
				t.Fatalf("globally reachable special address was denied: %v", err)
			}
			_ = connection.Close()
			if connector.callCount() != 1 {
				t.Fatal("approved resolved address was not used")
			}
		})
	}
}

func TestRegistryHostDialerRejectsIPv6AnswersForIPv4OnlyProfile(t *testing.T) {
	connector := &memoryRegistryDialer{}
	dialer := registryHostDialer{resolver: fixedRegistryResolver{addresses: []netip.Addr{netip.MustParseAddr("64:ff9b::a00:1")}}, connector: connector}
	if _, err := dialer.DialContext(context.Background(), "tcp", "registry.npmjs.org:443"); !errors.Is(err, errRegistryProxyPolicy) {
		t.Fatalf("IPv6/NAT64 answer error = %v, want deny", err)
	}
	if connector.callCount() != 0 {
		t.Fatal("IPv6/NAT64 answer reached the connector")
	}
}

type fixedRegistryResolver struct {
	addresses       []netip.Addr
	err             error
	observedNetwork *string
}

func (r fixedRegistryResolver) LookupNetIP(_ context.Context, network, _ string) ([]netip.Addr, error) {
	if r.observedNetwork != nil {
		*r.observedNetwork = network
	}
	return append([]netip.Addr(nil), r.addresses...), r.err
}
