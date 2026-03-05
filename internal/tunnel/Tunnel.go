package tunnel

import (
	"fmt"
	"time"

	"github.com/avanha/pmaas-plugin-hetunnelbroker/data"
	"github.com/avanha/pmaas-plugin-hetunnelbroker/internal/common"
	spi "github.com/avanha/pmaas-spi"
)

type Tunnel struct {
	container        spi.IPMAASContainer
	requestHandlerFn func(request common.BrokerRequest) error
	pmaasId          string
	tunnelId         string
	name             string
	currentData      data.TunnelData
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

func NewTunnel(
	container spi.IPMAASContainer,
	requestHandlerFn func(request common.BrokerRequest) error,
	pmaasId string,
	tunnelId string,
	name string) *Tunnel {
	return &Tunnel{
		container:        container,
		requestHandlerFn: requestHandlerFn,
		pmaasId:          pmaasId,
		tunnelId:         tunnelId,
		name:             name,
	}
}

func readAndProcessResult[T any](t *Tunnel, resultCh <-chan T, processFn func(T), resultDescription string) {
	result := <-resultCh
	err := t.container.EnqueueOnPluginGoRoutine(func() { processFn(result) })

	if err != nil {
		fmt.Printf("%T Error processing %s result: %v\n", t, resultDescription, err)
	}
}
