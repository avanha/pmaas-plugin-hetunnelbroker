package tunnel

import (
	"fmt"
	"time"

	"github.com/avanha/pmaas-plugin-hetunnelbroker/data"
	"github.com/avanha/pmaas-plugin-hetunnelbroker/entities"
	"github.com/avanha/pmaas-plugin-hetunnelbroker/internal/common"
	spi "github.com/avanha/pmaas-spi"
	spicommon "github.com/avanha/pmaas-spi/common"
)

type Tunnel struct {
	container        spi.IPMAASContainer
	requestHandlerFn func(request common.BrokerRequest) error
	localId          string
	pmaasEntityId    string
	tunnelId         string
	name             string
	currentData      data.TunnelData
	stub             *Stub
}

func NewTunnel(
	container spi.IPMAASContainer,
	requestHandlerFn func(request common.BrokerRequest) error,
	localId string,
	tunnelId string,
	name string) *Tunnel {
	return &Tunnel{
		container:        container,
		requestHandlerFn: requestHandlerFn,
		localId:          localId,
		tunnelId:         tunnelId,
		name:             name,
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

	go readAndProcessResult(t, resultCh, t.processGetTunnelInfoResult, "Tunnel info retrieval")

	return nil
}

func (t *Tunnel) processGetTunnelInfoResult(result common.BrokerResult) {
	if result.Error == nil {
		fmt.Printf("%T tunnel %s: %s\n", t, t.name, result.Message)
		//t.updateData(&result.CurrentData)
		t.currentData.GetSuccessCount++
	} else {
		fmt.Printf("%T Error retrieving tunnel %s info: %v\n", t, t.name, result.Error)
		t.currentData.LastError = result.Error
		t.currentData.LastErrorTime = time.Now()
		t.currentData.GetErrorCount++
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
