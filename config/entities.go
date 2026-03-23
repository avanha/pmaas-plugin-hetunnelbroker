package config

import (
	"fmt"
	"net"
	"slices"

	"github.com/avanha/pmaas-plugin-hetunnelbroker/entities"
	"github.com/avanha/pmaas-plugin-hetunnelbroker/events"
)

type Tunnel struct {
	Name                           string
	Id                             string
	onEntityStubAvailableListeners []func(event events.TunnelEntityStubAvailableEvent)
	entityStub                     entities.Tunnel
}

func NewTunnel(name string, id string) *Tunnel {
	tunnel := &Tunnel{
		Name:                           name,
		Id:                             id,
		onEntityStubAvailableListeners: make([]func(event events.TunnelEntityStubAvailableEvent), 0),
	}
	tunnel.AddOnEntityStubAvailableListener(tunnel.onEntityStubAvailable)

	return tunnel
}

func (t *Tunnel) UpdateClientIpV4Address(value net.IP) error {
	if t.entityStub == nil {
		return fmt.Errorf("unable to update tunnel %s: entity stub is not available", t.Name)
	}

	return t.entityStub.UpdateClientIpV4Address(value)
}

func (t *Tunnel) AddOnEntityStubAvailableListener(eventListener func(event events.TunnelEntityStubAvailableEvent)) {
	t.onEntityStubAvailableListeners = append(t.onEntityStubAvailableListeners, eventListener)
}

func (t *Tunnel) OnEntityStubAvailableListeners() []func(event events.TunnelEntityStubAvailableEvent) {
	return slices.Clone(t.onEntityStubAvailableListeners)
}

func (t *Tunnel) onEntityStubAvailable(event events.TunnelEntityStubAvailableEvent) {
	t.entityStub = event.EntityStub
}
