package common

const (
	BrokerRequestTypeGetTunnelInfo = iota + 1
)

type BrokerRequest struct {
	RequestType          int
	ResultCh             chan BrokerResult
	GetTunnelInfoRequest GetTunnelInfoRequest
}

type GetTunnelInfoRequest struct {
	TunnelId string
}
