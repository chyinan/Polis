// pattern: Functional Core
package probe

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"polis/internal/codex"
)

type T16NamespaceRelation string

const (
	T16NamespaceSame      T16NamespaceRelation = "SAME"
	T16NamespaceDifferent T16NamespaceRelation = "DIFFERENT"
	T16NamespaceUnknown   T16NamespaceRelation = "UNKNOWN"
)

type T16RouteRelation string

const (
	T16RouteSame      T16RouteRelation = "SAME"
	T16RouteDifferent T16RouteRelation = "DIFFERENT"
	T16RouteUnknown   T16RouteRelation = "UNKNOWN"
)

type T16NamespaceIdentity struct {
	Target         string `json:"target"`
	Inode          string `json:"namespace_inode"`
	ProcEntryInode string `json:"proc_entry_inode"`
}

type T16ResolverSummary struct {
	MetadataClassification string   `json:"metadata_classification"`
	ContentDigest          string   `json:"content_digest"`
	DNSServerCategories    []string `json:"dns_server_categories"`
	SearchDomainPresent    bool     `json:"search_domain_present"`
}

type T16ProcNetSummary struct {
	RouteEntries int    `json:"route_entries"`
	TCPEntries   int    `json:"tcp_entries"`
	TCP6Entries  int    `json:"tcp6_entries"`
	StateDigest  string `json:"state_digest"`
}

type T16NetworkInventory struct {
	InterfaceCategories   []string           `json:"interface_categories"`
	LoopbackState         string             `json:"loopback_state"`
	DefaultRoutePresent   bool               `json:"default_route_present"`
	DefaultRouteInterface string             `json:"default_route_interface,omitempty"`
	RouteEntries          int                `json:"route_entries"`
	Resolver              T16ResolverSummary `json:"resolver"`
	ProcNet               T16ProcNetSummary  `json:"proc_net"`
	Topology              string             `json:"topology"`
}

func ParseT16Inventory(raw string) (T16NetworkInventory, error) {
	var inventory T16NetworkInventory
	section := ""
	var resolverCategories []string
	for _, rawLine := range strings.Split(raw, "\n") {
		line := strings.TrimSpace(rawLine)
		switch line {
		case "IFACES_BEGIN", "ROUTES_BEGIN", "RESOLV_BEGIN":
			section = line
			continue
		case "IFACES_END", "ROUTES_END", "RESOLV_END":
			section = ""
			continue
		}
		if line == "" {
			continue
		}
		switch section {
		case "IFACES_BEGIN":
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			categories := make([]string, 0, 2)
			for _, address := range fields[2:] {
				switch {
				case strings.Contains(address, "/") && strings.Contains(address, "."):
					categories = append(categories, "ipv4")
				case strings.Contains(address, "/") && strings.Contains(address, ":"):
					categories = append(categories, "ipv6")
				}
			}
			sort.Strings(categories)
			inventory.InterfaceCategories = append(inventory.InterfaceCategories, fields[0]+":"+fields[1]+":"+strings.Join(categories, ","))
			if fields[0] == "lo" {
				inventory.LoopbackState = fields[1] + ":" + strings.Join(categories, ",")
			}
		case "ROUTES_BEGIN":
			fields := strings.Fields(line)
			inventory.RouteEntries++
			if len(fields) > 0 && fields[0] == "default" {
				inventory.DefaultRoutePresent = true
				for index, field := range fields {
					if field == "dev" && index+1 < len(fields) {
						inventory.DefaultRouteInterface = fields[index+1]
						break
					}
				}
			}
		case "RESOLV_BEGIN":
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "nameserver" {
				resolverCategories = append(resolverCategories, classifyT16DNSServer(fields[1]))
			}
			if len(fields) >= 2 && fields[0] == "search" {
				inventory.Resolver.SearchDomainPresent = true
			}
		default:
			if strings.HasPrefix(line, "RESOLV_META ") {
				inventory.Resolver.MetadataClassification = classifyT16ResolverMetadata(strings.TrimPrefix(line, "RESOLV_META "))
			}
			if strings.HasPrefix(line, "PROC ") {
				for _, field := range strings.Fields(strings.TrimPrefix(line, "PROC ")) {
					parts := strings.SplitN(field, "=", 2)
					if len(parts) != 2 {
						continue
					}
					value, err := strconv.Atoi(parts[1])
					if err != nil {
						return T16NetworkInventory{}, fmt.Errorf("invalid proc network count: %s", field)
					}
					switch parts[0] {
					case "route":
						inventory.ProcNet.RouteEntries = value
					case "tcp":
						inventory.ProcNet.TCPEntries = value
					case "tcp6":
						inventory.ProcNet.TCP6Entries = value
					}
				}
			}
		}
	}
	if len(inventory.InterfaceCategories) == 0 {
		return T16NetworkInventory{}, errors.New("missing interface inventory")
	}
	if inventory.LoopbackState == "" {
		inventory.LoopbackState = "not_observed"
	}
	inventory.Resolver.DNSServerCategories = append([]string(nil), resolverCategories...)
	canonical := append([]string(nil), inventory.InterfaceCategories...)
	sort.Strings(canonical)
	state := fmt.Sprintf("interfaces=%s;default=%t;dev=%s;routes=%d;resolver=%s;dns=%s;search=%t;proc=%d/%d/%d", strings.Join(canonical, ","), inventory.DefaultRoutePresent, inventory.DefaultRouteInterface, inventory.RouteEntries, inventory.Resolver.MetadataClassification, strings.Join(resolverCategories, ","), inventory.Resolver.SearchDomainPresent, inventory.ProcNet.RouteEntries, inventory.ProcNet.TCPEntries, inventory.ProcNet.TCP6Entries)
	digest := sha256.Sum256([]byte(state))
	inventory.Resolver.ContentDigest = hex.EncodeToString(digest[:])
	inventory.ProcNet.StateDigest = hex.EncodeToString(digest[:])
	inventory.Topology = ClassifyT16Topology(inventory, T16NamespaceUnknown)
	return inventory, nil
}

