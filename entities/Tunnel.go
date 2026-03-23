package entities

import (
	"net"
	"reflect"
)

type Tunnel interface {
	Name() string
	UpdateClientIpV4Address(value net.IP) error
}

var TunnelType = reflect.TypeOf((*Tunnel)(nil)).Elem()
