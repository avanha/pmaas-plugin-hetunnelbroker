package config

type Tunnel struct {
	Name string
	Id   string
}

func NewTunnel(name string, id string) *Tunnel {
	return &Tunnel{
		Name: name,
		Id:   id,
	}
}
