// pattern: Functional Core
package runner

import "testing"

func TestBuildAppContainerServiceListenerPlanPinsOneLoopbackTCPPort(t *testing.T) {
	plan, err := BuildAppContainerServiceListenerPlan(AppContainerServiceListenerSpec{BindAddress: "127.0.0.1", Port: 43123})
	if err != nil {
		t.Fatal(err)
	}
	if plan.BindAddress != "127.0.0.1" || plan.Port != 43123 {
		t.Fatalf("service listener target = %s:%d", plan.BindAddress, plan.Port)
	}
	if len(plan.Capabilities) != 1 || plan.Capabilities[0] != AppContainerInternetClientServerCapability {
		t.Fatalf("service listener capabilities = %v", plan.Capabilities)
	}
	if len(plan.Filters) != 11 {
		t.Fatalf("service listener filter count = %d, want three exact permits and eight family-wide blocks", len(plan.Filters))
	}
	for index, filter := range plan.Filters {
		if index < 3 && filter.Action != ServiceListenerFilterPermit {
			t.Fatalf("filter %d action=%q, want permit before blocks", index, filter.Action)
		}
		if index >= 3 && filter.Action != ServiceListenerFilterBlock {
			t.Fatalf("filter %d action=%q, want block", index, filter.Action)
		}
	}
	resource := plan.Filters[0]
	if resource.Layer != ServiceListenerLayerIPv4ResourceAssignment || resource.Weight != ServiceListenerPermitWeight || !hasServiceListenerCondition(resource.Conditions, ServiceListenerFieldLocalIPv4, "127.0.0.1") || !hasServiceListenerCondition(resource.Conditions, ServiceListenerFieldLocalPort, "43123") {
		t.Fatalf("service listener resource-assignment permit = %+v", resource)
	}
	listen := plan.Filters[1]
	if listen.Layer != ServiceListenerLayerIPv4Listen || listen.Weight != ServiceListenerPermitWeight || !hasServiceListenerCondition(listen.Conditions, ServiceListenerFieldLocalIPv4, "127.0.0.1") || !hasServiceListenerCondition(listen.Conditions, ServiceListenerFieldLocalPort, "43123") || hasServiceListenerCondition(listen.Conditions, ServiceListenerFieldProtocol, "tcp") {
		t.Fatalf("service listener listen permit = %+v", listen)
	}
	receive := plan.Filters[2]
	if receive.Layer != ServiceListenerLayerIPv4Receive || receive.Weight != ServiceListenerPermitWeight || !hasServiceListenerCondition(receive.Conditions, ServiceListenerFieldLocalIPv4, "127.0.0.1") || !hasServiceListenerCondition(receive.Conditions, ServiceListenerFieldRemoteIPv4, "127.0.0.1") || !hasServiceListenerCondition(receive.Conditions, ServiceListenerFieldLocalPort, "43123") || !hasServiceListenerCondition(receive.Conditions, ServiceListenerFieldProtocol, "tcp") {
		t.Fatalf("service listener receive permit = %+v", receive)
	}
	for _, filter := range plan.Filters[3:] {
		if filter.Weight != ServiceListenerBlockWeight || len(filter.Conditions) != 1 || filter.Conditions[0].Field != ServiceListenerFieldAppContainerSID {
			t.Fatalf("unscoped service listener block = %+v", filter)
		}
	}
}

func TestBuildAppContainerServiceListenerPlanSupportsIPv6LoopbackWithoutIPv4Permit(t *testing.T) {
	plan, err := BuildAppContainerServiceListenerPlan(AppContainerServiceListenerSpec{BindAddress: "::1", Port: 52119})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Filters[0].Layer != ServiceListenerLayerIPv6ResourceAssignment || !hasServiceListenerCondition(plan.Filters[0].Conditions, ServiceListenerFieldLocalIPv6, "::1") {
		t.Fatalf("IPv6 resource-assignment permit = %+v", plan.Filters[0])
	}
	if plan.Filters[1].Layer != ServiceListenerLayerIPv6Listen || !hasServiceListenerCondition(plan.Filters[1].Conditions, ServiceListenerFieldLocalIPv6, "::1") {
		t.Fatalf("IPv6 listener permit = %+v", plan.Filters[1])
	}
	if plan.Filters[2].Layer != ServiceListenerLayerIPv6Receive || !hasServiceListenerCondition(plan.Filters[2].Conditions, ServiceListenerFieldRemoteIPv6, "::1") {
		t.Fatalf("IPv6 receive permit = %+v", plan.Filters[2])
	}
	for _, filter := range plan.Filters[:3] {
		if filter.Layer == ServiceListenerLayerIPv4Listen || filter.Layer == ServiceListenerLayerIPv4Receive {
			t.Fatalf("IPv6 service received an IPv4 permit: %+v", filter)
		}
	}
}

func TestBuildAppContainerServiceListenerPlanRejectsAnyUnboundedTarget(t *testing.T) {
	for _, spec := range []AppContainerServiceListenerSpec{
		{}, {BindAddress: "127.0.0.1", Port: 0}, {BindAddress: "0.0.0.0", Port: 43123},
		{BindAddress: "127.0.0.2", Port: 43123}, {BindAddress: "::", Port: 43123}, {BindAddress: "::2", Port: 43123}, {BindAddress: "192.0.2.1", Port: 43123},
		{BindAddress: "::ffff:127.0.0.1", Port: 43123}, {BindAddress: "fe80::1%4", Port: 43123},
	} {
		if _, err := BuildAppContainerServiceListenerPlan(spec); err == nil {
			t.Errorf("service listener plan accepted unbounded target %+v", spec)
		}
	}
}

func hasServiceListenerCondition(conditions []ServiceListenerFilterCondition, field ServiceListenerFilterField, value string) bool {
	for _, condition := range conditions {
		if condition.Field == field && condition.Match == ServiceListenerFilterEqual && condition.Value == value {
			return true
		}
	}
	return false
}