func CompareT16RouteInventory(host, guest T16NetworkInventory) T16RouteRelation {
	if host.DefaultRouteInterface == "" && guest.DefaultRouteInterface == "" && host.DefaultRoutePresent == guest.DefaultRoutePresent && host.RouteEntries == guest.RouteEntries {
		return T16RouteSame
	}
	if host.DefaultRoutePresent != guest.DefaultRoutePresent || host.DefaultRouteInterface != guest.DefaultRouteInterface || host.RouteEntries != guest.RouteEntries {
		return T16RouteDifferent
	}
	hostInterfaces := append([]string(nil), host.InterfaceCategories...)
	guestInterfaces := append([]string(nil), guest.InterfaceCategories...)
	sort.Strings(hostInterfaces)
	sort.Strings(guestInterfaces)
	if strings.Join(hostInterfaces, "\x00") != strings.Join(guestInterfaces, "\x00") {
		return T16RouteDifferent
	}
	return T16RouteSame
}

func ClassifyT16Topology(inventory T16NetworkInventory, relation T16NamespaceRelation) string {
	nonLoopback := false
	hasVeth := false
	for _, category := range inventory.InterfaceCategories {
		name := strings.SplitN(category, ":", 2)[0]
		if name != "lo" {
			nonLoopback = true
		}
		if strings.HasPrefix(name, "veth") {
			hasVeth = true
		}
	}
	if !nonLoopback {
		return "loopback_only"
	}
	if !inventory.DefaultRoutePresent {
		return "no_default_route"
	}
	if relation == T16NamespaceSame {
		return "shared_wsl_interfaces"
	}
	if relation == T16NamespaceDifferent && hasVeth {
		return "isolated_veth"
	}
	if relation == T16NamespaceDifferent {
		return "isolated_nonloopback_with_default_route"
	}
	return "networked_with_default_route"
}

func ClassifyT16NetworkPolicy(flags []string, relation T16NamespaceRelation) (codex.NetworkNamespacePolicy, error) {
	hasUnshareAll, hasUnshareNet, hasShareNet := false, false, false
	for _, flag := range flags {
		switch flag {
		case "--unshare-all":
			hasUnshareAll = true
		case "--unshare-net":
			hasUnshareNet = true
		case "--share-net":
			hasShareNet = true
		}
	}
	if hasShareNet && relation == T16NamespaceSame {
		return codex.NetworkNamespacePolicySharedHostNetwork, nil
	}
	if hasShareNet && relation == T16NamespaceDifferent {
		return "", errors.New("network policy flags claim shared network but namespace evidence differs")
	}
	if (hasUnshareNet || hasUnshareAll) && relation == T16NamespaceDifferent {
		return codex.NetworkNamespacePolicyIsolatedNetworkNamespace, nil
	}
	return "", errors.New("network namespace policy cannot be qualified from flags and observed relation")
}

func classifyT16DNSServer(value string) string {
	ip := net.ParseIP(value)
	if ip == nil {
		return "opaque"
	}
	if ip.IsLoopback() {
		return "loopback"
	}
	if ip.IsPrivate() {
		if ip.To4() != nil {
			return "private_ipv4"
		}
		return "private_ipv6"
	}
	if ip.To4() != nil {
		return "ipv4"
	}
	return "ipv6"
}

func classifyT16ResolverMetadata(value string) string {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return "not_observed"
	}
	return strings.Join(fields[:minT16Int(2, len(fields))], " ")
}

func minT16Int(a, b int) int {
	if a < b {
		return a
	}
	return b
}
