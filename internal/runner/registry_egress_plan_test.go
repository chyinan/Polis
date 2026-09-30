// pattern: Functional Core
package runner

import "testing"

func TestRegistryEgressPlanAllowsOnlyTheBoundIPv4ProxyEndpoint(t *testing.T) {
	plan, err := BuildRegistryEgressPlan("127.0.0.1:43123")
	if err != nil {
		t.Fatal(err)
	}
	if plan.ProxyAddress != "127.0.0.1" || plan.ProxyPort != 43123 || len(plan.Capabilities) != 1 || plan.Capabilities[0] != AppContainerInternetClientCapability {
		t.Fatalf("registry egress binding = %+v", plan)
	}
	if len(plan.Filters) != 3 {
		t.Fatalf("registry egress filter count = %d, want 3", len(plan.Filters))
	}
	permit := plan.Filters[0]
	if permit.Layer != RegistryFilterLayerIPv4Connect || permit.Action != RegistryFilterPermit || permit.Weight != RegistryPermitWeight {
		t.Fatalf("proxy permit filter = %+v", permit)
	}
	wantPermitConditions := []RegistryFilterCondition{
		{Field: RegistryFilterAppContainerSID, Match: RegistryFilterEqual},
		{Field: RegistryFilterRemoteIPv4, Match: RegistryFilterEqual, Value: "127.0.0.1"},
		{Field: RegistryFilterProtocol, Match: RegistryFilterEqual, Value: "tcp"},
		{Field: RegistryFilterRemotePort, Match: RegistryFilterEqual, Value: "43123"},
	}
	if !equalRegistryFilterConditions(permit.Conditions, wantPermitConditions) {
		t.Fatalf("proxy permit conditions = %+v", permit.Conditions)
	}
	for index, wantLayer := range []RegistryFilterLayer{RegistryFilterLayerIPv4Connect, RegistryFilterLayerIPv6Connect} {
		filter := plan.Filters[index+1]
		if filter.Layer != wantLayer || filter.Action != RegistryFilterBlock || filter.Weight != RegistryBlockWeight || len(filter.Conditions) != 1 || filter.Conditions[0].Field != RegistryFilterAppContainerSID {
			t.Fatalf("default-deny filter %d = %+v", index, filter)
		}
	}
}

func TestRegistryEgressPlanRejectsEveryNonLoopbackOrMalformedEndpoint(t *testing.T) {
	for _, endpoint := range []string{"", "localhost:43123", "8.8.8.8:443", "[::1]:443", "127.0.0.1:0", "127.0.0.1:65536", "127.0.0.1:abc", "127.0.0.1:443/path"} {
		t.Run(endpoint, func(t *testing.T) {
			if _, err := BuildRegistryEgressPlan(endpoint); err == nil {
				t.Fatalf("invalid registry proxy endpoint %q was accepted", endpoint)
			}
		})
	}
}

func equalRegistryFilterConditions(actual, expected []RegistryFilterCondition) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return false
		}
	}
	return true
}
