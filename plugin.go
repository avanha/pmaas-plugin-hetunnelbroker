package hetunnelbroker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/avanha/pmaas-common/queue"
	"github.com/avanha/pmaas-plugin-hetunnelbroker/config"
	"github.com/avanha/pmaas-plugin-hetunnelbroker/entities"
	"github.com/avanha/pmaas-plugin-hetunnelbroker/internal/common"
	"github.com/avanha/pmaas-plugin-hetunnelbroker/internal/tunnel"
	"github.com/avanha/pmaas-plugin-hetunnelbroker/internal/worker"
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
	worker               *worker.Worker
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
	p.worker = worker.NewWorker(p.requestCh, p.pluginConfig.Username, p.pluginConfig.UpdateKey)
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
	p.registerEntities()
	ctx, cancel := context.WithCancel(context.Background())
	p.cancelFn = cancel
	p.workersWg.Go(p.requestQueue.Run)
	p.workersWg.Go(p.requestRetryingQueue.Run)
	p.workersWg.Go(func() { p.worker.Run(ctx) })
	p.running = true
	go func() { p.poll(ctx) }()
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
	p.deregisterEntities()
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
			p.container,
			p.enqueueRequest,
			fmt.Sprintf("HE_Tunnelbroker_Tunnel_%d", p.nextEntityId()),
			configuredTunnel.Id,
			configuredTunnel.Name)
		p.tunnels[configuredTunnel.Name] = instance
	}
}

func (p *plugin) registerEntities() {
	for _, instance := range p.tunnels {
		// This lambda captures the plugin instance and the hostInstance
		// and passes it to the entity manager.  However, entities are deregistered on plugin
		// stop, so this is OK.
		var tunnelStubFactoryFn spi.EntityStubFactoryFunc = func() (any, error) {
			return instance.GetStub(), nil
		}
		pmaasId, err := p.container.RegisterEntity(
			instance.LocalId(),
			entities.TunnelType,
			instance.LocalId(),
			tunnelStubFactoryFn)

		if err != nil {
			fmt.Printf("%T Error registering %s: %s\n", p, instance.LocalId(), err)
			continue
		}

		instance.SetPmaasEntityId(pmaasId)
	}
}

func (p *plugin) deregisterEntities() {
	for _, instance := range p.tunnels {
		err := p.container.DeregisterEntity(instance.PmaasEntityId())

		if err == nil {
			instance.ClearPmaasEntityId()
		} else {
			fmt.Printf("%TError deregistering %s: %s\n", p, instance.LocalId(), err)
		}

		instance.CloseStubIfPresent()
	}
}

func (p *plugin) poll(ctx context.Context) {
	// Initial pause
	timer := time.NewTimer(5 * time.Second)

	select {
	case <-ctx.Done():
		timer.Stop()
		return
	case <-timer.C:
	}

	p.enqueueRefresh()

	// Refresh every 4 hours
	ticker := time.NewTicker(4 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.enqueueRefresh()
		}
	}
}

func (p *plugin) enqueueRefresh() {
	err := p.container.EnqueueOnPluginGoRoutine(func() {
		refreshError := p.refresh()

		if refreshError != nil {
			fmt.Printf("%T: Error enqueueing refresh: %v\n", p, refreshError)
		}
	})

	if err != nil {
		fmt.Printf("%T: Unable to enque refresh: %v\n", p, err)
	}
}

func (p *plugin) refresh() error {
	if !p.running {
		return fmt.Errorf("plugin is not running")
	}

	errors := make([]error, 0)

	for _, tun := range p.tunnels {
		err := tun.Refresh()

		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors encountered during refresh: %v", errors)
	}

	return nil
}

// enqueueRequest sends a request to the plugin's worker(s) if the plugin is running.
// Returns an error if unable to add to the queue.  Must be called from the main plugin goroutine since it
// reads the running state of the plugin.
func (p *plugin) enqueueRequest(request common.BrokerRequest) error {
	if !p.running {
		return fmt.Errorf("unable to enqueue request, plugin is not running")
	}

	err := p.requestRetryingQueue.Enqueue(&request)

	if err != nil {
		return fmt.Errorf("unable to enqueue request: %w", err)
	}

	return err
}
