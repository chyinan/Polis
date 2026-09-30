// pattern: Functional Core
package runner

import (
	"net"
	"testing"
)

func TestTCPListenerOwnerPolicyRejectsAmbiguousLoopbackPorts(t *testing.T) {
	const port = uint16(43127)
	v4 := func(address string, pid int) tcpListenerOwner {
		return tcpListenerOwner{family: tcpListenerIPv4, address: net.ParseIP(address).To4(), port: port, processID: pid}
	}
	v6 := func(address string, pid int) tcpListenerOwner {
		return tcpListenerOwner{family: tcpListenerIPv6, address: net.ParseIP(address).To16(), port: port, processID: pid}
	}
	tests := []struct {
		name    string
		address string
		rows    []tcpListenerOwner
		want    bool
	}{
		{name: "owned exact ipv4", address: "127.0.0.1", rows: []tcpListenerOwner{v4("127.0.0.1", 9)}, want: true},
		{name: "same process exact ipv4 rows", address: "127.0.0.1", rows: []tcpListenerOwner{v4("127.0.0.1", 9), v4("127.0.0.1", 9)}, want: true},
		{name: "competing exact ipv4 process", address: "127.0.0.1", rows: []tcpListenerOwner{v4("127.0.0.1", 9), v4("127.0.0.1", 10)}, want: false},
		{name: "ipv4 wildcard listener", address: "127.0.0.1", rows: []tcpListenerOwner{v4("127.0.0.1", 9), v4("0.0.0.0", 9)}, want: false},
		{name: "dual stack ipv6 wildcard listener", address: "127.0.0.1", rows: []tcpListenerOwner{v4("127.0.0.1", 9), v6("::", 10)}, want: false},
		{name: "competing ipv4 mapped ipv6 listener", address: "127.0.0.1", rows: []tcpListenerOwner{v4("127.0.0.1", 9), v6("::ffff:127.0.0.1", 10)}, want: false},
		{name: "ipv4 mapped wildcard listener", address: "127.0.0.1", rows: []tcpListenerOwner{v4("127.0.0.1", 9), v6("::ffff:0.0.0.0", 10)}, want: false},
		{name: "unrelated port ignored", address: "127.0.0.1", rows: []tcpListenerOwner{v4("127.0.0.1", 9), {family: tcpListenerIPv4, address: net.ParseIP("0.0.0.0").To4(), port: port + 1, processID: 10}}, want: true},
		{name: "owned exact ipv6", address: "::1", rows: []tcpListenerOwner{v6("::1", 9)}, want: true},
		{name: "competing exact ipv6 process", address: "::1", rows: []tcpListenerOwner{v6("::1", 9), v6("::1", 10)}, want: false},
		{name: "ipv6 wildcard listener", address: "::1", rows: []tcpListenerOwner{v6("::1", 9), v6("::", 9)}, want: false},
		{name: "missing exact listener", address: "::1", rows: []tcpListenerOwner{v6("::", 9)}, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := tcpListenerOwnerIsUnambiguous(test.rows, 9, test.address, port); got != test.want {
				t.Fatalf("unambiguous=%v, want %v", got, test.want)
			}
		})
	}
}
