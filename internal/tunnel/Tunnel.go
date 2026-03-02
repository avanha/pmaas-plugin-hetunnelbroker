package tunnel

type Tunnel struct {
	pmaasId  string
	tunnelId string
	name     string
}

func NewTunnel(pmaasId string, tunnelId string, name string) *Tunnel {
	return &Tunnel{
		pmaasId:  pmaasId,
		tunnelId: tunnelId,
		name:     name,
	}
}
