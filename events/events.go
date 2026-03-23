package events

import (
	"github.com/avanha/pmaas-plugin-hetunnelbroker/entities"
	"github.com/avanha/pmaas-spi/events"
)

type TunnelEntityStubAvailableEvent struct {
	events.EntityEvent
	EntityStub entities.Tunnel
}
