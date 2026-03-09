package data

import (
	"net"
	"time"
)

type TunnelData struct {
	Name               string
	LastUpdateTime     time.Time
	LastModifiedTime   time.Time
	GetSuccessCount    int
	GetErrorCount      int
	UpdateSuccessCount int
	UpdateErrorCount   int
	LastError          error
	LastErrorTime      time.Time
	Description        string
	ServerIpV4Address  net.IP
	ServerIpV6Address  net.IP
	ClientIpV4Address  net.IP
	ClientIpV6Address  net.IP
	Routed64Net        net.IPNet
	Routed48Net        net.IPNet
}
