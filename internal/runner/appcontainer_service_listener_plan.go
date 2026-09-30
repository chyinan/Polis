// pattern: Functional Core
package runner

import (
	"errors"
	"net/netip"
	"strconv"
)

const AppContainerInternetClientServerCapability = "internetClientServer"

const (
	ServiceListenerPermitWeight uint8 = 15
	ServiceListenerBlockWeight  uint8 = 0
)

type AppContainerServiceListenerSpec struct {
	BindAddress string
	Port        uint16
}

type ServiceListenerFilterLayer string
type ServiceListenerFilterAction string
type ServiceListenerFilterField string
type ServiceListenerFilterMatch string

const (
	ServiceListenerLayerIPv4ResourceAssignment ServiceListenerFilterLayer = "ALE_RESOURCE_ASSIGNMENT_V4"
	ServiceListenerLayerIPv6ResourceAssignment ServiceListenerFilterLayer = "ALE_RESOURCE_ASSIGNMENT_V6"
	ServiceListenerLayerIPv4Listen             ServiceListenerFilterLayer = "ALE_AUTH_LISTEN_V4"
	ServiceListenerLayerIPv6Listen             ServiceListenerFilterLayer = "ALE_AUTH_LISTEN_V6"
	ServiceListenerLayerIPv4Receive            ServiceListenerFilterLayer = "ALE_AUTH_RECV_ACCEPT_V4"
	ServiceListenerLayerIPv6Receive            ServiceListenerFilterLayer = "ALE_AUTH_RECV_ACCEPT_V6"
	ServiceListenerLayerIPv4Connect            ServiceListenerFilterLayer = "ALE_AUTH_CONNECT_V4"
	ServiceListenerLayerIPv6Connect            ServiceListenerFilterLayer = "ALE_AUTH_CONNECT_V6"

	ServiceListenerFilterPermit ServiceListenerFilterAction = "permit"
	ServiceListenerFilterBlock  ServiceListenerFilterAction = "block"
	ServiceListenerFilterEqual  ServiceListenerFilterMatch  = "equal"

	ServiceListenerFieldAppContainerSID ServiceListenerFilterField = "ALE_PACKAGE_ID"
	ServiceListenerFieldLocalIPv4       ServiceListenerFilterField = "IP_LOCAL_ADDRESS_V4"
	ServiceListenerFieldRemoteIPv4      ServiceListenerFilterField = "IP_REMOTE_ADDRESS_V4"
	ServiceListenerFieldLocalIPv6       ServiceListenerFilterField = "IP_LOCAL_ADDRESS_V6"
	ServiceListenerFieldRemoteIPv6      ServiceListenerFilterField = "IP_REMOTE_ADDRESS_V6"
	ServiceListenerFieldProtocol        ServiceListenerFilterField = "IP_PROTOCOL"
	ServiceListenerFieldLocalPort       ServiceListenerFilterField = "IP_LOCAL_PORT"
)

var ErrInvalidAppContainerServiceListener = errors.New("AppContainer service listener must use a fixed TCP port on 127.0.0.1 or ::1")

type ServiceListenerFilterCondition struct {
	Field ServiceListenerFilterField
	Match ServiceListenerFilterMatch
	Value string
}

type ServiceListenerFilter struct {
	Layer      ServiceListenerFilterLayer
	Action     ServiceListenerFilterAction
	Weight     uint8
	Conditions []ServiceListenerFilterCondition
}

type AppContainerServiceListenerPlan struct {
	BindAddress  string
	Port         uint16
	Capabilities []string
	Filters      []ServiceListenerFilter
}

func BuildAppContainerServiceListenerPlan(spec AppContainerServiceListenerSpec) (AppContainerServiceListenerPlan, error) {
	address, err := netip.ParseAddr(spec.BindAddress)
	if err != nil || spec.Port == 0 || address.Zone() != "" || address.Is4In6() {
		return AppContainerServiceListenerPlan{}, ErrInvalidAppContainerServiceListener
	}
	if address.String() != "127.0.0.1" && address.String() != "::1" {
		return AppContainerServiceListenerPlan{}, ErrInvalidAppContainerServiceListener
	}
	port := strconv.FormatUint(uint64(spec.Port), 10)
	packageCondition := ServiceListenerFilterCondition{Field: ServiceListenerFieldAppContainerSID, Match: ServiceListenerFilterEqual}
	protocolCondition := ServiceListenerFilterCondition{Field: ServiceListenerFieldProtocol, Match: ServiceListenerFilterEqual, Value: "tcp"}
	portCondition := ServiceListenerFilterCondition{Field: ServiceListenerFieldLocalPort, Match: ServiceListenerFilterEqual, Value: port}
	var localAddress, remoteAddress ServiceListenerFilterCondition
	var resourceLayer, listenLayer, receiveLayer ServiceListenerFilterLayer
	var connect4, connect6 ServiceListenerFilterLayer
	if address.Is4() {
		localAddress = ServiceListenerFilterCondition{Field: ServiceListenerFieldLocalIPv4, Match: ServiceListenerFilterEqual, Value: address.String()}
		remoteAddress = ServiceListenerFilterCondition{Field: ServiceListenerFieldRemoteIPv4, Match: ServiceListenerFilterEqual, Value: address.String()}
		resourceLayer, listenLayer, receiveLayer = ServiceListenerLayerIPv4ResourceAssignment, ServiceListenerLayerIPv4Listen, ServiceListenerLayerIPv4Receive
	} else {
		localAddress = ServiceListenerFilterCondition{Field: ServiceListenerFieldLocalIPv6, Match: ServiceListenerFilterEqual, Value: address.String()}
		remoteAddress = ServiceListenerFilterCondition{Field: ServiceListenerFieldRemoteIPv6, Match: ServiceListenerFilterEqual, Value: address.String()}
		resourceLayer, listenLayer, receiveLayer = ServiceListenerLayerIPv6ResourceAssignment, ServiceListenerLayerIPv6Listen, ServiceListenerLayerIPv6Receive
	}
	connect4, connect6 = ServiceListenerLayerIPv4Connect, ServiceListenerLayerIPv6Connect
	permit := func(layer ServiceListenerFilterLayer, conditions ...ServiceListenerFilterCondition) ServiceListenerFilter {
		return ServiceListenerFilter{Layer: layer, Action: ServiceListenerFilterPermit, Weight: ServiceListenerPermitWeight, Conditions: conditions}
	}
	block := func(layer ServiceListenerFilterLayer) ServiceListenerFilter {
		return ServiceListenerFilter{Layer: layer, Action: ServiceListenerFilterBlock, Weight: ServiceListenerBlockWeight, Conditions: []ServiceListenerFilterCondition{packageCondition}}
	}
	return AppContainerServiceListenerPlan{
		BindAddress: address.String(), Port: spec.Port,
		Capabilities: []string{AppContainerInternetClientServerCapability},
		Filters: []ServiceListenerFilter{
			permit(resourceLayer, packageCondition, localAddress, protocolCondition, portCondition),
			permit(listenLayer, packageCondition, localAddress, portCondition),
			permit(receiveLayer, packageCondition, localAddress, remoteAddress, protocolCondition, portCondition),
			block(ServiceListenerLayerIPv4ResourceAssignment), block(ServiceListenerLayerIPv6ResourceAssignment),
			block(ServiceListenerLayerIPv4Listen), block(ServiceListenerLayerIPv6Listen),
			block(ServiceListenerLayerIPv4Receive), block(ServiceListenerLayerIPv6Receive),
			block(connect4), block(connect6),
		},
	}, nil
}
