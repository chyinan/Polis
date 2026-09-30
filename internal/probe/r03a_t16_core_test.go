// pattern: Functional Core
package probe

import (
	"testing"

	"polis/internal/codex"
)

func TestParseT16InventoryRedactsAddressesAndClassifiesSharedTopology(t *testing.T) {
	raw := "IFACES_BEGIN\nlo UNKNOWN 127.0.0.1/8 ::1/128\neth0 UP 172.24.72.52/20 fe80::1/64\nIFACES_END\nROUTES_BEGIN\ndefault via 172.24.64.1 dev eth0 proto kernel\n172.24.64.0/20 dev eth0 scope link src 172.24.72.52\nROUTES_END\nRESOLV_META 777 symbolic_link 20\nRESOLV_BEGIN\nnameserver 10.0.0.2\nsearch example.ts.net\nRESOLV_END\nPROC route=3 tcp=2 tcp6=1\n"
	inventory, err := ParseT16Inventory(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !inventory.DefaultRoutePresent || inventory.DefaultRouteInterface != "eth0" {
		t.Fatalf("default route = %+v", inventory)
	}
	if inventory.Resolver.DNSServerCategories[0] != "private_ipv4" || !inventory.Resolver.SearchDomainPresent {
		t.Fatalf("resolver classification = %+v", inventory.Resolver)
	}
	if inventory.Topology != "networked_with_default_route" {
		t.Fatalf("topology = %s", inventory.Topology)
	}
	for _, category := range inventory.InterfaceCategories {
		if category == "172.24.72.52" || category == "10.0.0.2" || category == "example.ts.net" {
			t.Fatal("raw network address leaked into classification")
		}
	}
}

func TestClassifyT16NetworkPolicyRequiresObservedNamespaceRelation(t *testing.T) {
	flags := []string{"--unshare-all", "--share-net"}
	policy, err := ClassifyT16NetworkPolicy(flags, T16NamespaceSame)
	if err != nil || policy != codex.NetworkNamespacePolicySharedHostNetwork {
		t.Fatalf("shared policy = %q err=%v", policy, err)
	}
	if _, err := ClassifyT16NetworkPolicy(flags, T16NamespaceUnknown); err == nil {
		t.Fatal("unknown namespace relation was accepted as a qualified policy")
	}
}

func TestT16RouteRelationAndTopologyDoNotTreatMissingDefaultAsShared(t *testing.T) {
	host := T16NetworkInventory{InterfaceCategories: []string{"lo:UNKNOWN:ipv4,ipv6", "eth0:UP:ipv4"}, DefaultRoutePresent: true, DefaultRouteInterface: "eth0"}
	guest := T16NetworkInventory{InterfaceCategories: []string{"lo:UNKNOWN:ipv4,ipv6"}, DefaultRoutePresent: false}
	if relation := CompareT16RouteInventory(host, guest); relation != T16RouteDifferent {
		t.Fatalf("route relation = %s", relation)
	}
	if topology := ClassifyT16Topology(guest, T16NamespaceDifferent); topology != "loopback_only" {
		t.Fatalf("guest topology = %s", topology)
	}
}
