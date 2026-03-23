package tunnel

import (
	"fmt"
	"net"
	"time"

	"github.com/avanha/pmaas-plugin-hetunnelbroker/data"
	"github.com/avanha/pmaas-plugin-hetunnelbroker/entities"
	"github.com/avanha/pmaas-plugin-hetunnelbroker/events"
	"github.com/avanha/pmaas-plugin-hetunnelbroker/internal/common"
	spi "github.com/avanha/pmaas-spi"
	spicommon "github.com/avanha/pmaas-spi/common"
)

type Tunnel struct {
	container                      spi.IPMAASContainer
	requestHandlerFn               func(request common.BrokerRequest) error
	localId                        string
	pmaasEntityId                  string
	tunnelId                       string
	name                           string
	currentData                    data.TunnelData
	onEntityStubAvailableListeners []func(event events.TunnelEntityStubAvailableEvent)
	stub                           *Stub
}

func NewTunnel(
	container spi.IPMAASContainer,
	requestHandlerFn func(request common.BrokerRequest) error,
	localId string,
	tunnelId string,
	name string,
	onEntityStubAvailableListeners []func(event events.TunnelEntityStubAvailableEvent)) *Tunnel {
	tunnel := &Tunnel{
		container:                      container,
		requestHandlerFn:               requestHandlerFn,
		localId:                        localId,
		tunnelId:                       tunnelId,
		name:                           name,
		onEntityStubAvailableListeners: onEntityStubAvailableListeners,
	}
	tunnel.currentData.Name = name

	return tunnel
}

func (t *Tunnel) Data() data.TunnelData {
	return t.currentData
}

func (t *Tunnel) ProcessConfiguredListeners(container spi.IPMAASContainer) {
	t.processOnEntityStubAvailableListeners(container)
}

func (t *Tunnel) processOnEntityStubAvailableListeners(container spi.IPMAASContainer) {
	numListeners := len(t.onEntityStubAvailableListeners)

	if numListeners == 0 {
		return
	}

	event := events.TunnelEntityStubAvailableEvent{EntityStub: t.GetStub()}
	invocations := make([]func(), numListeners)

	for i, listener := range t.onEntityStubAvailableListeners {
		invocations[i] = func() { listener(event) }
	}

	err := container.EnqueueOnServerGoRoutine(invocations)

	if err != nil {
		fmt.Printf("error enqueuing TunnelEntityStubAvailableEvent listener invocations: %v\n", err)
	}
}

func (t *Tunnel) Refresh() error {
	resultCh := make(chan common.BrokerResult)
	request := common.BrokerRequest{
		RequestType: common.BrokerRequestTypeGetTunnelInfo,
		ResultCh:    resultCh,
		GetTunnelInfoRequest: common.GetTunnelInfoRequest{
			TunnelId: t.tunnelId,
		},
	}

	err := t.requestHandlerFn(request)

	if err != nil {
		return fmt.Errorf("failed to enqueue tunnel info %s retrieval: %v", t.name, err)
	}

	// Waits for the result channel to return a value and process it on the main plugin goroutine.
	go readAndProcessResult(t, resultCh, t.processGetTunnelInfoResult, "Tunnel info retrieval")

	return nil
}

func (t *Tunnel) UpdateClientIpV4Address(value net.IP) error {
	fmt.Printf("Received request to update tunnel %s client IPv4 address to %s\n", t.Name, value)
	resultCh := make(chan common.BrokerResult)
	request := common.BrokerRequest{
		RequestType: common.BrokerRequestTypeUpdateTunnelClientIpV4Address,
		ResultCh:    resultCh,
		UpdateTunnelClientIpV4AddressRequest: common.UpdateTunnelClientIpV4AddressRequest{
			TunnelId:    t.tunnelId,
			CurrentData: t.currentData,
			NewAddress:  value,
		},
	}

	err := t.requestHandlerFn(request)

	if err != nil {
		return fmt.Errorf("failed to enqueue tunnel %s update: %v", t.Name, err)
	}

	go readAndProcessResult(t, resultCh, t.processUpdateTunnelClientIpV4AddressResult, "update tunnel client address")

	return nil
}

func (t *Tunnel) processGetTunnelInfoResult(result common.BrokerResult) {
	if result.Error == nil {
		fmt.Printf("%T tunnel %s: %s\n", t, t.name, result.Message)
		t.updateData(&result.TunnelData)
		t.currentData.GetSuccessCount++
	} else {
		fmt.Printf("%T Error retrieving tunnel %s info: %v\n", t, t.name, result.Error)
		t.currentData.LastError = result.Error
		t.currentData.LastErrorTime = time.Now()
		t.currentData.GetErrorCount++
	}
}

func (t *Tunnel) updateData(newData *data.TunnelData) {
	t.currentData.Description = newData.Description
	t.currentData.LastUpdateTime = newData.LastUpdateTime
	t.currentData.ClientIpV4Address = newData.ClientIpV4Address
	t.currentData.ClientIpV6Address = newData.ClientIpV6Address
	t.currentData.ServerIpV4Address = newData.ServerIpV4Address
	t.currentData.ServerIpV6Address = newData.ServerIpV6Address
	t.currentData.Routed64Net = newData.Routed64Net
	t.currentData.Routed48Net = newData.Routed48Net
}

func (t *Tunnel) processUpdateTunnelClientIpV4AddressResult(result common.BrokerResult) {
	if result.Error == nil {
		fmt.Printf("Updated tunnel %s successfully: %s\n", t.name, result.Message)
		t.updateData(&result.TunnelData)
		t.currentData.LastModifiedTime = result.TunnelData.LastModifiedTime
		t.currentData.UpdateSuccessCount++
	} else {
		fmt.Printf("Error updating tunnel %s: %v\n", t.name, result.Error)
		t.currentData.LastError = result.Error
		t.currentData.LastErrorTime = time.Now()
		t.currentData.UpdateErrorCount++
	}
}

// GetStub returns a proxy struct that implements the entities.Tunnel interface.  The function ensures
// that only one stub is created for the entity.  This function is not thread-safe, but that is because
// it's only called from the plugin goroutine, whether direcftly or via the PMAAS server.
func (t *Tunnel) GetStub() entities.Tunnel {
	if t.stub == nil {
		t.stub = NewStub(
			t.pmaasEntityId,
			&spicommon.ThreadSafeEntityWrapper[entities.Tunnel]{
				Container: t.container,
				Entity:    t,
			})
	}

	return t.stub
}

func (t *Tunnel) LocalId() string {
	return t.localId
}

func (t *Tunnel) PmaasEntityId() string {
	return t.pmaasEntityId
}

func (t *Tunnel) SetPmaasEntityId(id string) {
	if t.pmaasEntityId != "" {
		panic(fmt.Errorf("Tunnel %s already has pmass entity id %s", t.localId, t.pmaasEntityId))
	}

	t.pmaasEntityId = id
}

func (t *Tunnel) ClearPmaasEntityId() {
	t.pmaasEntityId = ""
}

func (t *Tunnel) CloseStubIfPresent() {
	if t.stub != nil {
		t.stub.Close()
		t.stub = nil
	}
}

func (t *Tunnel) Name() string {
	return t.name
}

func readAndProcessResult[T any](t *Tunnel, resultCh <-chan T, processFn func(T), resultDescription string) {
	result := <-resultCh
	err := t.container.EnqueueOnPluginGoRoutine(func() { processFn(result) })

	if err != nil {
		fmt.Printf("%T Error processing %s result: %v\n", t, resultDescription, err)
	}
}
