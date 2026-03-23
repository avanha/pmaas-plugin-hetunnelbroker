package common

import (
	"net"

	"github.com/avanha/pmaas-plugin-hetunnelbroker/data"
)

const (
	BrokerRequestTypeGetTunnelInfo = iota + 1
	BrokerRequestTypeUpdateTunnelClientIpV4Address
)

type BrokerRequest struct {
	RequestType                          int
	ResultCh                             chan BrokerResult
	GetTunnelInfoRequest                 GetTunnelInfoRequest
	UpdateTunnelClientIpV4AddressRequest UpdateTunnelClientIpV4AddressRequest
}

type GetTunnelInfoRequest struct {
	TunnelId string
}

type UpdateTunnelClientIpV4AddressRequest struct {
	TunnelId    string
	CurrentData data.TunnelData
	NewAddress  net.IP
}
