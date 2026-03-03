package hetunnelbroker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/avanha/pmaas-common/queue"
	"github.com/avanha/pmaas-plugin-hetunnelbroker/config"
	"github.com/avanha/pmaas-plugin-hetunnelbroker/internal/common"
	"github.com/avanha/pmaas-plugin-hetunnelbroker/internal/tunnel"
	spi "github.com/avanha/pmaas-spi"
)

type plugin struct {
	pluginConfig         config.PluginConfig
	container            spi.IPMAASContainer
	entityCounter        int
	running              bool
	requestCh            chan common.BrokerRequest
	requestQueue         *queue.RequestQueue[common.BrokerRequest]
	requestRetryingQueue *queue.RetryingRequestQueue[common.BrokerRequest, common.BrokerResult]
	workersWg            sync.WaitGroup
	cancelFn             context.CancelFunc
	tunnels              map[string]*tunnel.Tunnel
}

func NewPluginConfig() config.PluginConfig {
	return config.NewPluginConfig()
}

type Plugin interface {
	spi.IPMAASPlugin
}

func NewPlugin(pluginConfig config.PluginConfig) Plugin {
	instance := &plugin{
		pluginConfig: pluginConfig,
		tunnels:      make(map[string]*tunnel.Tunnel),
		requestCh:    make(chan common.BrokerRequest),
	}

	return instance
}

func (p *plugin) Init(container spi.IPMAASContainer) {
	p.container = container
	p.processConfig()
	p.requestQueue = queue.NewRequestQueue(p.requestCh)
	p.requestRetryingQueue = queue.NewRetryingRequestQueue(
		getResultChannel,
		exchangeResponseChannel,
		createErrorResponse,
		isFailedResult,
		canRetryRequest,
		p.requestQueue)
}

func getResultChannel(request *common.BrokerRequest) chan common.BrokerResult {
	return request.ResultCh
}

func exchangeResponseChannel(
	request *common.BrokerRequest, newChannel chan common.BrokerResult) chan common.BrokerResult {
	current := request.ResultCh
	request.ResultCh = newChannel

	return current
}

func createErrorResponse(err error) common.BrokerResult {
	return common.BrokerResult{
		Error: err,
	}
}

func isFailedResult(response *common.BrokerResult) bool {
	return response.Error != nil
}

func canRetryRequest(_ *common.BrokerRequest, _ *common.BrokerResult,
	attempts int, _ time.Time) bool {
	return attempts < 11
}

func (p *plugin) Start() {
	//p.registerEntities()
	_, cancel := context.WithCancel(context.Background())
	p.cancelFn = cancel
	p.workersWg.Go(p.requestQueue.Run)
	p.workersWg.Go(p.requestRetryingQueue.Run)
	//p.workersWg.Go(func() { p.worker.Run(ctx) })
	//go func() { p.poll(ctx) }()
	p.running = true
}

func (p *plugin) Stop() chan func() {
	fmt.Printf("%T Stopping...\n", p)
	p.running = false
	p.requestRetryingQueue.Stop()
	p.requestQueue.Stop()
	p.cancelFn()
	callbackCh := make(chan func())
	go func() {
		fmt.Printf("%T Waiting for workers to finish...\n", p)
		p.workersWg.Wait()
		callbackCh <- func() { p.onWorkersStopped(callbackCh) }
	}()

	return callbackCh
}

func (p *plugin) onWorkersStopped(callbackCh chan func()) {
	fmt.Printf("%T Workers stopped, deregistering entities...\n", p)
	//p.deregisterEntities()
	close(callbackCh)
}

func (p *plugin) nextEntityId() int {
	result := p.entityCounter
	p.entityCounter = p.entityCounter + 1

	return result
}

func (p *plugin) processConfig() {
	for _, configuredTunnel := range p.pluginConfig.Tunnels {
		instance := tunnel.NewTunnel(
			fmt.Sprintf("HE_Tunnelbroker_Tunnel_%d", p.nextEntityId()),
			configuredTunnel.Id,
			configuredTunnel.Name)
		p.tunnels[configuredTunnel.Name] = instance
	}
}
