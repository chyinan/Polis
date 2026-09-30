// pattern: Functional Core
package runner

import "net"

const (
	tcpListenerIPv4 = 4
	tcpListenerIPv6 = 6
)

type tcpListenerOwner struct {
	family    int
	address   net.IP
	port      uint16
	processID int
}

// tcpListenerOwnerIsUnambiguous accepts only exact loopback listeners owned by
// the authorized process. Wildcard listeners are rejected because they can
// receive the same traffic; IPv6 wildcard listeners can also accept IPv4 on
// dual-stack sockets.
func tcpListenerOwnerIsUnambiguous(rows []tcpListenerOwner, processID int, bindAddress string, port uint16) bool {
	if processID <= 0 || port == 0 {
		return false
	}
	target := net.ParseIP(bindAddress)
	if target == nil {
		return false
	}
	family := tcpListenerIPv6
	if target = target.To4(); target != nil {
		family = tcpListenerIPv4
	} else {
		target = net.ParseIP(bindAddress).To16()
	}
	if target == nil || (family == tcpListenerIPv4 && !target.Equal(net.ParseIP("127.0.0.1").To4())) ||
		(family == tcpListenerIPv6 && !target.Equal(net.ParseIP("::1").To16())) {
		return false
	}
	foundExact := false
	for _, row := range rows {
		if row.port != port || row.family != family || len(row.address) == 0 {
			continue
		}
		if row.address.IsUnspecified() {
			return false
		}
		if row.address.Equal(target) {
			if row.processID != processID {
				return false
			}
			foundExact = true
		}
	}
	if family == tcpListenerIPv4 {
		for _, row := range rows {
			if row.family != tcpListenerIPv6 || row.port != port || row.address == nil {
				continue
			}
			if row.address.IsUnspecified() {
				return false
			}
			if mappedAddress := row.address.To4(); mappedAddress != nil {
				if mappedAddress.IsUnspecified() || mappedAddress.Equal(target) && row.processID != processID {
					return false
				}
			}
		}
	}
	return foundExact
}
