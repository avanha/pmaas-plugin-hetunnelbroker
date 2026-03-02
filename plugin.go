package hetunnelbroker

import (
	"fmt"

	"github.com/avanha/pmaas-plugin-hetunnelbroker/config"
	"github.com/avanha/pmaas-plugin-hetunnelbroker/internal/tunnel"
	spi "github.com/avanha/pmaas-spi"
)

type plugin struct {
	pluginConfig  config.PluginConfig
	container     spi.IPMAASContainer
	entityCounter int
	tunnels       map[string]*tunnel.Tunnel
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
	}

	return instance
}

func (p *plugin) Init(container spi.IPMAASContainer) {
	p.container = container
	p.processConfig()
}

func (p *plugin) Start() {

}

func (p *plugin) Stop() chan func() {
	return p.container.ClosedCallbackChannel()
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
