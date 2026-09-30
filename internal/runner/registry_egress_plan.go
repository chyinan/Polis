// pattern: Functional Core
package runner

import (
	"errors"
	"net"
	"net/netip"
	"strconv"
)

const (
	AppContainerNetworkRegistryOnly            = "registry_allowlist"
	AppContainerInternetClientCapability       = "internetClient"
	RegistryPermitWeight                 uint8 = 15
	RegistryBlockWeight                  uint8 = 0
)

var errInvalidRegistryEgressEndpoint = errors.New("registry egress proxy endpoint must be an IPv4 loopback address and a valid port")

type RegistryFilterLayer string
type RegistryFilterAction string
type RegistryFilterField string
type RegistryFilterMatch string

const (
	RegistryFilterLayerIPv4Connect RegistryFilterLayer = "ALE_AUTH_CONNECT_V4"
	RegistryFilterLayerIPv6Connect RegistryFilterLayer = "ALE_AUTH_CONNECT_V6"

	RegistryFilterPermit RegistryFilterAction = "permit"
	RegistryFilterBlock  RegistryFilterAction = "block"

	RegistryFilterAppContainerSID RegistryFilterField = "ALE_PACKAGE_ID"
	RegistryFilterRemoteIPv4      RegistryFilterField = "IP_REMOTE_ADDRESS_V4"
	RegistryFilterProtocol        RegistryFilterField = "IP_PROTOCOL"
	RegistryFilterRemotePort      RegistryFilterField = "IP_REMOTE_PORT"

	RegistryFilterEqual RegistryFilterMatch = "equal"
)

type RegistryFilterCondition struct {
	Field RegistryFilterField
	Match RegistryFilterMatch
	Value string
}

type RegistryEgressFilter struct {
	Layer      RegistryFilterLayer
	Action     RegistryFilterAction
	Weight     uint8
	Conditions []RegistryFilterCondition
}

type RegistryEgressPlan struct {
	ProxyAddress string
	ProxyPort    uint16
	Capabilities []string
	Filters      []RegistryEgressFilter
}

func BuildRegistryEgressPlan(proxyEndpoint string) (RegistryEgressPlan, error) {
	host, portText, err := net.SplitHostPort(proxyEndpoint)
	if err != nil {
		return RegistryEgressPlan{}, errInvalidRegistryEgressEndpoint
	}
	address, err := netip.ParseAddr(host)
	if err != nil || !address.Is4() || address != netip.MustParseAddr("127.0.0.1") {
		return RegistryEgressPlan{}, errInvalidRegistryEgressEndpoint
	}
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil || port == 0 {
		return RegistryEgressPlan{}, errInvalidRegistryEgressEndpoint
	}
	portValue := strconv.FormatUint(port, 10)
	packageCondition := RegistryFilterCondition{Field: RegistryFilterAppContainerSID, Match: RegistryFilterEqual}
	return RegistryEgressPlan{
		ProxyAddress: address.String(), ProxyPort: uint16(port),
		Capabilities: []string{AppContainerInternetClientCapability},
		Filters: []RegistryEgressFilter{
			{
				Layer: RegistryFilterLayerIPv4Connect, Action: RegistryFilterPermit, Weight: RegistryPermitWeight,
				Conditions: []RegistryFilterCondition{
					packageCondition,
					{Field: RegistryFilterRemoteIPv4, Match: RegistryFilterEqual, Value: address.String()},
					{Field: RegistryFilterProtocol, Match: RegistryFilterEqual, Value: "tcp"},
					{Field: RegistryFilterRemotePort, Match: RegistryFilterEqual, Value: portValue},
				},
			},
			{Layer: RegistryFilterLayerIPv4Connect, Action: RegistryFilterBlock, Weight: RegistryBlockWeight, Conditions: []RegistryFilterCondition{packageCondition}},
			{Layer: RegistryFilterLayerIPv6Connect, Action: RegistryFilterBlock, Weight: RegistryBlockWeight, Conditions: []RegistryFilterCondition{packageCondition}},
		},
	}, nil
}
