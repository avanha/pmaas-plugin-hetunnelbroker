package config

type PluginConfig struct {
	Username  string
	UpdateKey string
	Tunnels   map[string]*Tunnel
}

func NewPluginConfig() PluginConfig {
	return PluginConfig{
		Tunnels: make(map[string]*Tunnel),
	}
}

func (c *PluginConfig) AddTunnel(name string, id string) *Tunnel {
	tunnel := NewTunnel(name, id)
	c.Tunnels[name] = tunnel

	return tunnel
}
