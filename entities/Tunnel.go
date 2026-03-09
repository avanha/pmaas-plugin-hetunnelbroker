package entities

import "reflect"

type Tunnel interface {
	Name() string
}

var TunnelType = reflect.TypeOf((*Tunnel)(nil)).Elem()
