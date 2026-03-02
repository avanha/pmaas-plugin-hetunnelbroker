package hetunnelbroker

import (
	"github.com/avanha/pmaas-plugin-hetunnelbroker/config"
	spi "github.com/avanha/pmaas-spi"
)

type plugin struct {
	pluginConfig config.PluginConfig
	container    spi.IPMAASContainer
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
	}

	return instance
}

func (p *plugin) Init(container spi.IPMAASContainer) {
	p.container = container
}

func (p *plugin) Start() {

}

func (p *plugin) Stop() chan func() {
	return p.container.ClosedCallbackChannel()
}
