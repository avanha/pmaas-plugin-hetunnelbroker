package common

import "github.com/avanha/pmaas-plugin-hetunnelbroker/data"

type StatusAndEntities struct {
	Status  data.PluginStatus
	Tunnels []data.TunnelData
}

type EntityStore interface {
	GetStatusAndEntities() (StatusAndEntities, error)
}
